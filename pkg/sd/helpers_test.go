package sd

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/ardanlabs/malina/pkg/download"
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
		if loadErr = download.VerifyDefaultInstall(context.Background(), libPath); loadErr != nil {
			return
		}
		if loadErr = Load(libPath); loadErr != nil {
			return
		}
		loadErr = Init(libPath)
	})
	if loadErr != nil {
		t.Fatalf("failed to verify and load download.DefaultSDVersion from %s: %v", libPath, loadErr)
	}
}
