// Package sd provides Go FFI bindings to stable-diffusion.cpp using purego and
// jupiterrider/ffi. It mirrors the public C API exposed by stable-diffusion.h as
// a thin layer; higher-level ergonomics live in cmd/ or downstream consumers.
package sd

import (
	"fmt"

	"github.com/ardanlabs/malina/pkg/loader"
)

// Opaque handles. These are pointers in C; in Go we carry them as uintptr
// so they round-trip through the FFI boundary without retainability issues.
//
// # Concurrency
//
// A Context is NOT safe for concurrent use. Every call into GenerateImage
// (and GenerateVideo) mutates state inside the underlying sd_ctx_t —
// the RNG seed and counter, denoiser shift, LoRA model maps, VAE tiling
// params, circular-padding flags, and (when free_params_immediately is
// set) the parameter buffers of the shared sub-models. Upstream
// stable-diffusion.cpp performs no locking on any of this; two
// goroutines calling GenerateImage on the same Context is a data race.
// The upstream HTTP server example (examples/server/runtime.h) serializes
// access through an explicit std::mutex per context for this reason.
//
// To run generation in parallel, allocate one Context per goroutine via
// NewContext. The model weights are reloaded per Context, so the memory
// cost scales linearly — plan for it. Independent Contexts on different
// goroutines do not race on per-context state, but the log/progress/
// preview callbacks installed via this package are process-globals in
// upstream and should be configured once at startup.
type (
	Context uintptr
)

// SDType mirrors enum sd_type_t. Values match enum ggml_type.
type SDType int32

const (
	SDTypeF32    SDType = 0
	SDTypeF16    SDType = 1
	SDTypeQ4_0   SDType = 2
	SDTypeQ4_1   SDType = 3
	SDTypeQ5_0   SDType = 6
	SDTypeQ5_1   SDType = 7
	SDTypeQ8_0   SDType = 8
	SDTypeQ8_1   SDType = 9
	SDTypeQ2K    SDType = 10
	SDTypeQ3K    SDType = 11
	SDTypeQ4K    SDType = 12
	SDTypeQ5K    SDType = 13
	SDTypeQ6K    SDType = 14
	SDTypeQ8K    SDType = 15
	SDTypeIQ2XXS SDType = 16
	SDTypeIQ2XS  SDType = 17
	SDTypeIQ3XXS SDType = 18
	SDTypeIQ1S   SDType = 19
	SDTypeIQ4NL  SDType = 20
	SDTypeIQ3S   SDType = 21
	SDTypeIQ2S   SDType = 22
	SDTypeIQ4XS  SDType = 23
	SDTypeI8     SDType = 24
	SDTypeI16    SDType = 25
	SDTypeI32    SDType = 26
	SDTypeI64    SDType = 27
	SDTypeF64    SDType = 28
	SDTypeIQ1M   SDType = 29
	SDTypeBF16   SDType = 30
	SDTypeTQ1_0  SDType = 34
	SDTypeTQ2_0  SDType = 35
	SDTypeMXFP4  SDType = 39
	SDTypeNVFP4  SDType = 40
	SDTypeQ1_0   SDType = 41
	SDTypeCount  SDType = 42
)

// RngType mirrors enum rng_type_t.
type RngType int32

const (
	RngStdDefault RngType = 0
	RngCuda       RngType = 1
	RngCPU        RngType = 2
	RngTypeCount  RngType = 3
)

// SampleMethod mirrors enum sample_method_t.
type SampleMethod int32

const (
	SampleEuler        SampleMethod = 0
	SampleEulerA       SampleMethod = 1
	SampleHeun         SampleMethod = 2
	SampleDPM2         SampleMethod = 3
	SampleDPMPP2SA     SampleMethod = 4
	SampleDPMPP2M      SampleMethod = 5
	SampleDPMPP2Mv2    SampleMethod = 6
	SampleIPNDM        SampleMethod = 7
	SampleIPNDMV       SampleMethod = 8
	SampleLCM          SampleMethod = 9
	SampleDDIMTrailing SampleMethod = 10
	SampleTCD          SampleMethod = 11
	SampleResMultistep SampleMethod = 12
	SampleRes2S        SampleMethod = 13
	SampleERSDE        SampleMethod = 14
	SampleEulerCFGPP   SampleMethod = 15
	SampleEulerACFGPP  SampleMethod = 16
	SampleEulerGE      SampleMethod = 17
	SampleDPMPP2MSDE   SampleMethod = 18
	SampleDPMPP2MSDEBT SampleMethod = 19
	SampleLMS          SampleMethod = 20
	SampleMethodCount  SampleMethod = 21
)

// Scheduler mirrors enum scheduler_t.
type Scheduler int32

