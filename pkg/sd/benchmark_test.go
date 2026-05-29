package sd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ardanlabs/malina/pkg/download"
)

// BenchmarkGenerateImageSD15 measures end-to-end text-to-image throughput
// against the sd-1.5 bundle. The model is loaded once outside the timed
// loop and one warm-up iteration is dropped so Metal JIT / library init
// does not pollute the steady-state measurement.
//
// Requires: MALINA_LIB and one of (MALINA_BENCH_MODEL or MALINA_TEST_MODEL).
// MALINA_BENCH_MODEL takes precedence so callers can benchmark a different
// checkpoint without disturbing the regular test model.
//
// Reports the canonical bench timer (ns/op) plus custom metrics:
//   - "s/img"   — wall seconds per generated image
//   - "px"      — width*height of the generated image
//
// The default shape is 512x512 with 20 steps (the SD 1.5 native resolution
// and the C library default step count). Override BENCHTIME to control
// iteration count (e.g. `-benchtime=3x`); a single iteration on Apple
// Silicon Metal is on the order of seconds.
func BenchmarkGenerateImageSD15(b *testing.B) {
	benchSetup(b)
	modelPath := benchEnvModelFile(b, "MALINA_BENCH_MODEL", "MALINA_TEST_MODEL")

	cparams := benchContextParams()
	cparams.ModelPath = modelPath

	runGenerateBench(b, cparams, ImgGenParamsInit())
}

// BenchmarkGenerateImageSDXL mirrors the sd-1.5 benchmark against the
// sdxl-base-1.0 bundle. SDXL's native resolution is 1024x1024 but the
// benchmark defaults to 512x512 / 20 steps so a single iteration stays
// bounded on Apple Silicon Metal (a 1024x1024x20 SDXL inference takes
// ~30 s per iter on an M-series GPU). Override Width/Height by passing
// MALINA_BENCH_SDXL_WIDTH / MALINA_BENCH_SDXL_HEIGHT if you want the
// native resolution.
//
// Requires: MALINA_LIB and one of (MALINA_BENCH_SDXL_MODEL or
// MALINA_SDXL_TEST_MODEL).
func BenchmarkGenerateImageSDXL(b *testing.B) {
	benchSetup(b)
	modelPath := benchEnvModelFile(b, "MALINA_BENCH_SDXL_MODEL", "MALINA_SDXL_TEST_MODEL")

	cparams := benchContextParams()
	cparams.ModelPath = modelPath

	runGenerateBench(b, cparams, ImgGenParamsInit())
}

// BenchmarkGenerateImageFlux2 exercises the multi-file FLUX.2 [klein] 9B
// bundle: read manifest.json, wire DiffusionModelPath + VAEPath + LLMPath
// from the manifest, and generate one image per iteration. FLUX.2 [klein]
// is 4-step distilled so 4 steps is the lowest meaningful Steps value;
// the default 512x512 resolution keeps a single iteration in the ~30 s
// range on Apple Silicon Metal.
//
// Requires: MALINA_LIB and one of (MALINA_BENCH_FLUX2_DIR or
// MALINA_FLUX2_TEST_DIR), a directory containing the bundle's three
// files plus the manifest.json that `malina model pull flux2-klein-9b`
// writes.
func BenchmarkGenerateImageFlux2(b *testing.B) {
	benchSetup(b)
	bundleDir := benchEnvBundleDir(b, "MALINA_BENCH_FLUX2_DIR", "MALINA_FLUX2_TEST_DIR")

	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		b.Fatalf("LoadManifest: %v", err)
	}

	cparams := benchContextParams()
	cparams.DiffusionModelPath = manifest.Files[string(download.RoleDiffusion)]
	cparams.VAEPath = manifest.Files[string(download.RoleVAE)]
	cparams.LLMPath = manifest.Files[string(download.RoleLLM)]

	params := ImgGenParamsInit()
	params.Width = 512
	params.Height = 512
	params.Steps = 4

	runGenerateBench(b, cparams, params)
}

