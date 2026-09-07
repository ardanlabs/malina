package download

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultModelsDir(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	want := filepath.Join(homeDir, ".kronk", "malina-models")
	if got := DefaultModelsDir(); got != want {
		t.Errorf("DefaultModelsDir: got %q, want %q", got, want)
	}
}

func TestDefaultLibrariesDir(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	processor := CPU.String()
	if runtime.GOOS == Darwin.String() {
		processor = Metal.String()
	}
	want := filepath.Join(homeDir, ".kronk", "malina-libraries", runtime.GOOS, runtime.GOARCH, processor)
	if got := DefaultLibrariesDir(); got != want {
		t.Errorf("DefaultLibrariesDir: got %q, want %q", got, want)
	}
}

func TestGetBundleSkipsInstalledBundle(t *testing.T) {
	bundle, ok := BundleByName("sd-1.5")
	if !ok {
		t.Fatal("BundleByName: sd-1.5 not found")
	}

	dest := t.TempDir()
	bundleDir := filepath.Join(dest, bundle.Name)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	modelPath := filepath.Join(bundleDir, bundle.Files[0].Filename)
	if err := os.WriteFile(modelPath, []byte("installed"), 0o644); err != nil {
		t.Fatalf("WriteFile model: %v", err)
	}

	manifest := Manifest{
		Bundle:  bundle.Name,
		License: bundle.License,
		Gated:   bundle.Gated,
		Files: map[string]string{
			string(bundle.Files[0].Role): modelPath,
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, ManifestFilename), data, 0o644); err != nil {
		t.Fatalf("WriteFile manifest: %v", err)
	}

	got, err := GetBundleWithProgress(context.Background(), bundle.Name, dest, nil)
	if err != nil {
		t.Fatalf("GetBundleWithProgress: %v", err)
	}
	if !reflect.DeepEqual(got, manifest) {
		t.Errorf("manifest: got %#v, want %#v", got, manifest)
	}
}

// TestGetFileSuccess drives getFile against an in-process server and
// verifies the bytes land on disk intact.
func TestGetFileSuccess(t *testing.T) {
	payload := []byte("hello stable-diffusion")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "22")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "fake.gguf")
	f := BundleFile{
		Role:     RoleModel,
		Filename: "fake.gguf",
		URL:      srv.URL + "/fake.gguf",
	}
	if err := getFile(context.Background(), f, target, nil); err != nil {
		t.Fatalf("getFile: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !reflect.DeepEqual(got, payload) {
		t.Errorf("payload: got %q, want %q", got, payload)
	}
}

// TestGetFileSendsAuthorizationHeader verifies HF_TOKEN propagates through
// to the HTTP request as a Bearer Authorization header. Gated bundles
// (FLUX.2) rely on this; if go-getter changes header handling or our
// glue regresses, real downloads silently get 401/403 errors.
func TestGetFileSendsAuthorizationHeader(t *testing.T) {
	const wantAuth = "Bearer hf_token_for_test_only"
	t.Setenv("HF_TOKEN", "hf_token_for_test_only")

	var gotAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "gated.gguf")
	f := BundleFile{
		Role:     RoleModel,
		Filename: "gated.gguf",
		URL:      srv.URL + "/gated.gguf",
	}
	if err := getFile(context.Background(), f, target, nil); err != nil {
		t.Fatalf("getFile: %v", err)
	}

	if got := gotAuth.Load(); got != wantAuth {
		t.Errorf("Authorization header: got %q, want %q", got, wantAuth)
	}
}

// TestGetFileContextCanceled drives getFile against a server that blocks
// forever, cancels the context mid-flight, and verifies the call aborts.
// A bug that severs ctx propagation through go-getter would block this
// test until the test framework's deadline expires; that is the
// regression we want to fast-fail.
func TestGetFileContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	target := filepath.Join(t.TempDir(), "slow.gguf")
	f := BundleFile{
		Role:     RoleModel,
		Filename: "slow.gguf",
		URL:      srv.URL + "/slow.gguf",
	}

	done := make(chan error, 1)
	go func() { done <- getFile(ctx, f, target, nil) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("getFile: expected error after ctx cancel, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("getFile: did not return within 5s after ctx cancel")
	}
}

// TestGetFileGatedError verifies the unwrapped 401/403 hint that tells
// the user to accept the upstream license and set HF_TOKEN. Without this
// wrapping, a contributor hitting a license-gated file would just see
// the raw go-getter HTTP error and not know what to do.
func TestGetFileGatedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	target := filepath.Join(t.TempDir(), "gated.gguf")
	f := BundleFile{
		Role:     RoleModel,
		Filename: "gated.gguf",
		URL:      srv.URL + "/gated.gguf",
	}

	err := getFile(context.Background(), f, target, nil)
	if err == nil {
		t.Fatal("getFile: expected error on 403, got nil")
	}
	if !strings.Contains(err.Error(), "HF_TOKEN") {
		t.Errorf("error %q should hint about HF_TOKEN", err)
	}
}

// TestLoadManifestRoundTrip writes a manifest JSON to disk, reads it back
// via LoadManifest, and asserts every field round-trips. This is the
// boundary downstream consumers and examples rely on to wire bundle paths
// into pkg/sd.
func TestLoadManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Manifest{
		Bundle:  "animatediff-sd1.5",
		License: "CreativeML Open RAIL-M / Apache-2.0",
		Gated:   false,
		Files: map[string]string{
			"model":         filepath.Join(dir, "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf"),
			"motion_module": filepath.Join(dir, "mm_sd15_v3.safetensors"),
		},
	}

	b, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFilename), b, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("manifest mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

// TestLoadManifestMissing verifies LoadManifest returns a useful error
// when no manifest exists in the directory.
func TestLoadManifestMissing(t *testing.T) {
	_, err := LoadManifest(t.TempDir())
	if err == nil {
		t.Fatal("LoadManifest: expected error, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected errors.Is(os.ErrNotExist), got %v", err)
	}
}
