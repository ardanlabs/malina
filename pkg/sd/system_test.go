package sd

import "testing"

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
	// On every supported platform at least the CPU backend registers
	// itself via a static constructor at libstable-diffusion load time,
	// so the device count must be at least 1.
	if n < 1 {
		t.Fatalf("GGMLBackendDeviceCount = %d, want >= 1", n)
	}
	t.Logf("sd.GGMLBackendDeviceCount = %d", n)
}