// runGenerateBench is the shared body the per-bundle benchmarks delegate
// into. Caller supplies fully-prepared ContextParams (so each benchmark
// wires whichever model paths its bundle needs) and the seed
// ImgGenParams; runGenerateBench fills in a fixed prompt + seed, loads
// the context once, drops one untimed warm-up iteration (to absorb
// Metal/CUDA JIT and any first-call library setup), and then measures
// b.N GenerateImage calls reusing the same context.
func runGenerateBench(b *testing.B, cparams ContextParams, params ImgGenParams) {
	b.Helper()

	ctx, err := NewContext(cparams)
	if err != nil {
		b.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	if params.Prompt == "" {
		params.Prompt = "a lovely cat"
	}
	if params.Seed == 0 {
		params.Seed = 42
	}

	// Warm up: the first GenerateImage on Metal/CUDA includes JIT and
	// library setup we don't want folded into the timed loop.
	if _, err := GenerateImage(ctx, params); err != nil {
		b.Fatalf("GenerateImage (warmup): %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := GenerateImage(ctx, params); err != nil {
			b.Fatalf("GenerateImage: %v", err)
		}
	}
	b.StopTimer()

	wallSeconds := b.Elapsed().Seconds() / float64(b.N)
	b.ReportMetric(wallSeconds, "s/img")
	b.ReportMetric(float64(params.Width)*float64(params.Height), "px")
}

// benchContextParams returns ContextParams suitable for repeated
// GenerateImage calls against the same Context.
//
// sd_ctx_params_init defaults FreeParamsImmediately to true, which
// releases the model parameter memory after the first generation. A
// second GenerateImage call on the same Context then aborts inside
// libstable-diffusion with `GGML_ASSERT(buft) failed` because the
// freed param tensors no longer have a backing buffer-type. The
// benchmarks reuse one Context across b.N iterations so the model
// load time is not folded into the timed loop, which requires
// FreeParamsImmediately=false.
func benchContextParams() ContextParams {
	p := ContextParamsInit()
	p.FreeParamsImmediately = false
	return p
}

// benchSetup ensures the stable-diffusion shared library is loaded and
// initialized exactly once across the bench process. Mirrors testSetup
// but takes a *testing.B so benchmarks can skip (rather than fail) when
// MALINA_LIB is unset.
func benchSetup(b *testing.B) {
	b.Helper()

	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		b.Skip("MALINA_LIB not set; skipping stable-diffusion FFI benchmark")
	}

	loadOnce.Do(func() {
		if loadErr = Load(libPath); loadErr != nil {
			return
		}
		loadErr = Init(libPath)
	})
	if loadErr != nil {
		b.Fatalf("failed to load stable-diffusion.cpp from %s: %v", libPath, loadErr)
	}
}

// benchEnvModelFile returns the model path stored in the first non-empty
// of envs. The benchmark is skipped (not failed) when no env is set or
// when the resolved file is missing, mirroring testEnvModelFile's
// stale-env-tolerant behavior.
func benchEnvModelFile(b *testing.B, envs ...string) string {
	b.Helper()

	var (
		env, model string
	)
	for _, e := range envs {
		if v := os.Getenv(e); v != "" {
			env = e
			model = v
			break
		}
	}
	if model == "" {
		b.Skipf("%v not set; skipping benchmark that requires a model", envs)
	}
	if _, err := os.Stat(model); err != nil {
		b.Skipf("%s=%q not present: %v", env, model, err)
	}
	return model
}

// benchEnvBundleDir returns the bundle directory stored in the first
// non-empty of envs. Skipped (not failed) when no env is set, when the
// directory is missing, or when manifest.json is absent. Mirrors
// testEnvBundleDir for the multi-file FLUX.2 bundle.
func benchEnvBundleDir(b *testing.B, envs ...string) string {
	b.Helper()

	var (
		env, dir string
	)
	for _, e := range envs {
		if v := os.Getenv(e); v != "" {
			env = e
			dir = v
			break
		}
	}
	if dir == "" {
		b.Skipf("%v not set; skipping benchmark that requires a model bundle", envs)
	}
	info, err := os.Stat(dir)
	if err != nil {
		b.Skipf("%s=%q not present: %v", env, dir, err)
	}
	if !info.IsDir() {
		b.Skipf("%s=%q is not a directory", env, dir)
	}
	manifest := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		b.Skipf("bundle manifest %q not present: %v (did you run `malina model pull`?)", manifest, err)
	}
	return dir
}
