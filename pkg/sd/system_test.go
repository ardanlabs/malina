package sd

import (
	"testing"
)

func TestVersion(t *testing.T) {
	testSetup(t)

	v := Version()
	// stable-diffusion.cpp returns a non-empty version string for both
	// tagged and untagged builds (untagged returns "unknown"). Empty
	// would mean the FFI return-value marshalling broke.
	if v == "" {
		t.Fatal("Version returned empty string")
	}
	t.Logf("sd.Version = %q", v)
}

func TestSystemInfo(t *testing.T) {
	testSetup(t)

	info := SystemInfo()
	if info == "" {
		t.Fatal("SystemInfo returned empty string")
	}
	t.Logf("sd.SystemInfo = %q", info)
}

func TestNumPhysicalCores(t *testing.T) {
	testSetup(t)

	n := NumPhysicalCores()
	if n <= 0 {
		t.Fatalf("NumPhysicalCores = %d, want > 0", n)
	}
	t.Logf("sd.NumPhysicalCores = %d", n)
}

func TestGGMLBackendDeviceCount(t *testing.T) {
	testSetup(t)

	n := GGMLBackendDeviceCount()
	// -1 means the underlying ggml_backend_dev_count symbol is not exported
	// by the loaded libstable-diffusion. leejet's official Windows DLL is the
	// known case: ggml is statically linked but GGML_API expands to nothing
	// on a non-GGML_SHARED PE/COFF build, so the symbol is unreachable via
	// GetProcAddress. The CPU backend still self-registers at DLL load —
	// we just can't observe it through this API on that platform.
	if n == -1 {
		t.Skip("ggml_backend_dev_count not exported by this libstable-diffusion build")
	}
	// On every other supported platform at least the CPU backend registers
	// itself via a static constructor at libstable-diffusion load time,
	// so the device count must be at least 1.
	if n < 1 {
		t.Fatalf("GGMLBackendDeviceCount = %d, want >= 1", n)
	}
	t.Logf("sd.GGMLBackendDeviceCount = %d", n)
}

// NOTE: there is intentionally no concurrent-access test for Context.
// Upstream stable-diffusion.cpp is not safe to call concurrently on the
// same sd_ctx_t (see the doc comment on Context in sd.go). Callers that
// want parallel generation must allocate one Context per goroutine; that
// pattern is exercised by the existing single-instance tests, so a
// multi-instance test would only burn CI minutes loading N copies of the
// same model without verifying any new contract.