const (
	SchedulerDiscrete    Scheduler = 0
	SchedulerKarras      Scheduler = 1
	SchedulerExponential Scheduler = 2
	SchedulerAys         Scheduler = 3
	SchedulerGits        Scheduler = 4
	SchedulerSgmUniform  Scheduler = 5
	SchedulerSimple      Scheduler = 6
	SchedulerSmoothstep  Scheduler = 7
	SchedulerKLOptimal   Scheduler = 8
	SchedulerLCM         Scheduler = 9
	SchedulerBongTangent Scheduler = 10
	SchedulerLTX2        Scheduler = 11
	SchedulerLogitNormal Scheduler = 12
	SchedulerFlux2       Scheduler = 13
	SchedulerFlux        Scheduler = 14
	SchedulerBeta        Scheduler = 15
	SchedulerCount       Scheduler = 16
)

// Prediction mirrors enum prediction_t.
type Prediction int32

const (
	PredictionEPS         Prediction = 0
	PredictionV           Prediction = 1
	PredictionEDMV        Prediction = 2
	PredictionFlow        Prediction = 3
	PredictionFluxFlow    Prediction = 4
	PredictionSeFiFlow    Prediction = 5
	PredictionMinit2IFlow Prediction = 6
	PredictionCount       Prediction = 7
)

// LogLevel mirrors enum sd_log_level_t.
type LogLevel int32

const (
	LogDebug LogLevel = 0
	LogInfo  LogLevel = 1
	LogWarn  LogLevel = 2
	LogError LogLevel = 3
)

// SDVaeFormat mirrors enum sd_vae_format_t. The C library uses this to
// select the VAE numerical format for image decoding. AUTO is the
// default written by sd_ctx_params_init and is the right choice for
// most models — the C library picks the matching format based on the
// loaded checkpoint.
type SDVaeFormat int32

const (
	SDVaeFormatAuto  SDVaeFormat = -1
	SDVaeFormatFlux  SDVaeFormat = 0
	SDVaeFormatSD3   SDVaeFormat = 1
	SDVaeFormatFlux2 SDVaeFormat = 2
	SDVaeFormatWan   SDVaeFormat = 3
	SDVaeFormatCount SDVaeFormat = 4
)

// LoraApplyMode mirrors enum lora_apply_mode_t.
type LoraApplyMode int32

const (
	LoraApplyAuto        LoraApplyMode = 0
	LoraApplyImmediately LoraApplyMode = 1
	LoraApplyAtRuntime   LoraApplyMode = 2
	LoraApplyModeCount   LoraApplyMode = 3
)

var libPath string

// LibPath returns the path to the loaded stable-diffusion.cpp shared library.
func LibPath() string {
	return libPath
}

// Load loads the shared stable-diffusion.cpp library from the specified path
// and resolves all FFI function pointers used by this package.
//
// Load does NOT register ggml backends. Call Init after Load (and before the
// first context creation) to populate the process-wide ggml backend registry
// from the same directory.
//
// In the official leejet releases the ggml backends are statically linked into
// libstable-diffusion, so backend registration happens automatically as part
// of library load. Init is still safe to call (and recommended for consistency
// with bucky) but is effectively a no-op in that case.
func Load(path string) error {
	libPath = path

	lib, err := loader.LoadLibrary(path, "stable-diffusion")
	if err != nil {
		return err
	}

	if err := loadSystemFuncs(lib); err != nil {
		return err
	}

	if err := loadLogFuncs(lib); err != nil {
		return err
	}

	if err := loadContextFuncs(lib); err != nil {
		return err
	}

	if err := loadGenFuncs(lib); err != nil {
		return err
	}

	if err := installLogCallback(); err != nil {
		return err
	}

	return nil
}

// Init registers every ggml backend shared library found under path with the
// process-wide ggml registry. Required when libstable-diffusion was built with
// -DGGML_BACKEND_DL=ON, where backends ship as separate libggml-*.so files
// that don't auto-register on libstable-diffusion load. No-op on static builds
// (the upstream leejet releases) because the underlying ggml symbol is either
// absent or finds nothing to load.
//
// Call Init AFTER Load and BEFORE the first context creation.
//
// Callers running stable-diffusion alongside another ggml-based library (e.g.
// llama.cpp via yzma, or whisper.cpp via bucky) that already populated the
// registry SHOULD skip this call to avoid registering the same physical device
// twice. Use sd.GGMLBackendDeviceCount() > 0 as the sentinel in that case.
func Init(path string) error {
	if err := ggmlBackendLoadAllFromPath(path); err != nil {
		return fmt.Errorf("ggml_backend_load_all_from_path: %w", err)
	}

	return nil
}

// loadError wraps an FFI symbol resolution failure with a stable prefix so
// callers can identify which symbol failed.
func loadError(name string, err error) error {
	return fmt.Errorf("could not load %q: %w", name, err)
}
