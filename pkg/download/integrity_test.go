package download

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrustedManifestMatchesDefaultVersion(t *testing.T) {
	manifest, ok := trustedManifest(DefaultSDVersion)
	if !ok {
		t.Fatalf("trustedManifest(%q): got no manifest", DefaultSDVersion)
	}
	if len(manifest.Assets) == 0 {
		t.Fatal("trusted manifest has no assets")
	}
	for name, asset := range manifest.Assets {
		if asset.ID == 0 || asset.Size == 0 || len(asset.SHA256) != sha256.Size*2 {
			t.Errorf("asset %q has incomplete release metadata", name)
		}
		if len(asset.Files) == 0 && len(asset.Links) == 0 {
			t.Errorf("asset %q has no installed paths", name)
		}
	}
	if got := trustedManifestHash(DefaultSDVersion); len(got) != sha256.Size*2 {
		t.Errorf("trustedManifestHash: got %q", got)
	}
}

func TestExpectedAssetDigestUsesTrustedManifest(t *testing.T) {
	manifest, ok := trustedManifest(DefaultSDVersion)
	if !ok {
		t.Fatal("trusted manifest is unavailable")
	}
	var name string
	var entry manifestAsset
	for name, entry = range manifest.Assets {
		break
	}

	asset := releaseAsset{
		ID:     entry.ID,
		Name:   name,
		Size:   entry.Size,
		Digest: "sha256:" + entry.SHA256,
		State:  "uploaded",
	}
	got, err := expectedAssetDigest(DefaultSDVersion, asset)
	if err != nil {
		t.Fatalf("expectedAssetDigest: %v", err)
	}
	if got != entry.SHA256 {
		t.Errorf("digest: got %q, want %q", got, entry.SHA256)
	}

	asset.Size++
	if _, err := expectedAssetDigest(DefaultSDVersion, asset); !errors.Is(err, ErrRecordMismatch) {
		t.Errorf("changed release metadata error = %v, want ErrRecordMismatch", err)
	}
}

func TestVerifyInstallRequiresRecord(t *testing.T) {
	_, err := VerifyInstall(context.Background(), t.TempDir(), DefaultSDVersion)
	if !errors.Is(err, ErrNoInstallRecord) {
		t.Fatalf("VerifyInstall error = %v, want ErrNoInstallRecord", err)
	}
}

func TestVerifyFiles(t *testing.T) {
	const (
		tag       = "master-1-abcdef0"
		assetName = "libraries.zip"
		filename  = "libstable-diffusion.dylib"
		contents  = "native library"
	)
	digest := sha256.Sum256([]byte(contents))
	manifest := libraryManifest{
		Tag: tag,
		Assets: map[string]manifestAsset{
			assetName: {
				ID:     1,
				Size:   10,
				SHA256: strings.Repeat("a", sha256.Size*2),
				Files: map[string]string{
					filename: hex.EncodeToString(digest[:]),
				},
			},
		},
	}
	record := InstallRecord{
		Tag: tag,
		Assets: []InstallAsset{
			{ID: 1, Name: assetName, Size: 10, SHA256: strings.Repeat("a", sha256.Size*2)},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version.json"), []byte("metadata"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := verifyFiles(context.Background(), dir, record, manifest)
	if err != nil {
		t.Fatalf("verifyFiles: %v", err)
	}
	if !report.OK() || report.Verified != 1 || report.Unexpected != 1 {
		t.Errorf("report = %+v, want one verified and one unexpected file", report)
	}

	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err = verifyFiles(context.Background(), dir, record, manifest)
	if err != nil {
		t.Fatalf("verify changed file: %v", err)
	}
	if report.OK() || report.Changed != 1 {
		t.Errorf("changed report = %+v, want one changed file", report)
	}
}

func TestDownloadAndExtractChecksDigest(t *testing.T) {
	archive := makeZip(t, "libstable-diffusion.dylib", "native library")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	dir := t.TempDir()
	err := downloadAndExtract(context.Background(), server.URL+"/libraries.zip", dir, Darwin, strings.Repeat("0", sha256.Size*2), nil)
	if !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("downloadAndExtract error = %v, want ErrDigestMismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "libstable-diffusion.dylib")); !os.IsNotExist(err) {
		t.Errorf("library was extracted before verification: %v", err)
	}

	digest := sha256.Sum256(archive)
	err = downloadAndExtract(context.Background(), server.URL+"/libraries.zip", dir, Darwin, hex.EncodeToString(digest[:]), nil)
	if err != nil {
		t.Fatalf("downloadAndExtract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "libstable-diffusion.dylib")); err != nil {
		t.Errorf("verified library was not extracted: %v", err)
	}
}

func makeZip(t *testing.T, name string, contents string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "libraries.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
