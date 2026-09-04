package download

import (
	"cmp"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var (
	// ErrNoInstallRecord means a library directory has no Malina install record.
	ErrNoInstallRecord = errors.New("no install record")

	// ErrNoFileDigests means Malina has no trusted per-file manifest for a release.
	ErrNoFileDigests = errors.New("the trusted manifest does not cover installed files")

	// ErrRecordMismatch means an install record does not match the trusted manifest.
	ErrRecordMismatch = errors.New("the install record does not agree with the trusted manifest")

	// ErrInvalidDigest means a version pin is not sha256 followed by 64 hexadecimal characters.
	ErrInvalidDigest = errors.New("invalid digest")
)

const (
	installRecordVersion = 1

	// InstallRecordName is the metadata file written beside installed libraries.
	InstallRecordName = "malina-install.json"
)

//go:embed library_manifest.json
var trustedManifestJSON []byte

type libraryManifest struct {
	Version    int                      `json:"version"`
	Repository string                   `json:"repository"`
	Tag        string                   `json:"tag"`
	Generated  time.Time                `json:"generated"`
	Assets     map[string]manifestAsset `json:"assets"`
}

type manifestAsset struct {
	ID     int64             `json:"id"`
	Size   int64             `json:"size"`
	SHA256 string            `json:"sha256"`
	Files  map[string]string `json:"files,omitempty"`
	Links  map[string]string `json:"links,omitempty"`
}

// InstallAsset records one stable-diffusion.cpp release asset used by an install.
type InstallAsset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// InstallRecord records the release assets installed in a library directory.
type InstallRecord struct {
	Version      int            `json:"version"`
	Tag          string         `json:"tag"`
	Arch         string         `json:"arch"`
	OS           string         `json:"os"`
	Processor    string         `json:"processor"`
	Installed    time.Time      `json:"installed"`
	ManifestHash string         `json:"manifest_sha256,omitempty"`
	Assets       []InstallAsset `json:"assets"`
}

// FileState describes the result of checking one installed path.
type FileState int

const (
	// FileVerified means an installed path matches the trusted manifest.
	FileVerified FileState = iota

	// FileChanged means an installed path does not match the trusted manifest.
	FileChanged

	// FileMissing means a path from the trusted manifest is not installed.
	FileMissing

	// FileUnexpected means an installed path is not in the trusted manifest.
	FileUnexpected
)

// String returns the name of a file verification state.
func (state FileState) String() string {
	switch state {
	case FileVerified:
		return "verified"
	case FileChanged:
		return "changed"
	case FileMissing:
		return "missing"
	case FileUnexpected:
		return "unexpected"
	default:
		return "unknown"
	}
}

// MarshalJSON writes a file state as its name.
func (state FileState) MarshalJSON() ([]byte, error) {
	return json.Marshal(state.String())
}

// FileReport describes the verification state of one installed path.
type FileReport struct {
	Name  string    `json:"name"`
	State FileState `json:"state"`
}

// VerifyReport describes the files checked in an installed library directory.
type VerifyReport struct {
	Tag        string       `json:"tag"`
	LibPath    string       `json:"lib_path"`
	Files      []FileReport `json:"files"`
	Verified   int          `json:"verified"`
	Changed    int          `json:"changed"`
	Missing    int          `json:"missing"`
	Unexpected int          `json:"unexpected"`
}

// OK reports whether every expected installed path is present and unchanged.
func (report *VerifyReport) OK() bool {
	return report.Changed == 0 && report.Missing == 0
}

// ReadInstallRecord reads the Malina install record in libPath.
func ReadInstallRecord(libPath string) (InstallRecord, error) {
	data, err := os.ReadFile(filepath.Join(libPath, InstallRecordName))
	if err != nil {
		return InstallRecord{}, err
	}

	var record InstallRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return InstallRecord{}, fmt.Errorf("decode install record: %w", err)
	}
	if record.Version != installRecordVersion || record.Tag == "" || len(record.Assets) == 0 {
		return InstallRecord{}, fmt.Errorf("%w: invalid record", ErrRecordMismatch)
	}

	return record, nil
}

// VerifyInstall checks an installed library directory against Malina's trusted
// manifest. An empty version uses the release recorded during installation.
func VerifyInstall(ctx context.Context, libPath string, version string) (*VerifyReport, error) {
	tag, manifestSHA, err := ParsePinnedVersion(version)
	if err != nil {
		return nil, err
	}
	record, err := ReadInstallRecord(libPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w in %s", ErrNoInstallRecord, libPath)
		}
		return nil, err
	}
	if tag != "" && record.Tag != tag {
		return nil, fmt.Errorf("%w: installed %s, requested %s", ErrRecordMismatch, record.Tag, tag)
	}
	if manifestSHA != "" && !strings.EqualFold(record.ManifestHash, manifestSHA) {
		return nil, fmt.Errorf("%w: installed manifest digest does not match version pin", ErrRecordMismatch)
	}

	return verifyRecord(ctx, libPath, record)
}

