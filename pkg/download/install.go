package download

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	getter "github.com/hashicorp/go-getter"
)

var (
	ErrUnknownArch         = errors.New("unknown architecture")
	ErrUnknownOS           = errors.New("unknown OS")
	ErrUnknownProcessor    = errors.New("unknown processor")
	ErrInvalidVersion      = errors.New("invalid version")
	ErrFileNotFound        = errors.New("could not download file: the requested stable-diffusion.cpp release does not include an asset for this platform")
	ErrUnsupportedPlatform = errors.New("no prebuilt stable-diffusion.cpp asset for this platform")
	ErrNoAssetDigest       = errors.New("release asset has no SHA-256 digest")
	ErrDigestMismatch      = errors.New("SHA-256 digest does not match")
)

// DefaultSDVersion is the authenticated leejet/stable-diffusion.cpp release
// malina's FFI struct mirrors (e.g. sd_ctx_params_t's 288-byte layout) are
// tested against. The suffix pins the exact bytes of library_manifest.json.
// Bumping this value is a deliberate, reviewable change that should be paired
// with regenerating that manifest and re-running the FFI sizeof tests in
// pkg/sd.
const DefaultSDVersion = "master-849-d04e895@sha256:b9c5d34ad3de676968b375716197718bce89b53999ea82ce778ab0c691ab00a9"

// SDRepo is the upstream GitHub repo we fetch prebuilt libraries from.
const SDRepo = "leejet/stable-diffusion.cpp"

var (
	// RetryCount is how many times we retry the GitHub releases API.
	RetryCount = 3
	// RetryDelay is the delay between releases-API retries.
	RetryDelay = 3 * time.Second
)

// SDLatestVersion queries the GitHub releases API for the greatest upstream
// stable-diffusion.cpp build tag. GitHub's latest-release marker is not used
// because concurrent CI builds can finish and publish out of order.
func SDLatestVersion() (string, error) {
	var (
		version string
		err     error
	)
	for range RetryCount {
		version, err = getLatestSDVersion()
		if err == nil {
			return version, nil
		}
		time.Sleep(RetryDelay)
	}
	return "", fmt.Errorf("unable to fetch latest version: %w", err)
}

type sdRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func getLatestSDVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100", SDRepo)
	body, err := httpGetJSON(context.Background(), url)
	if err != nil {
		return "", err
	}
	var releases []sdRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", err
	}
	return latestSDVersion(releases)
}

func latestSDVersion(releases []sdRelease) (string, error) {
	latestVersion := ""
	latestBuild := -1
	tagPattern := regexp.MustCompile(`^master-([0-9]+)-[0-9a-f]+$`)
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		matches := tagPattern.FindStringSubmatch(release.TagName)
		if matches == nil {
			continue
		}

		build, err := strconv.Atoi(matches[1])
		if err != nil {
			return "", fmt.Errorf("parse stable-diffusion.cpp build number %q: %w", matches[1], err)
		}
		if build > latestBuild {
			latestVersion = release.TagName
			latestBuild = build
		}
	}
	if latestVersion == "" {
		return "", errors.New("releases API returned no stable-diffusion.cpp build tags")
	}

	return latestVersion, nil
}

// AlreadyInstalled reports whether a stable-diffusion shared library is
// present at libPath. The check matches any file starting with
// libstable-diffusion since the upstream zip layout (and library extension)
// varies per OS.
func AlreadyInstalled(libPath string) bool {
	entries, err := os.ReadDir(libPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasPrefix(name, "libstable-diffusion") || name == "stable-diffusion.dll" {
			return true
		}
	}
	return false
}

var execCommand = exec.Command

// HasCUDA reports whether nvidia-smi is on PATH and parses the CUDA version
// from its output.
func HasCUDA() (bool, string) {
	if runtime.GOOS == "darwin" {
		return false, ""
	}
	cmd := execCommand("nvidia-smi")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return false, ""
	}
	re := regexp.MustCompile(`CUDA Version:\s*([0-9.]+)`)
	matches := re.FindStringSubmatch(out.String())
	if len(matches) >= 2 {
		return true, matches[1]
	}
	return true, ""
}

// VersionIsValid does a cheap shape check on a leejet release tag.
// Accepted forms: "master-N-SHA" (current CI), "vN.M.P" (legacy semver),
// or any non-empty string that contains "-".
func VersionIsValid(version string) error {
	if version == "" {
		return ErrInvalidVersion
	}
	if !strings.Contains(version, "-") && !strings.HasPrefix(version, "v") {
		return ErrInvalidVersion
	}
	return nil
}

