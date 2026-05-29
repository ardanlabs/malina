package loader

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetLibraryFilename(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		lib      string
		expected map[string]string // OS -> expected result
	}{
		{
			name: "stable-diffusion library",
			path: "/usr/local/lib",
			lib:  "stable-diffusion",
			expected: map[string]string{
				"linux":   "/usr/local/lib/libstable-diffusion.so",
				"freebsd": "/usr/local/lib/libstable-diffusion.so",
				"darwin":  "/usr/local/lib/libstable-diffusion.dylib",
				"windows": "/usr/local/lib/stable-diffusion.dll",
			},
		},
		{
			name: "ggml library",
			path: "/opt/malina",
			lib:  "ggml",
			expected: map[string]string{
				"linux":   "/opt/malina/libggml.so",
				"freebsd": "/opt/malina/libggml.so",
				"darwin":  "/opt/malina/libggml.dylib",
				"windows": "/opt/malina/ggml.dll",
			},
		},
		{
			name: "ggml-cpu library",
			path: "/home/user/libs",
			lib:  "ggml-cpu",
			expected: map[string]string{
				"linux":   "/home/user/libs/libggml-cpu.so",
				"freebsd": "/home/user/libs/libggml-cpu.so",
				"darwin":  "/home/user/libs/libggml-cpu.dylib",
				"windows": "/home/user/libs/ggml-cpu.dll",
			},
		},
		{
			name: "empty path",
			path: "",
			lib:  "stable-diffusion",
			expected: map[string]string{
				"linux":   "libstable-diffusion.so",
				"freebsd": "libstable-diffusion.so",
				"darwin":  "libstable-diffusion.dylib",
				"windows": "stable-diffusion.dll",
			},
		},
		{
			name: "relative path",
			path: "./lib",
			lib:  "stable-diffusion",
			expected: map[string]string{
				"linux":   "lib/libstable-diffusion.so",
				"freebsd": "lib/libstable-diffusion.so",
				"darwin":  "lib/libstable-diffusion.dylib",
				"windows": "lib/stable-diffusion.dll",
			},
		},
		{
			name: "path with spaces",
			path: "/path/with spaces",
			lib:  "stable-diffusion",
			expected: map[string]string{
				"linux":   "/path/with spaces/libstable-diffusion.so",
				"freebsd": "/path/with spaces/libstable-diffusion.so",
				"darwin":  "/path/with spaces/libstable-diffusion.dylib",
				"windows": "/path/with spaces/stable-diffusion.dll",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetLibraryFilename(tt.path, tt.lib)

			expected, ok := tt.expected[runtime.GOOS]
			if !ok {
				if result == "" {
					t.Error("expected non-empty result for unsupported OS")
				}
				return
			}

			expectedNorm := filepath.FromSlash(expected)
			if result != expectedNorm {
				t.Errorf("expected '%s', got '%s'", expectedNorm, result)
			}
		})
	}
}

func TestGetLibraryFilename_CurrentOS(t *testing.T) {
	path := "/test/path"
	lib := "testlib"

	result := GetLibraryFilename(path, lib)

	switch runtime.GOOS {
	case "linux", "freebsd":
		expected := filepath.Join(path, "libtestlib.so")
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	case "darwin":
		expected := filepath.Join(path, "libtestlib.dylib")
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	case "windows":
		expected := filepath.Join(path, "testlib.dll")
		if result != expected {
			t.Errorf("expected '%s', got '%s'", expected, result)
		}
	}
}

// TestLoadLibrary_MissingPath verifies LoadLibrary returns a useful error
// (and does not panic) when neither path nor MALINA_LIB is supplied.
func TestLoadLibrary_MissingPath(t *testing.T) {
	t.Setenv("MALINA_LIB", "")

	_, err := LoadLibrary("", "stable-diffusion")
	if err == nil {
		t.Fatal("LoadLibrary(\"\", ...): expected error, got nil")
	}
	if !strings.Contains(err.Error(), "MALINA_LIB") {
		t.Errorf("error message %q should mention MALINA_LIB", err)
	}
}

// TestLoadLibrary_BadPath verifies LoadLibrary returns an error (and does
// not panic) when the path is set but no library exists at the expected
// filename. dlopen / LoadLibrary on every supported OS reports a clear
// failure here; we just need to confirm the wrapper surfaces it.
func TestLoadLibrary_BadPath(t *testing.T) {
	tmp := t.TempDir()

	_, err := LoadLibrary(tmp, "this-library-does-not-exist")
	if err == nil {
		t.Fatalf("LoadLibrary(%q, ...): expected error, got nil", tmp)
	}
}

// TestLoadLibrary_Success verifies LoadLibrary returns a usable handle for
// the real libstable-diffusion when MALINA_LIB is set. Skipped otherwise
// so this file passes in environments without the C library installed.
func TestLoadLibrary_Success(t *testing.T) {
	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		t.Skip("MALINA_LIB not set; skipping LoadLibrary success test")
	}

	if _, err := LoadLibrary(libPath, "stable-diffusion"); err != nil {
		t.Fatalf("LoadLibrary(%q, stable-diffusion): %v", libPath, err)
	}
}

func TestGetLibraryFilename_DifferentLibNames(t *testing.T) {
	libs := []string{"stable-diffusion", "ggml", "ggml-base", "ggml-cpu"}
	basePath := "/lib"

	for _, lib := range libs {
		t.Run(lib, func(t *testing.T) {
			result := GetLibraryFilename(basePath, lib)

			if result == "" {
				t.Errorf("expected non-empty result for lib '%s'", lib)
			}

			expectedPrefix := filepath.FromSlash(basePath)
			if len(result) < len(expectedPrefix) || result[:len(expectedPrefix)] != expectedPrefix {
				t.Errorf("expected path to start with '%s', got '%s'", expectedPrefix, result)
			}

			if !strings.Contains(result, lib) {
				t.Errorf("expected result to contain '%s', got '%s'", lib, result)
			}
		})
	}
}
