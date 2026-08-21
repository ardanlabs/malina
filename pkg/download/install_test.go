package download

import (
	"errors"
	"slices"
	"testing"
)

// TestDefaultSDVersion guards the stable-diffusion.cpp release whose ABI
// Malina supports and installs by default.
func TestDefaultSDVersion(t *testing.T) {
	const want = "master-827-97d2990"

	if DefaultSDVersion != want {
		t.Errorf("DefaultSDVersion: got %q, want %q", DefaultSDVersion, want)
	}
}

// TestVersionIsValid covers the leejet release tag shape check. Empty,
// pure-numeric (no "v" prefix and no "-"), and the literal "latest" must
// be rejected; "master-N-sha" and "vX.Y.Z" must be accepted.
func TestVersionIsValid(t *testing.T) {
	tests := []struct {
		version string
		wantErr bool
	}{
		{"master-656-0e4ee04", false},
		{"v0.9.0", false},
		{"v1.0", false},
		{"1.0.0", true}, // missing "v" prefix and no "-"
		{"latest", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			err := VersionIsValid(tt.version)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.version)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error for %q, got %v", tt.version, err)
			}
		})
	}
}

// TestLibraryName covers the per-OS shared-library filename returned by
// LibraryName, including the "unknown" sentinel for unsupported OSes.
func TestLibraryName(t *testing.T) {
	tests := []struct {
		os   string
		want string
	}{
		{"linux", "libstable-diffusion.so"},
		{"darwin", "libstable-diffusion.dylib"},
		{"windows", "stable-diffusion.dll"},
		{"plan9", "unknown"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			got := LibraryName(tt.os)
			if got != tt.want {
				t.Errorf("LibraryName(%q) = %q, want %q", tt.os, got, tt.want)
			}
		})
	}
}

// TestParseHelpers exercises the happy and error paths of the three enum
// parsers used to validate `malina install` flags.
func TestParseHelpers(t *testing.T) {
	t.Run("ParseArch", func(t *testing.T) {
		for _, v := range []string{"amd64", "arm64"} {
			if _, err := ParseArch(v); err != nil {
				t.Errorf("ParseArch(%q): %v", v, err)
			}
		}
		if _, err := ParseArch("nope"); err == nil {
			t.Error("ParseArch(nope) should fail")
		}
	})
	t.Run("ParseOS", func(t *testing.T) {
		for _, v := range []string{"linux", "darwin", "windows"} {
			if _, err := ParseOS(v); err != nil {
				t.Errorf("ParseOS(%q): %v", v, err)
			}
		}
		if _, err := ParseOS("nope"); err == nil {
			t.Error("ParseOS(nope) should fail")
		}
	})
	t.Run("ParseProcessor", func(t *testing.T) {
		for _, v := range []string{"cpu", "cuda", "metal", "vulkan", "rocm"} {
			if _, err := ParseProcessor(v); err != nil {
				t.Errorf("ParseProcessor(%q): %v", v, err)
			}
		}
		if _, err := ParseProcessor("nope"); err == nil {
			t.Error("ParseProcessor(nope) should fail")
		}
	})
}

