//go:build ignore

// This program generates the trusted library manifest for a stable-diffusion.cpp
// release. Run it from this directory when DefaultSDVersion changes:
//
//	go run generate_manifest.go -version master-841-6b3edaa
package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const repository = "leejet/stable-diffusion.cpp"

type releaseAsset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Digest      string `json:"digest"`
	State       string `json:"state"`
	DownloadURL string `json:"browser_download_url"`
}

type manifest struct {
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

func main() {
	version := flag.String("version", "", "stable-diffusion.cpp release tag")
	output := flag.String("output", "library_manifest.json", "output manifest")
	flag.Parse()

	if *version == "" {
		fmt.Fprintln(os.Stderr, "-version is required")
		os.Exit(2)
	}

	if err := generate(context.Background(), *version, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(ctx context.Context, version string, output string) error {
	assets, err := releaseAssets(ctx, version)
	if err != nil {
		return err
	}

	m := manifest{
		Version:    1,
		Repository: repository,
		Tag:        version,
		Generated:  time.Now().UTC(),
		Assets:     make(map[string]manifestAsset),
	}

	for _, asset := range assets {
		if asset.State != "uploaded" || !isLibraryAsset(asset.Name) {
			continue
		}

		fmt.Fprintf(os.Stderr, "hashing %s\n", asset.Name)
		entry, err := inspectAsset(ctx, asset)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", asset.Name, err)
		}
		m.Assets[asset.Name] = entry
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func releaseAssets(ctx context.Context, version string) ([]releaseAsset, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repository, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API status %s", resp.Status)
	}

	var release struct {
		Assets []releaseAsset `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return release.Assets, nil
}

func inspectAsset(ctx context.Context, asset releaseAsset) (manifestAsset, error) {
	dir, err := os.MkdirTemp("", "malina-manifest-")
	if err != nil {
		return manifestAsset{}, err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, asset.Name)
	gotDigest, err := download(ctx, asset.DownloadURL, path)
	if err != nil {
		return manifestAsset{}, err
	}
	wantDigest, ok := strings.CutPrefix(asset.Digest, "sha256:")
	if !ok || len(wantDigest) != sha256.Size*2 {
		return manifestAsset{}, fmt.Errorf("invalid GitHub digest %q", asset.Digest)
	}
	if !strings.EqualFold(gotDigest, wantDigest) {
		return manifestAsset{}, fmt.Errorf("archive digest got %s, want %s", gotDigest, wantDigest)
	}

	files, links, err := inspectZip(path)
	if err != nil {
		return manifestAsset{}, err
	}

	return manifestAsset{
		ID:     asset.ID,
		Size:   asset.Size,
		SHA256: wantDigest,
		Files:  files,
		Links:  links,
	}, nil
}

func download(ctx context.Context, url string, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download status %s", resp.Status)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, h), resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func inspectZip(path string) (map[string]string, map[string]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, err
	}
	defer zr.Close()

	files := make(map[string]string)
	links := make(map[string]string)
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if !isSharedLibrary(name) {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}

		if f.Mode()&os.ModeSymlink != 0 {
			target, err := io.ReadAll(rc)
			if err != nil {
				rc.Close()
				return nil, nil, err
			}
			if err := rc.Close(); err != nil {
				return nil, nil, err
			}
			links[name] = strings.TrimSpace(string(target))
			continue
		}

		h := sha256.New()
		_, copyErr := io.Copy(h, rc)
		closeErr := rc.Close()
		if copyErr != nil {
			return nil, nil, copyErr
		}
		if closeErr != nil {
			return nil, nil, closeErr
		}
		files[name] = hex.EncodeToString(h.Sum(nil))
	}

	return files, links, nil
}

func isLibraryAsset(name string) bool {
	return strings.HasSuffix(name, ".zip") && (strings.HasPrefix(name, "sd-") || strings.HasPrefix(name, "cudart-sd-"))
}

func isSharedLibrary(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, ".dll") || strings.HasSuffix(name, ".dylib") || strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.")
}
