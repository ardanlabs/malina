package download

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	getter "github.com/hashicorp/go-getter"
)

// ErrBundleNotFound is returned by GetBundle when name is not in the
// curated catalog.
var ErrBundleNotFound = errors.New("bundle not found")

// DefaultModelsDir returns the default malina models directory under the
// user's home (~/models). Bundles are written to a subdirectory named after
// the bundle (e.g. ~/models/sd-1.5/). Mirrors bucky's layout so a single
// ~/models tree can serve both packages.
func DefaultModelsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "models")
	}
	return filepath.Join(homeDir, "models")
}

// Manifest is the JSON written alongside a downloaded bundle so consumers
// (kronk, examples, CLI) can resolve role → on-disk path without having to
// reach back into the catalog. The file is written as manifest.json inside
// the bundle directory.
type Manifest struct {
	Bundle  string            `json:"bundle"`
	License string            `json:"license"`
	Gated   bool              `json:"gated"`
	Files   map[string]string `json:"files"` // role → absolute path
}

// ManifestFilename is the well-known name of the manifest file written
// inside each bundle directory.
const ManifestFilename = "manifest.json"

// GetBundle downloads every file in the named bundle into a subdirectory of
// dest named after the bundle, then writes a manifest.json mapping roles to
// absolute file paths. Partial files left over from interrupted downloads
// are resumed (HTTP Range), and fully-present files are skipped.
//
// dest may be empty, in which case DefaultModelsDir() is used.
func GetBundle(ctx context.Context, name, dest string) (Manifest, error) {
	return GetBundleWithProgress(ctx, name, dest, ProgressTracker)
}

// GetBundleWithProgress is GetBundle with a caller-supplied progress
// tracker. Pass nil to suppress progress output entirely.
func GetBundleWithProgress(ctx context.Context, name, dest string, progress getter.ProgressTracker) (Manifest, error) {
	b, ok := BundleByName(name)
	if !ok {
		return Manifest{}, fmt.Errorf("%w: %q", ErrBundleNotFound, name)
	}

	if dest == "" {
		dest = DefaultModelsDir()
	}

	bundleDir := filepath.Join(dest, b.Name)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("get-bundle: create %s: %w", bundleDir, err)
	}

	manifest := Manifest{
		Bundle:  b.Name,
		License: b.License,
		Gated:   b.Gated,
		Files:   make(map[string]string, len(b.Files)),
	}

	for _, f := range b.Files {
		target := filepath.Join(bundleDir, f.Filename)
		abs, _ := filepath.Abs(target)
		manifest.Files[string(f.Role)] = abs

		if err := getFile(ctx, f, target, progress); err != nil {
			return Manifest{}, fmt.Errorf("get-bundle %q: %s (%s): %w", b.Name, f.Filename, f.Role, err)
		}
	}

	manifestPath := filepath.Join(bundleDir, ManifestFilename)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, fmt.Errorf("get-bundle: marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return manifest, fmt.Errorf("get-bundle: write %s: %w", manifestPath, err)
	}
	return manifest, nil
}

// LoadManifest reads the manifest.json inside bundleDir and returns it.
func LoadManifest(bundleDir string) (Manifest, error) {
	manifestPath := filepath.Join(bundleDir, ManifestFilename)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("load-manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("load-manifest: parse %s: %w", manifestPath, err)
	}
	return m, nil
}

// =============================================================================

// getFile downloads a single bundle file via hashicorp/go-getter, which:
//   - issues HTTP Range requests when a partial file exists at target,
//     so interrupted downloads resume instead of restarting,
//   - sets Authorization: Bearer $HF_TOKEN when present, for gated repos,
//   - appends ?archive=false so go-getter does not try to auto-extract a
//     .gguf / .safetensors based on its extension.
func getFile(ctx context.Context, f BundleFile, target string, progress getter.ProgressTracker) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}

	src := f.URL
	if !strings.Contains(src, "?") {
		src += "?archive=false"
	} else {
		src += "&archive=false"
	}

	header := http.Header{}
	if tok := os.Getenv("HF_TOKEN"); tok != "" {
		header.Set("Authorization", "Bearer "+tok)
	}

	httpGetter := &getter.HttpGetter{
		Header: header,
	}

	client := &getter.Client{
		Ctx:  ctx,
		Src:  src,
		Dst:  target,
		Mode: getter.ClientModeFile,
		Getters: map[string]getter.Getter{
			"http":  httpGetter,
			"https": httpGetter,
		},
	}
	if progress != nil {
		client.ProgressListener = progress
	}

	if err := client.Get(); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "401") || strings.Contains(msg, "403") {
			return fmt.Errorf("%w: this file is license-gated. "+
				"Accept the license on the upstream Hugging Face page in your browser, "+
				"then set HF_TOKEN to a token with read access", err)
		}
		return err
	}
	return nil
}