// LibraryName returns the filename of the primary stable-diffusion shared
// library for the given OS, as it ships inside the upstream release zip.
func LibraryName(operatingSystem string) string {
	osVal, err := ParseOS(operatingSystem)
	if err != nil {
		return "unknown"
	}
	switch osVal {
	case Linux:
		return "libstable-diffusion.so"
	case Windows:
		return "stable-diffusion.dll"
	case Darwin:
		return "libstable-diffusion.dylib"
	default:
		return "unknown"
	}
}

// =============================================================================

// Get downloads and installs the stable-diffusion.cpp precompiled
// libraries for the requested platform.
//
//	architecture: "amd64" or "arm64"
//	osName:       "linux", "darwin", or "windows"
//	processor:    "cpu", "cuda", "metal", "vulkan", or "rocm"
//	version:      a leejet release tag (e.g. "master-849-d04e895")
//	dest:         destination directory for the extracted libraries
func Get(architecture, osName, processor, version, dest string) error {
	return GetWithProgress(architecture, osName, processor, version, dest, ProgressTracker)
}

// GetWithProgress is Get with a caller-supplied progress tracker.
func GetWithProgress(architecture, osName, processor, version, dest string, progress getter.ProgressTracker) error {
	return GetWithContext(context.Background(), architecture, osName, processor, version, dest, progress)
}

// GetWithContext is GetWithProgress with a caller-supplied context.
func GetWithContext(ctx context.Context, architecture, osName, processor, version, dest string, progress getter.ProgressTracker) error {
	tag, manifestSHA, err := ParsePinnedVersion(version)
	if err != nil {
		return err
	}
	version = tag

	arch, err := ParseArch(architecture)
	if err != nil {
		return ErrUnknownArch
	}
	osVal, err := ParseOS(osName)
	if err != nil {
		return ErrUnknownOS
	}
	prcssr, err := ParseProcessor(processor)
	if err != nil {
		return ErrUnknownProcessor
	}
	if err := VersionIsValid(version); err != nil {
		return ErrInvalidVersion
	}
	if err := authenticateManifest(version, manifestSHA); err != nil {
		return err
	}

	assets, releaseMetadata, err := resolveAssets(ctx, arch, osVal, prcssr, version)
	if err != nil {
		return err
	}

	installed := make([]InstallAsset, 0, len(assets))
	for _, asset := range assets {
		digest, err := expectedAssetDigest(version, manifestSHA, asset)
		if err != nil {
			return err
		}
		files, links, err := downloadAndExtract(ctx, asset.DownloadURL, dest, osVal, digest, progress)
		if err != nil {
			return err
		}
		installed = append(installed, InstallAsset{
			ID:     asset.ID,
			Name:   asset.Name,
			Size:   asset.Size,
			SHA256: digest,
			Files:  files,
			Links:  links,
		})
	}

	record := InstallRecord{
		Version:      installRecordVersion,
		Tag:          version,
		Arch:         architecture,
		OS:           osName,
		Processor:    processor,
		Installed:    time.Now().UTC(),
		ManifestHash: trustedManifestHash(version),
		Assets:       installed,
	}
	if len(releaseMetadata) > 0 {
		record.ReleaseHash = hashBytes(releaseMetadata)
		if err := writeReleaseMetadata(dest, releaseMetadata); err != nil {
			return err
		}
	}
	if err := verifyExpectedFiles(ctx, dest, record); err != nil {
		return err
	}
	if err := writeInstallRecord(dest, record); err != nil {
		return err
	}

	return nil
}

// =============================================================================

type releaseAsset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Digest      string `json:"digest"`
	State       string `json:"state"`
	DownloadURL string `json:"browser_download_url"`
}

