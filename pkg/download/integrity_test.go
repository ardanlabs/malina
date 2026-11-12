package download

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConcurrentLibraryDownloads(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	firstStarted, secondStarted, firstDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	const filename = "sd-master-999-aaaa-bin-Darwin-15.7.7-arm64.zip"
	archive := makeZip(t, "libstable-diffusion.dylib", "native library")
	digest := hashBytes(archive)
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/repos/") {
			first := strings.HasSuffix(r.URL.Path, "master-999-aaaa")
			name, tag := "second.zip", "master-1000-bbbb"
			if first {
				name, tag = "first.zip", "master-999-aaaa"
			}
			if err := json.NewEncoder(w).Encode(map[string]any{
				"tag_name": tag,
				"assets":   []releaseAsset{{ID: 1, Name: filename, Size: int64(len(archive)), Digest: "sha256:" + digest, State: "uploaded", DownloadURL: serverURL + "/" + name}},
			}); err != nil {
				t.Errorf("encode release: %v", err)
			}
			return
		}
		if r.Method == http.MethodHead {
			wait := firstDone
			if r.URL.Path == "/first.zip" {
				close(firstStarted)
				wait = secondStarted
			} else {
				close(secondStarted)
			}
			select {
			case <-wait:
			case <-ctx.Done():
				return
			}
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(archive)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(archive)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	// Only release metadata uses http.DefaultTransport. Asset URLs returned by the
	// local release response exercise the real go-getter download path.
	oldTransport := http.DefaultTransport
	http.DefaultTransport = downloadRoundTripper(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL = r.URL.Clone()
		r.URL.Scheme, r.URL.Host = "http", strings.TrimPrefix(server.URL, "http://")
		return oldTransport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	firstDir, secondDir := t.TempDir(), t.TempDir()
	results := make(chan error, 2)
	go func() {
		defer close(firstDone)
		results <- GetWithContext(ctx, "arm64", "darwin", "metal", "master-999-aaaa", firstDir, libraryProgressCheck{t, "first.zip"})
	}()
	select {
	case <-firstStarted:
	case err := <-results:
		t.Fatalf("first download stopped before HEAD: %v", err)
	case <-ctx.Done():
		t.Fatal("first download did not start")
	}
	go func() {
		results <- GetWithContext(ctx, "arm64", "darwin", "metal", "master-1000-bbbb", secondDir, libraryProgressCheck{t, "second.zip"})
	}()
	for range 2 {
		if err := <-results; err != nil {
			t.Errorf("GetWithContext: %v", err)
		}
	}
	for _, dir := range []string{firstDir, secondDir} {
		got, err := os.ReadFile(filepath.Join(dir, "libstable-diffusion.dylib"))
		if err != nil || string(got) != "native library" {
			t.Errorf("installed library: got %q, %v; want native library", got, err)
		}
	}
}

type downloadRoundTripper func(*http.Request) (*http.Response, error)

func (f downloadRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type libraryProgressCheck struct {
	t    *testing.T
	want string
}

func (p libraryProgressCheck) TrackProgress(src string, _, _ int64, stream io.ReadCloser) io.ReadCloser {
	if src != p.want {
		p.t.Errorf("progress source: got %q, want %q", src, p.want)
	}
	return stream
}

func TestDefaultProgressTrackerIndependentStreams(t *testing.T) {
	tracker := DefaultProgressTracker()
	streams := []struct {
		stream io.ReadCloser
		want   string
	}{
		{tracker.TrackProgress("first", 0, 5, io.NopCloser(strings.NewReader("first"))), "first"},
		{tracker.TrackProgress("second", 0, 6, io.NopCloser(strings.NewReader("second"))), "second"},
	}
	for _, test := range streams {
		got, err := io.ReadAll(test.stream)
		if err != nil || string(got) != test.want {
			t.Errorf("ReadAll: got %q, %v; want %q", got, err, test.want)
		}
		if err := test.stream.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}
}

func TestTrustedManifestMatchesDefaultVersion(t *testing.T) {
	tag, digest, err := ParsePinnedVersion(DefaultSDVersion)
	if err != nil {
		t.Fatalf("ParsePinnedVersion(%q): %v", DefaultSDVersion, err)
	}
	manifest, ok := trustedManifest(tag)
	if !ok {
		t.Fatalf("trustedManifest(%q): got no manifest", tag)
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
	if got := trustedManifestHash(tag); got != digest {
		t.Errorf("trustedManifestHash: got %q, want default pin %q", got, digest)
	}
}

func TestTrustedManifestCoversSupportedMatrix(t *testing.T) {
	tag, _, err := ParsePinnedVersion(DefaultSDVersion)
	if err != nil {
		t.Fatalf("ParsePinnedVersion(%q): %v", DefaultSDVersion, err)
	}

	tests := []struct {
		name       string
		arch       Arch
		os         OS
		processor  Processor
		wantAssets int
	}{
		{"darwin metal", ARM64, Darwin, Metal, 1},
		{"windows cpu", AMD64, Windows, CPU, 1},
		{"windows cuda", AMD64, Windows, CUDA, 2},
		{"windows vulkan", AMD64, Windows, Vulkan, 1},
		{"windows rocm", AMD64, Windows, ROCm, 1},
		{"linux cpu", AMD64, Linux, CPU, 1},
		{"linux vulkan", AMD64, Linux, Vulkan, 1},
		{"linux rocm", AMD64, Linux, ROCm, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assets, releaseMetadata, err := resolveAssets(context.Background(), test.arch, test.os, test.processor, tag)
			if err != nil {
				t.Fatalf("resolveAssets: %v", err)
			}
			if len(assets) != test.wantAssets {
				t.Errorf("asset count: got %d, want %d", len(assets), test.wantAssets)
			}
			if len(releaseMetadata) != 0 {
				t.Errorf("release metadata: got %d bytes, want none for trusted manifest", len(releaseMetadata))
			}
		})
	}
}

func TestExpectedAssetDigestUsesTrustedManifest(t *testing.T) {
	tag, manifestSHA, err := ParsePinnedVersion(DefaultSDVersion)
	if err != nil {
		t.Fatalf("ParsePinnedVersion(%q): %v", DefaultSDVersion, err)
	}
	manifest, ok := trustedManifest(tag)
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
	got, err := expectedAssetDigest(tag, manifestSHA, asset)
	if err != nil {
		t.Fatalf("expectedAssetDigest: %v", err)
	}
	if got != entry.SHA256 {
		t.Errorf("digest: got %q, want %q", got, entry.SHA256)
	}
	if _, err := expectedAssetDigest(tag, strings.Repeat("0", sha256.Size*2), asset); !errors.Is(err, ErrDigestMismatch) {
		t.Errorf("changed manifest pin error = %v, want ErrDigestMismatch", err)
	}
	got, err = expectedAssetDigest("master-999-deadbee", "", asset)
	if err != nil {
		t.Fatalf("unpinned custom version: %v", err)
	}
	if got != entry.SHA256 {
		t.Errorf("unpinned custom digest: got %q, want GitHub digest %q", got, entry.SHA256)
	}
	if _, err := expectedAssetDigest("master-999-deadbee", manifestSHA, asset); !errors.Is(err, ErrNoFileDigests) {
		t.Errorf("pinned custom version error = %v, want ErrNoFileDigests", err)
	}

	asset.Size++
	if _, err := expectedAssetDigest(tag, manifestSHA, asset); !errors.Is(err, ErrRecordMismatch) {
		t.Errorf("changed release metadata error = %v, want ErrRecordMismatch", err)
	}
}

func TestVerifyInstallRequiresRecord(t *testing.T) {
	_, err := VerifyInstall(context.Background(), t.TempDir(), DefaultSDVersion)
	if !errors.Is(err, ErrNoInstallRecord) {
		t.Fatalf("VerifyInstall error = %v, want ErrNoInstallRecord", err)
	}
}

func TestVerifyInstallUsesCustomInstallRecordOffline(t *testing.T) {
	const (
		tag      = "master-999-deadbee"
		filename = "libstable-diffusion.dylib"
		contents = "custom native library"
	)

	dir := t.TempDir()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(contents))
	releaseMetadata := []byte(`{"tag_name":"` + tag + `","assets":[{"id":1,"name":"custom.zip","size":10,"digest":"sha256:` + strings.Repeat("a", sha256.Size*2) + `","state":"uploaded"}]}`)
	record := InstallRecord{
		Version:     installRecordVersion,
		Tag:         tag,
		Arch:        "arm64",
		OS:          "darwin",
		Processor:   "metal",
		ReleaseHash: hashBytes(releaseMetadata),
		Assets: []InstallAsset{{
			ID:     1,
			Name:   "custom.zip",
			Size:   10,
			SHA256: strings.Repeat("a", sha256.Size*2),
			Files:  map[string]string{filename: hex.EncodeToString(digest[:])},
		}},
	}
	if err := writeReleaseMetadata(dir, releaseMetadata); err != nil {
		t.Fatal(err)
	}
	if err := writeInstallRecord(dir, record); err != nil {
		t.Fatal(err)
	}

	report, err := VerifyInstall(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("VerifyInstall: %v", err)
	}
	if !report.OK() || report.Source != "install-record" || report.ManifestAuthenticated || report.Verified != 1 {
		t.Errorf("report = %+v, want one locally verified file", report)
	}

	if err := os.WriteFile(filepath.Join(dir, ReleaseMetadataName), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyInstall(context.Background(), dir, ""); !errors.Is(err, ErrRecordMismatch) {
		t.Fatalf("VerifyInstall changed release metadata error = %v, want ErrRecordMismatch", err)
	}
	if err := writeReleaseMetadata(dir, releaseMetadata); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	report, err = VerifyInstall(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("VerifyInstall changed file: %v", err)
	}
	if report.OK() || report.Changed != 1 {
		t.Errorf("changed report = %+v, want one changed file", report)
	}
}

func TestParsePinnedVersion(t *testing.T) {
	digest := strings.Repeat("a", sha256.Size*2)
	tests := []struct {
		name       string
		version    string
		wantTag    string
		wantDigest string
		wantErr    error
	}{
		{name: "unpinned", version: "master-841-6b3edaa", wantTag: "master-841-6b3edaa"},
		{name: "pinned", version: "master-841-6b3edaa@sha256:" + digest, wantTag: "master-841-6b3edaa", wantDigest: digest},
		{name: "short digest", version: "master-841-6b3edaa@sha256:abcd", wantErr: ErrInvalidDigest},
		{name: "wrong algorithm", version: "master-841-6b3edaa@sha512:" + digest, wantErr: ErrInvalidDigest},
		{name: "latest pin", version: "latest@sha256:" + digest, wantErr: ErrInvalidVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, gotDigest, err := ParsePinnedVersion(tt.version)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ParsePinnedVersion error = %v, want %v", err, tt.wantErr)
			}
			if tag != tt.wantTag || gotDigest != tt.wantDigest {
				t.Errorf("ParsePinnedVersion: got %q/%q, want %q/%q", tag, gotDigest, tt.wantTag, tt.wantDigest)
			}
		})
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
	_, _, err := downloadAndExtract(context.Background(), server.URL+"/libraries.zip", dir, Darwin, strings.Repeat("0", sha256.Size*2), nil)
	if !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("downloadAndExtract error = %v, want ErrDigestMismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "libstable-diffusion.dylib")); !os.IsNotExist(err) {
		t.Errorf("library was extracted before verification: %v", err)
	}

	digest := sha256.Sum256(archive)
	files, links, err := downloadAndExtract(context.Background(), server.URL+"/libraries.zip", dir, Darwin, hex.EncodeToString(digest[:]), nil)
	if err != nil {
		t.Fatalf("downloadAndExtract: %v", err)
	}
	libraryDigest := sha256.Sum256([]byte("native library"))
	if files["libstable-diffusion.dylib"] != hex.EncodeToString(libraryDigest[:]) || len(links) != 0 {
		t.Errorf("recorded paths: got files=%v links=%v", files, links)
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
