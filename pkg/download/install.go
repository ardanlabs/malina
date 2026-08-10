package download

import (
	"archive/zip"
	"bytes"
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
	"sort"
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
)

// DefaultSDVersion is the leejet/stable-diffusion.cpp release tag malina's
// FFI struct mirrors (e.g. sd_ctx_params_t's 280-byte layout) are tested
// against. `malina install` uses this when no -v flag is supplied so first
// installs and CI runs don't depend on the GitHub releases API. Bumping
// this value is a deliberate, reviewable change that should be paired with
// re-running the FFI sizeof tests in pkg/sd.
const DefaultSDVersion = "master-813-bfbef5b"

// SDRepo is the upstream GitHub repo we fetch prebuilt libraries from.
const SDRepo = "leejet/stable-diffusion.cpp"

var (
	// RetryCount is how many times we retry the GitHub releases API.
	RetryCount = 3
	// RetryDelay is the delay between releases-API retries.
	RetryDelay = 3 * time.Second
)

// SDLatestVersion queries the GitHub releases API for the most recent
// upstream stable-diffusion.cpp release tag. leejet currently tags every
// CI build (e.g. "master-813-bfbef5b"), so "latest" usually means
// last-merged-to-master, not a semver release.
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

func getLatestSDVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", SDRepo)
	body, err := httpGetJSON(url)
	if err != nil {
		return "", err
	}
	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.TagName == "" {
		return "", errors.New("releases API returned empty tag_name")
	}
	return result.TagName, nil
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
//	version:      a leejet release tag (e.g. "master-813-bfbef5b")
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

	urls, err := resolveAssetURLs(ctx, arch, osVal, prcssr, version)
	if err != nil {
		return err
	}
	for _, url := range urls {
		if err := downloadAndExtract(ctx, url, dest, osVal, progress); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================

type releaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// resolveAssetURLs queries the GitHub releases API for the requested tag
// and selects the assets matching the platform.
//
// leejet asset names contain the commit SHA and the build VM's OS minor
// version (e.g. ubuntu 24.04, macOS 15.7.7), so we cannot compose the URL
// from the version tag alone — we have to discover it.
func resolveAssetURLs(_ context.Context, arch Arch, osVal OS, prcssr Processor, version string) ([]string, error) {
	if osVal.Equal(Linux) && prcssr.Equal(CUDA) {
		return nil, fmt.Errorf("%w: leejet/stable-diffusion.cpp publishes no linux/cuda artifact; use -p vulkan or -p rocm, or build stable-diffusion.cpp yourself", ErrUnsupportedPlatform)
	}

	pattern, err := assetPattern(arch, osVal, prcssr)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", SDRepo, version)
	body, err := httpGetJSON(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch release %s: %w", version, err)
	}

	var rel struct {
		Assets []releaseAsset `json:"assets"`
	}
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("parse release %s: %w", version, err)
	}

	return selectAssetURLs(rel.Assets, pattern, osVal, prcssr, version)
}

func selectAssetURLs(assets []releaseAsset, pattern *regexp.Regexp, osVal OS, prcssr Processor, version string) ([]string, error) {
	// Pick the asset whose name matches the per-platform regex. If multiple
	// match (e.g. two ROCm variants), prefer the lexicographically-greatest
	// — for leejet's ROCm-7.13.0 vs ROCm-7.2.1 layout that gets us the
	// newer build.
	var matches []string
	for _, a := range assets {
		if pattern.MatchString(a.Name) {
			matches = append(matches, a.DownloadURL)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("%w: release %s has no asset matching %q",
			ErrFileNotFound, version, pattern)
	}
	sort.Strings(matches)
	urls := []string{matches[len(matches)-1]}

	// Current Windows CUDA releases package the CUDA runtime and cuBLAS DLLs
	// separately from stable-diffusion.dll. Older releases were self-contained,
	// so include the companion archive when the same release provides it.
	if osVal.Equal(Windows) && prcssr.Equal(CUDA) {
		for _, a := range assets {
			if a.Name == "cudart-sd-bin-win-cu12-x64.zip" {
				urls = append(urls, a.DownloadURL)
				break
			}
		}
	}

	return urls, nil
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
func downloadAndExtract(ctx context.Context, url, dest string, osVal OS, progress getter.ProgressTracker) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
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
			return fmt.Errorf("%w: %s", ErrFileNotFound, url)
		}
		return err
	}
	defer os.Remove(downloadFile)

	return extractSharedLibs(downloadFile, dest, osVal)
}

// extractSharedLibs walks the release zip and writes every shared library
// (.so / .dylib / .dll) and SONAME symlink flat into dest. Inner directory
// structure is discarded.
func extractSharedLibs(zipPath, dest string, osVal OS) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer zr.Close()

	suffixes := libSuffixes(osVal)
	any := false
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
				return err
			}
			any = true
			continue
		}

		if err := writeZipRegular(f, target); err != nil {
			return err
		}
		any = true
	}

	if !any {
		return fmt.Errorf("%s contained no shared library files for %s", zipPath, osVal)
	}
	return nil
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

func httpGetJSON(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