// ParsePinnedVersion splits VERSION@sha256:DIGEST into its bare tag and digest.
// An unpinned version is returned unchanged with an empty digest.
func ParsePinnedVersion(version string) (tag string, digest string, err error) {
	tag, pin, found := strings.Cut(version, "@")
	if !found {
		return version, "", nil
	}
	if tag == "" || tag == "latest" {
		return "", "", fmt.Errorf("%w: a digest needs an exact version", ErrInvalidVersion)
	}
	digest, err = parseSHA256(pin)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrInvalidDigest, err)
	}
	if err := VersionIsValid(tag); err != nil {
		return "", "", err
	}
	return tag, digest, nil
}

func expectedAssetDigest(version string, manifestSHA string, asset releaseAsset) (string, error) {
	if asset.State != "uploaded" {
		return "", fmt.Errorf("release asset %s is not uploaded", asset.Name)
	}
	if err := authenticateManifest(version, manifestSHA); err != nil {
		return "", err
	}

	githubDigest, err := parseSHA256(asset.Digest)
	if err != nil {
		return "", fmt.Errorf("%w for %s: %v", ErrNoAssetDigest, asset.Name, err)
	}

	manifest, ok := trustedManifest(version)
	if !ok {
		if manifestSHA != "" {
			return "", fmt.Errorf("%w: %s", ErrNoFileDigests, version)
		}
		return githubDigest, nil
	}
	entry, ok := manifest.Assets[asset.Name]
	if !ok {
		return "", fmt.Errorf("%w: asset %s", ErrRecordMismatch, asset.Name)
	}
	if entry.ID != asset.ID || entry.Size != asset.Size || !strings.EqualFold(entry.SHA256, githubDigest) {
		return "", fmt.Errorf("%w: release metadata changed for %s", ErrRecordMismatch, asset.Name)
	}

	return entry.SHA256, nil
}

func authenticateManifest(version string, manifestSHA string) error {
	if manifestSHA == "" {
		return nil
	}
	if _, ok := trustedManifest(version); !ok {
		return fmt.Errorf("%w: %s", ErrNoFileDigests, version)
	}
	if !strings.EqualFold(trustedManifestHash(version), manifestSHA) {
		return fmt.Errorf("%w for the trusted manifest of %s", ErrDigestMismatch, version)
	}
	return nil
}

func trustedManifest(version string) (libraryManifest, bool) {
	var manifest libraryManifest
	if err := json.Unmarshal(trustedManifestJSON, &manifest); err != nil {
		return libraryManifest{}, false
	}
	if manifest.Version != 1 || manifest.Repository != SDRepo || manifest.Tag != version {
		return libraryManifest{}, false
	}
	return manifest, true
}

func trustedManifestHash(version string) string {
	if _, ok := trustedManifest(version); !ok {
		return ""
	}
	sum := sha256.Sum256(trustedManifestJSON)
	return hex.EncodeToString(sum[:])
}

func verifyExpectedFiles(ctx context.Context, libPath string, record InstallRecord) error {
	if record.ManifestHash == "" {
		return nil
	}
	report, err := verifyRecord(ctx, libPath, record)
	if err != nil {
		return fmt.Errorf("verify installed libraries: %w", err)
	}
	if !report.OK() {
		return fmt.Errorf("verify installed libraries: %d changed and %d missing files", report.Changed, report.Missing)
	}
	return nil
}

func verifyRecord(ctx context.Context, libPath string, record InstallRecord) (*VerifyReport, error) {
	manifest, ok := trustedManifest(record.Tag)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoFileDigests, record.Tag)
	}
	if record.ManifestHash != trustedManifestHash(record.Tag) {
		return nil, fmt.Errorf("%w: manifest hash", ErrRecordMismatch)
	}
	if err := verifyRecordAssets(record, manifest); err != nil {
		return nil, err
	}

	return verifyFiles(ctx, libPath, record, manifest)
}

