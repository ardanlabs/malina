package sd

import (
	"context"
	"os"
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
// benchmark uses the same 512x512 / 20-step defaults as the sd-1.5
// benchmark so a single iteration stays bounded on Apple Silicon Metal
// (a 1024x1024x20 SDXL inference takes ~30 s per iter on an M-series
// GPU).
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

// BenchmarkGenerateImageImg2ImgSD15 measures end-to-end image-to-image
// throughput against the sd-1.5 bundle. It mirrors the txt2img benchmark
// and feeds a synthesized 512x512 neutral-grey buffer as InitImage. Strength
// is left at the C library default (0.75) so the full denoising schedule runs
// and the per-iteration cost includes the VAE encode pass unique to img2img.
//
// Requires: MALINA_LIB and one of (MALINA_BENCH_MODEL or MALINA_TEST_MODEL).
func BenchmarkGenerateImageImg2ImgSD15(b *testing.B) {
	benchSetup(b)
	modelPath := benchEnvModelFile(b, "MALINA_BENCH_MODEL", "MALINA_TEST_MODEL")

	cparams := benchContextParams()
	cparams.ModelPath = modelPath

	params := ImgGenParamsInit()
	params.InitImage = benchSyntheticImage(int(params.Width), int(params.Height))

	runGenerateBench(b, cparams, params)
}

// benchSyntheticImage returns a 3-channel RGB SDImage with every pixel
// set to neutral grey. Used as the InitImage for the img2img benchmark
// so the bench has no file I/O dependency.
func benchSyntheticImage(width, height int) *SDImage {
	img := SDImage{
		Width:   uint32(width),
		Height:  uint32(height),
		Channel: 3,
		Data:    make([]byte, width*height*3),
	}
	for i := range img.Data {
		img.Data[i] = 128
	}
	return &img
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

// benchContextParams returns ContextParams suitable for benchmarks.
func benchContextParams() ContextParams {
	return ContextParamsInit()
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
	if err := download.VerifyDefaultInstall(context.Background(), libPath); err != nil {
		b.Fatalf("MALINA_LIB must contain download.DefaultSDVersion: %v", err)
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