// TestAssetPattern verifies the per-platform asset-name regex produced by
// assetPattern matches the asset names leejet/stable-diffusion.cpp actually
// publishes for every supported (Arch, OS, Processor) combination and
// rejects neighbouring/unrelated assets. Error paths assert the right
// sentinel error is wrapped.
func TestAssetPattern(t *testing.T) {
	matchTests := []struct {
		name    string
		arch    Arch
		os      OS
		proc    Processor
		matches []string // should be accepted by the regex
		rejects []string // should be rejected by the regex
	}{
		{
			name: "darwin arm64 cpu",
			arch: ARM64, os: Darwin, proc: CPU,
			matches: []string{
				"sd-master-656-0e4ee04-bin-Darwin-15.7.7-arm64.zip",
				"sd-master-700-deadbee-bin-Darwin-14.6.1-arm64.zip",
				"sd-master-97d2990-bin-Darwin-macOS-26.5.2-arm64.zip",
			},
			rejects: []string{
				"sd-master-656-0e4ee04-bin-Darwin-15.7.7-x86_64.zip",
				"sd-master-656-0e4ee04-bin-Linux-Ubuntu-24.04-x86_64.zip",
			},
		},
		{
			name: "darwin arm64 metal",
			arch: ARM64, os: Darwin, proc: Metal,
			matches: []string{"sd-master-97d2990-bin-Darwin-macOS-26.5.2-arm64.zip"},
		},
		{
			name: "windows amd64 cpu",
			arch: AMD64, os: Windows, proc: CPU,
			matches: []string{
				"sd-master-656-0e4ee04-bin-win-avx2-x64.zip",
				"sd-master-97d2990-bin-win-cpu-x64.zip",
			},
			rejects: []string{
				"sd-master-656-0e4ee04-bin-win-cuda12-x64.zip",
				"sd-master-656-0e4ee04-bin-win-vulkan-x64.zip",
			},
		},
		{
			name: "windows amd64 cuda",
			arch: AMD64, os: Windows, proc: CUDA,
			matches: []string{"sd-master-97d2990-bin-win-cuda12-x64.zip"},
			rejects: []string{"sd-master-656-0e4ee04-bin-win-avx2-x64.zip"},
		},
		{
			name: "windows amd64 vulkan",
			arch: AMD64, os: Windows, proc: Vulkan,
			matches: []string{"sd-master-97d2990-bin-win-vulkan-x64.zip"},
		},
		{
			name: "windows amd64 rocm",
			arch: AMD64, os: Windows, proc: ROCm,
			matches: []string{
				"sd-master-97d2990-bin-win-rocm-7.14.0-x64.zip",
				"sd-master-656-0e4ee04-bin-win-rocm-7.2.1-x64.zip",
			},
		},
		{
			name: "linux amd64 cpu",
			arch: AMD64, os: Linux, proc: CPU,
			matches: []string{"sd-master-97d2990-bin-Linux-Ubuntu-24.04-x86_64.zip"},
			rejects: []string{
				"sd-master-656-0e4ee04-bin-Linux-Ubuntu-24.04-x86_64-vulkan.zip",
				"sd-master-656-0e4ee04-bin-Linux-Ubuntu-24.04-x86_64-rocm-7.2.1.zip",
			},
		},
		{
			name: "linux amd64 vulkan",
			arch: AMD64, os: Linux, proc: Vulkan,
			matches: []string{"sd-master-97d2990-bin-Linux-Ubuntu-24.04-x86_64-vulkan.zip"},
			rejects: []string{"sd-master-656-0e4ee04-bin-Linux-Ubuntu-24.04-x86_64.zip"},
		},
		{
			name: "linux amd64 rocm",
			arch: AMD64, os: Linux, proc: ROCm,
			matches: []string{
				"sd-master-97d2990-bin-Linux-Ubuntu-24.04-x86_64-rocm-7.14.0.zip",
				"sd-master-656-0e4ee04-bin-Linux-Ubuntu-24.04-x86_64-rocm-7.2.1.zip",
			},
		},
	}
	for _, tt := range matchTests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := assetPattern(tt.arch, tt.os, tt.proc)
			if err != nil {
				t.Fatalf("assetPattern: unexpected error: %v", err)
			}
			for _, name := range tt.matches {
				if !re.MatchString(name) {
					t.Errorf("regex %q: expected to match %q", re, name)
				}
			}
			for _, name := range tt.rejects {
				if re.MatchString(name) {
					t.Errorf("regex %q: expected to reject %q", re, name)
				}
			}
		})
	}

	errTests := []struct {
		name    string
		arch    Arch
		os      OS
		proc    Processor
		wantErr error
	}{
		{
			name: "darwin amd64 unsupported",
			arch: AMD64, os: Darwin, proc: CPU,
			wantErr: ErrUnsupportedPlatform,
		},
		{
			name: "darwin cuda unsupported",
			arch: ARM64, os: Darwin, proc: CUDA,
			wantErr: ErrUnknownProcessor,
		},
		{
			name: "windows arm64 unsupported",
			arch: ARM64, os: Windows, proc: CPU,
			wantErr: ErrUnsupportedPlatform,
		},
		{
			name: "windows metal unsupported",
			arch: AMD64, os: Windows, proc: Metal,
			wantErr: ErrUnknownProcessor,
		},
		{
			name: "linux arm64 unsupported",
			arch: ARM64, os: Linux, proc: CPU,
			wantErr: ErrUnsupportedPlatform,
		},
		{
			name: "linux cuda unsupported",
			arch: AMD64, os: Linux, proc: CUDA,
			wantErr: ErrUnknownProcessor,
		},
		{
			name: "linux metal unsupported",
			arch: AMD64, os: Linux, proc: Metal,
			wantErr: ErrUnknownProcessor,
		},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := assetPattern(tt.arch, tt.os, tt.proc)
			if err == nil {
				t.Fatalf("expected error wrapping %v, got nil", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(%v), got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSelectAssetURLsWindowsCUDA(t *testing.T) {
	pattern, err := assetPattern(AMD64, Windows, CUDA)
	if err != nil {
		t.Fatalf("assetPattern: unexpected error: %v", err)
	}

	assets := []releaseAsset{
		{Name: "sd-master-97d2990-bin-win-cuda12-x64.zip", DownloadURL: "https://example.com/stable-diffusion.zip"},
		{Name: "cudart-sd-bin-win-cu12-x64.zip", DownloadURL: "https://example.com/cudart.zip"},
		{Name: "sd-master-97d2990-bin-win-cpu-x64.zip", DownloadURL: "https://example.com/cpu.zip"},
	}
	want := []string{"https://example.com/stable-diffusion.zip", "https://example.com/cudart.zip"}

	got, err := selectAssetURLs(assets, pattern, Windows, CUDA, "master-827-97d2990")
	if err != nil {
		t.Fatalf("selectAssetURLs: unexpected error: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("selectAssetURLs: got %v, want %v", got, want)
	}
}