func verifyRecordAssets(record InstallRecord, manifest libraryManifest) error {
	arch, err := ParseArch(record.Arch)
	if err != nil {
		return fmt.Errorf("%w: architecture %q", ErrRecordMismatch, record.Arch)
	}
	osVal, err := ParseOS(record.OS)
	if err != nil {
		return fmt.Errorf("%w: operating system %q", ErrRecordMismatch, record.OS)
	}
	processor, err := ParseProcessor(record.Processor)
	if err != nil {
		return fmt.Errorf("%w: processor %q", ErrRecordMismatch, record.Processor)
	}
	pattern, err := assetPattern(arch, osVal, processor)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRecordMismatch, err)
	}

	assets := make([]releaseAsset, 0, len(manifest.Assets))
	for name, asset := range manifest.Assets {
		assets = append(assets, releaseAsset{
			ID:     asset.ID,
			Name:   name,
			Size:   asset.Size,
			Digest: "sha256:" + asset.SHA256,
			State:  "uploaded",
		})
	}
	expected, err := selectAssets(assets, pattern, osVal, processor, record.Tag)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRecordMismatch, err)
	}
	if len(record.Assets) != len(expected) {
		return fmt.Errorf("%w: got %d assets, want %d", ErrRecordMismatch, len(record.Assets), len(expected))
	}
	for i, asset := range expected {
		installed := record.Assets[i]
		if installed.ID != asset.ID || installed.Name != asset.Name || installed.Size != asset.Size || !strings.EqualFold(installed.SHA256, strings.TrimPrefix(asset.Digest, "sha256:")) {
			return fmt.Errorf("%w: asset %s", ErrRecordMismatch, installed.Name)
		}
	}

	return nil
}

func verifyFiles(ctx context.Context, libPath string, record InstallRecord, manifest libraryManifest) (*VerifyReport, error) {
	wantFiles := make(map[string]string)
	wantLinks := make(map[string]string)
	for _, installed := range record.Assets {
		asset, ok := manifest.Assets[installed.Name]
		if !ok || asset.ID != installed.ID || asset.Size != installed.Size || !strings.EqualFold(asset.SHA256, installed.SHA256) {
			return nil, fmt.Errorf("%w: asset %s", ErrRecordMismatch, installed.Name)
		}
		for name, digest := range asset.Files {
			if previous, exists := wantFiles[name]; exists && !strings.EqualFold(previous, digest) {
				return nil, fmt.Errorf("%w: conflicting file %s", ErrRecordMismatch, name)
			}
			wantFiles[name] = digest
		}
		for name, target := range asset.Links {
			if previous, exists := wantLinks[name]; exists && previous != target {
				return nil, fmt.Errorf("%w: conflicting link %s", ErrRecordMismatch, name)
			}
			wantLinks[name] = target
		}
	}
	if len(wantFiles) == 0 && len(wantLinks) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoFileDigests, record.Tag)
	}

	report := VerifyReport{Tag: record.Tag, LibPath: libPath}
	seen := make(map[string]bool)
	err := filepath.WalkDir(libPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		name, err := filepath.Rel(libPath, path)
		if err != nil {
			return err
		}
		name = filepath.ToSlash(name)
		if name == InstallRecordName {
			return nil
		}
		seen[name] = true

		if entry.Type()&fs.ModeSymlink != 0 {
			want, ok := wantLinks[name]
			if !ok {
				report.add(name, FileUnexpected)
				return nil
			}
			got, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if got != want {
				report.add(name, FileChanged)
				return nil
			}
			report.add(name, FileVerified)
			return nil
		}

		want, ok := wantFiles[name]
		if !ok {
			report.add(name, FileUnexpected)
			return nil
		}
		got, err := hashFile(path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(got, want) {
			report.add(name, FileChanged)
			return nil
		}
		report.add(name, FileVerified)
		return nil
	})
	if err != nil {
		return nil, err
	}

	for name := range wantFiles {
		if !seen[name] {
			report.add(name, FileMissing)
		}
	}
	for name := range wantLinks {
		if !seen[name] {
			report.add(name, FileMissing)
		}
	}
	slices.SortFunc(report.Files, func(a FileReport, b FileReport) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return &report, nil
}

func writeInstallRecord(libPath string, record InstallRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode install record: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(libPath, InstallRecordName), data, 0o644); err != nil {
		return fmt.Errorf("write install record: %w", err)
	}
	return nil
}

func parseSHA256(digest string) (string, error) {
	value, ok := strings.CutPrefix(digest, "sha256:")
	if !ok {
		return "", errors.New("digest is not SHA-256")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return "", errors.New("digest is malformed")
	}
	return strings.ToLower(value), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (report *VerifyReport) add(name string, state FileState) {
	report.Files = append(report.Files, FileReport{Name: name, State: state})
	switch state {
	case FileVerified:
		report.Verified++
	case FileChanged:
		report.Changed++
	case FileMissing:
		report.Missing++
	case FileUnexpected:
		report.Unexpected++
	}
}
