package sd

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var loadOnce sync.Once
var loadErr error

func testSetup(t *testing.T) {
	t.Helper()

	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		t.Skip("MALINA_LIB not set; skipping stable-diffusion FFI test")
	}

	loadOnce.Do(func() {
		if loadErr = Load(libPath); loadErr != nil {
			return
		}
		loadErr = Init(libPath)
	})
	if loadErr != nil {
		t.Fatalf("failed to load stable-diffusion.cpp from %s: %v", libPath, loadErr)
	}
}

// testEnvModelFile returns the path stored in env. The test is skipped
// when env is unset and skipped (not failed) when the file is missing,
// so a stale env var on a contributor's machine never breaks the suite.
// Used by the per-bundle smoke tests so each bundle binds to its own
// env var (MALINA_TEST_MODEL for sd-1.5, MALINA_SDXL_TEST_MODEL for
// sdxl-base-1.0) independently.
func testEnvModelFile(t *testing.T, env string) string {
	t.Helper()

	model := os.Getenv(env)
	if model == "" {
		t.Skipf("%s not set; skipping test that requires a model", env)
	}
	if _, err := os.Stat(model); err != nil {
		t.Skipf("model file %q not present: %v", model, err)
	}
	return model
}

// testEnvBundleDir returns the bundle directory stored in env. The test
// is skipped when env is unset and skipped (not failed) when the
// directory or manifest.json inside it are missing. Used by the flux2
// smoke test which loads three files (diffusion + VAE + LLM) via the
// bundle's manifest.json rather than a single model path.
func testEnvBundleDir(t *testing.T, env string) string {
	t.Helper()

	dir := os.Getenv(env)
	if dir == "" {
		t.Skipf("%s not set; skipping test that requires a model bundle", env)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Skipf("bundle dir %q not present: %v", dir, err)
	}
	if !info.IsDir() {
		t.Skipf("bundle path %q is not a directory", dir)
	}
	manifest := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Skipf("bundle manifest %q not present: %v (did you run `malina model pull`?)", manifest, err)
	}
	return dir
}
