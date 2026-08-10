package sd

import (
	"os"
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
