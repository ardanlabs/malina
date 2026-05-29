package sd

import (
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

var (
	// SD_API const char* sd_version(void);
	versionFunc ffi.Fun

	// SD_API const char* sd_get_system_info(void);
	getSystemInfoFunc ffi.Fun

	// SD_API int32_t sd_get_num_physical_cores(void);
	getNumPhysicalCoresFunc ffi.Fun

	// GGML_API size_t ggml_backend_dev_count(void);
	ggmlBackendDevCountFunc ffi.Fun

	// GGML_API void ggml_backend_load_all_from_path(const char * dir_path);
	//
	// Re-exported transitively via libstable-diffusion. May be missing on
	// builds with static ggml backends; the loader treats a missing symbol
	// as a soft no-op.
	ggmlBackendLoadAllFromPathFunc ffi.Fun
)

func loadSystemFuncs(lib ffi.Lib) error {
	var err error

	if versionFunc, err = lib.Prep("sd_version", &ffi.TypePointer); err != nil {
		return loadError("sd_version", err)
	}

	if getSystemInfoFunc, err = lib.Prep("sd_get_system_info", &ffi.TypePointer); err != nil {
		return loadError("sd_get_system_info", err)
	}

	if getNumPhysicalCoresFunc, err = lib.Prep("sd_get_num_physical_cores", &ffi.TypeSint32); err != nil {
		return loadError("sd_get_num_physical_cores", err)
	}

	// Optional. leejet's official Windows zip ships a single stable-diffusion.dll
	// with ggml statically linked but NOT re-exported (GGML_API expands to nothing
	// on a non-GGML_SHARED PE/COFF build), so GetProcAddress returns
	// ERROR_PROC_NOT_FOUND for every ggml_* symbol. On Mach-O / ELF the same
	// static link still exports the symbols by default visibility. Treat both
	// ggml_* preps as best-effort and let GGMLBackendDeviceCount report -1
	// when the symbol is unavailable.
	if fn, perr := lib.Prep("ggml_backend_dev_count", &ffi.TypeUint64); perr == nil {
		ggmlBackendDevCountFunc = fn
	}

	// Optional: only present when libstable-diffusion was built with
	// -DGGML_BACKEND_DL=ON. Best-effort — a Prep failure here just means
	// the symbol isn't exported, which is fine for static builds.
	if fn, perr := lib.Prep("ggml_backend_load_all_from_path", &ffi.TypeVoid, &ffi.TypePointer); perr == nil {
		ggmlBackendLoadAllFromPathFunc = fn
	}

	return nil
}

// Version returns the stable-diffusion.cpp library version string.
func Version() string {
	var ptr *byte
	versionFunc.Call(unsafe.Pointer(&ptr))
	if ptr == nil {
		return ""
	}
	return utils.BytePtrToString(ptr)
}

// SystemInfo returns the system info string reported by stable-diffusion.cpp
// (the same string the upstream CLI prints at startup, e.g. CPU features and
// backend availability).
func SystemInfo() string {
	var ptr *byte
	getSystemInfoFunc.Call(unsafe.Pointer(&ptr))
	if ptr == nil {
		return ""
	}
	return utils.BytePtrToString(ptr)
}

// NumPhysicalCores returns the number of physical CPU cores detected by
// stable-diffusion.cpp on the current host.
func NumPhysicalCores() int32 {
	var result ffi.Arg
	getNumPhysicalCoresFunc.Call(unsafe.Pointer(&result))
	return int32(result)
}

// GGMLBackendDeviceCount returns the number of ggml backend devices currently
// registered in the process-wide registry. Callers use this as a sentinel to
// decide whether to call Init when running alongside other ggml-based
// libraries in the same process.
//
// Returns -1 when the underlying ggml_backend_dev_count symbol is not exported
// by the loaded libstable-diffusion (e.g. leejet's Windows DLL statically
// links ggml without re-exporting its API). In that case, callers cannot use
// this to detect a populated registry; assume Init is safe to call.
func GGMLBackendDeviceCount() int {
	if ggmlBackendDevCountFunc == (ffi.Fun{}) {
		return -1
	}
	var result ffi.Arg
	ggmlBackendDevCountFunc.Call(unsafe.Pointer(&result))
	return int(result)
}

// ggmlBackendLoadAllFromPath dlopens every libggml-*.{so,dylib,dll} found in
// dirPath so each backend's static constructor self-registers with ggml.
//
// Required for builds where ggml backends ship as separate dynamic libraries
// (-DGGML_BACKEND_DL=ON). Without this call, ggml ends up with zero
// registered backends and the first context creation asserts on
// device==NULL.
//
// On builds where backends are statically linked into libstable-diffusion
// (the upstream leejet releases), the symbol is not present at all and this
// is a no-op (loadSystemFuncs leaves the Fun zero-valued). On builds where
// the symbol is present but no libggml-*.{so,dylib,dll} files match in
// dirPath, the underlying ggml implementation is itself a safe no-op.
func ggmlBackendLoadAllFromPath(dirPath string) error {
	if ggmlBackendLoadAllFromPathFunc == (ffi.Fun{}) {
		return nil
	}
	cpath, err := utils.BytePtrFromString(dirPath)
	if err != nil {
		return err
	}
	ggmlBackendLoadAllFromPathFunc.Call(nil, unsafe.Pointer(&cpath))
	return nil
}
