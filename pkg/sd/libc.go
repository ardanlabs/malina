package sd

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// =============================================================================
// libc free
//
// stable-diffusion.cpp's generate_image returns an sd_image_t* allocated
// with plain libc calloc and a pixel buffer allocated with plain libc
// malloc (see upstream src/stable-diffusion.cpp decode_image_outputs +
// src/util.cpp tensor_to_sd_image). There is no public free_sd_image
// helper in the C API, so the bindings must pair the C library's
// malloc/calloc with the matching libc free themselves. This file
// resolves that free at Load() time and exposes a small cFree wrapper
// used by GenerateImage to drop the C-heap allocations once the pixels
// have been copied into a Go slice.

// freeFunc is the resolved libc free trampoline. Set once by
// loadLibcFuncs from sd.Load.
var freeFunc ffi.Fun

// loadLibcFuncs dlopens the platform libc and resolves free. Called
// from sd.Load alongside the other loadXxxFuncs helpers.
//
// Single-CRT platforms (Linux, macOS, FreeBSD) are unambiguous: every
// process has exactly one libc loaded and its free pairs with every
// malloc in the address space. Windows binaries can mix CRTs (msvcrt
// vs ucrtbase) where a free from one heap on a pointer allocated by
// the other corrupts state — the candidate list tries the MingW
// default (msvcrt) first because upstream leejet builds with mingw-w64.
func loadLibcFuncs() error {
	candidates := libcCandidates()

	var (
		lib     ffi.Lib
		lastErr error
	)
	for _, name := range candidates {
		l, err := ffi.Load(name)
		if err == nil {
			lib = l
			lastErr = nil
			break
		}
		lastErr = err
	}
	if lastErr != nil {
		return fmt.Errorf("could not load libc (tried %v): %w", candidates, lastErr)
	}

	f, err := lib.Prep("free", &ffi.TypeVoid, &ffi.TypePointer)
	if err != nil {
		return loadError("free", err)
	}
	freeFunc = f
	return nil
}

// libcCandidates returns the platform-specific shared library names to
// try when looking up free. Multiple names are listed where distros
// disagree (glibc soname pinning on Linux; libSystem location on
// macOS; msvcrt vs ucrtbase on Windows).
func libcCandidates() []string {
	switch runtime.GOOS {
	case "darwin":
		// libSystem is the macOS one-stop shop for libc + pthread +
		// dyld. Resolves via the dyld shared cache on Big Sur+ even
		// when the file isn't on disk.
		return []string{"libSystem.B.dylib", "/usr/lib/libSystem.B.dylib"}
	case "linux":
		// glibc ships as libc.so.6; some musl-based distros expose
		// libc.so directly.
		return []string{"libc.so.6", "libc.so"}
	case "freebsd":
		return []string{"libc.so.7", "libc.so"}
	case "windows":
		// Order matters: free MUST be resolved from the same CRT that
		// allocated the pointer, or Windows corrupts its heap and the
		// process dies silently (no Go panic, no test FAIL line).
		//
		// ucrtbase is first because every modern Windows toolchain
		// links it: leejet's upstream releases are MSVC builds whose
		// imports include VCRUNTIME140.dll and the
		// api-ms-win-crt-*-l1-1-0.dll forwarder set, which all point
		// at ucrtbase. Modern mingw-w64 (the -ucrt subtarget that's
		// become default) also links ucrtbase. The legacy msvcrt is
		// retained as a fallback only for ancient mingw builds; it
		// loads successfully on every Windows install, so listing it
		// first would silently win the lookup and produce the heap
		// mismatch described above.
		return []string{"ucrtbase.dll", "msvcrt.dll"}
	default:
		return []string{"libc.so"}
	}
}

// cFree releases a pointer returned by libc malloc/calloc/realloc. Safe
// to call with nil; the libc free call itself also treats NULL as a
// no-op, but checking in Go avoids a wasted FFI round-trip.
//
// The pointer MUST NOT be a Go-allocated address — only pointers handed
// out by C code (e.g. generate_image's sd_image_t* and its Data
// buffer). Passing a Go pointer to libc free is undefined behavior and
// will at minimum trip Go's runtime cgo checker.
func cFree(ptr unsafe.Pointer) {
	if ptr == nil {
		return
	}
	freeFunc.Call(nil, unsafe.Pointer(&ptr))
}