// resolveAssets selects assets from the embedded manifest when it covers the
// requested version. Other versions are resolved through the GitHub releases
// API, whose response is returned for caching beside the installed libraries.
//
// leejet asset names contain the commit SHA and the build VM's OS minor
// version (e.g. ubuntu 24.04, macOS 15.7.7), so custom versions must be
// discovered through release metadata.
func resolveAssets(ctx context.Context, arch Arch, osVal OS, prcssr Processor, version string) ([]releaseAsset, []byte, error) {
	if osVal.Equal(Linux) && prcssr.Equal(CUDA) {
		return nil, nil, fmt.Errorf("%w: leejet/stable-diffusion.cpp publishes no linux/cuda artifact; use -p vulkan or -p rocm, or build stable-diffusion.cpp yourself", ErrUnsupportedPlatform)
	}

	pattern, err := assetPattern(arch, osVal, prcssr)
	if err != nil {
		return nil, nil, err
	}

	if manifest, ok := trustedManifest(version); ok {
		assets := make([]releaseAsset, 0, len(manifest.Assets))
		for name, asset := range manifest.Assets {
			assets = append(assets, releaseAsset{
				ID:          asset.ID,
				Name:        name,
				Size:        asset.Size,
				Digest:      "sha256:" + asset.SHA256,
				State:       "uploaded",
				DownloadURL: fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", SDRepo, version, name),
			})
		}
		selected, err := selectAssets(assets, pattern, osVal, prcssr, version)
		return selected, nil, err
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", SDRepo, version)
	body, err := httpGetJSON(ctx, apiURL)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch release %s: %w", version, err)
	}

	var rel struct {
		TagName string         `json:"tag_name"`
		Assets  []releaseAsset `json:"assets"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, nil, fmt.Errorf("parse release %s: %w", version, err)
	}
	if rel.TagName != version {
		return nil, nil, fmt.Errorf("release tag mismatch: got %q, want %q", rel.TagName, version)
	}

	assets, err := selectAssets(rel.Assets, pattern, osVal, prcssr, version)
	if err != nil {
		return nil, nil, err
	}
	return assets, body, nil
}

func selectAssets(assets []releaseAsset, pattern *regexp.Regexp, osVal OS, prcssr Processor, version string) ([]releaseAsset, error) {
	// Pick the asset whose name matches the per-platform regex. If multiple
	// match (e.g. two ROCm variants), prefer the lexicographically-greatest
	// — for leejet's ROCm-7.13.0 vs ROCm-7.2.1 layout that gets us the
	// newer build.
	var matches []releaseAsset
	for _, a := range assets {
		if pattern.MatchString(a.Name) {
			matches = append(matches, a)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: release %s has no asset matching %q",
			ErrFileNotFound, version, pattern)
	}
	slices.SortFunc(matches, func(a releaseAsset, b releaseAsset) int {
		return cmp.Compare(a.Name, b.Name)
	})
	selected := []releaseAsset{matches[len(matches)-1]}

	// Current Windows CUDA releases package the CUDA runtime and cuBLAS DLLs
	// separately from stable-diffusion.dll. Older releases were self-contained,
	// so include the companion archive when the same release provides it.
	if osVal.Equal(Windows) && prcssr.Equal(CUDA) {
		for _, a := range assets {
			if a.Name == "cudart-sd-bin-win-cu12-x64.zip" {
				selected = append(selected, a)
				break
			}
		}
	}

	return selected, nil
}

func assetPattern(arch Arch, osVal OS, prcssr Processor) (*regexp.Regexp, error) {
	switch osVal {
	case Darwin:
		if !arch.Equal(ARM64) {
			return nil, fmt.Errorf("%w: darwin only ships arm64 (Apple Silicon)", ErrUnsupportedPlatform)
		}
		switch prcssr {
		case CPU, Metal:
			return regexp.MustCompile(`^sd-.*-bin-Darwin-.*-arm64\.zip$`), nil
		default:
			return nil, fmt.Errorf("%w: darwin only supports cpu/metal", ErrUnknownProcessor)
		}

	case Windows:
		if !arch.Equal(AMD64) {
			return nil, fmt.Errorf("%w: windows only ships x64", ErrUnsupportedPlatform)
		}
		switch prcssr {
		case CPU:
			return regexp.MustCompile(`^sd-.*-bin-win-(avx2|cpu)-x64\.zip$`), nil
		case CUDA:
			return regexp.MustCompile(`^sd-.*-bin-win-cuda12-x64\.zip$`), nil
		case Vulkan:
			return regexp.MustCompile(`^sd-.*-bin-win-vulkan-x64\.zip$`), nil
		case ROCm:
			return regexp.MustCompile(`^sd-.*-bin-win-rocm-.*-x64\.zip$`), nil
		default:
			return nil, fmt.Errorf("%w: windows supports cpu/cuda/vulkan/rocm", ErrUnknownProcessor)
		}

	case Linux:
		if !arch.Equal(AMD64) {
			return nil, fmt.Errorf("%w: linux only ships x86_64", ErrUnsupportedPlatform)
		}
		switch prcssr {
		case CPU:
			return regexp.MustCompile(`^sd-.*-bin-Linux-Ubuntu-.*-x86_64\.zip$`), nil
		case Vulkan:
			return regexp.MustCompile(`^sd-.*-bin-Linux-Ubuntu-.*-x86_64-vulkan\.zip$`), nil
		case ROCm:
			return regexp.MustCompile(`^sd-.*-bin-Linux-Ubuntu-.*-x86_64-rocm-.*\.zip$`), nil
		default:
			return nil, fmt.Errorf("%w: leejet linux releases support cpu/vulkan/rocm (no cuda)", ErrUnknownProcessor)
		}
	}
	return nil, ErrUnknownOS
}

// =============================================================================

// downloadAndExtract fetches the asset zip with go-getter (resumes
// interrupted downloads via HTTP Range) and extracts every shared library
// flat into dest.
func downloadAndExtract(ctx context.Context, url, dest string, osVal OS, wantDigest string, progress getter.ProgressTracker) (map[string]string, map[string]string, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create destination dir: %w", err)
	}

	downloadFile := filepath.Join(dest, filepath.Base(url))
	src := url
	if !strings.Contains(src, "?") {
		src += "?archive=false"
	} else {
		src += "&archive=false"
	}

	client := &getter.Client{
		Ctx:  ctx,
		Src:  src,
		Dst:  downloadFile,
		Mode: getter.ClientModeFile,
	}
	if progress != nil {
		client.ProgressListener = progress
	}
	if err := client.Get(); err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, nil, fmt.Errorf("%w: %s", ErrFileNotFound, url)
		}
		return nil, nil, err
	}
	defer os.Remove(downloadFile)

	gotDigest, err := hashFile(downloadFile)
	if err != nil {
		return nil, nil, fmt.Errorf("hash release asset: %w", err)
	}
	if !strings.EqualFold(gotDigest, wantDigest) {
		return nil, nil, fmt.Errorf("%w for %s: got %s, want %s", ErrDigestMismatch, filepath.Base(url), gotDigest, wantDigest)
	}

	return extractSharedLibs(downloadFile, dest, osVal)
}

// extractSharedLibs walks the release zip and writes every shared library
// (.so / .dylib / .dll) and SONAME symlink flat into dest. Inner directory
// structure is discarded.
func extractSharedLibs(zipPath, dest string, osVal OS) (map[string]string, map[string]string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer zr.Close()

	suffixes := libSuffixes(osVal)
	files := make(map[string]string)
	links := make(map[string]string)
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if !matchesAny(strings.ToLower(base), suffixes) {
			continue
		}

		target := filepath.Join(dest, base)
		mode := f.Mode()

		// SONAME symlinks (e.g. libstable-diffusion.so -> libstable-diffusion.so.1)
		// must be preserved or dlopen will fail at runtime.
		if mode&os.ModeSymlink != 0 {
			if err := writeZipSymlink(f, target); err != nil {
				return nil, nil, err
			}
			link, err := os.Readlink(target)
			if err != nil {
				return nil, nil, fmt.Errorf("read installed symlink %s: %w", target, err)
			}
			links[base] = link
			continue
		}

		if err := writeZipRegular(f, target); err != nil {
			return nil, nil, err
		}
		digest, err := hashFile(target)
		if err != nil {
			return nil, nil, fmt.Errorf("hash installed library %s: %w", target, err)
		}
		files[base] = digest
	}

	if len(files) == 0 && len(links) == 0 {
		return nil, nil, fmt.Errorf("%s contained no shared library files for %s", zipPath, osVal)
	}
	return files, links, nil
}

func libSuffixes(osVal OS) []string {
	switch osVal {
	case Linux:
		return []string{".so", ".so."}
	case Darwin:
		return []string{".dylib"}
	case Windows:
		return []string{".dll"}
	}
	return nil
}

func matchesAny(name string, suffixes []string) bool {
	for _, s := range suffixes {
		if strings.HasSuffix(name, s) || strings.Contains(name, s) {
			return true
		}
	}
	return false
}

func writeZipRegular(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open %s in zip: %w", f.Name, err)
	}
	defer rc.Close()

	mode := f.Mode() & 0o777
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return fmt.Errorf("write %s: %w", target, err)
	}
	return out.Close()
}

func writeZipSymlink(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open symlink %s: %w", f.Name, err)
	}
	defer rc.Close()
	linkData, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read symlink target %s: %w", f.Name, err)
	}
	_ = os.Remove(target)
	if err := os.Symlink(strings.TrimSpace(string(linkData)), target); err != nil {
		return fmt.Errorf("symlink %s: %w", target, err)
	}
	return nil
}

// =============================================================================

func httpGetJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api %s: status %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}
