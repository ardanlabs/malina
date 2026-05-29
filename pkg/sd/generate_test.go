package sd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ardanlabs/malina/pkg/download"
)

// TestGenerateImageSD15Smoke is the end-to-end happy-path test for the
// FFI bridge against the sd-1.5 bundle: load a real stable-diffusion
// model, run one denoising step at a tiny resolution, and assert the
// returned image has the requested shape and non-empty pixel data. The
// round-trip through SavePNG / LoadPNG also exercises the image I/O
// code paths against a real generated buffer rather than a synthesized
// gradient.
//
// Steps=1 and 64x64 keep the test fast on CPU; the bytes won't form a
// coherent picture but that is not what we're verifying. We're verifying
// that the C library accepts our struct layout, returns a buffer with
// matching dimensions, and that buffer survives the SDImage->PNG->SDImage
// round-trip.
//
// Requires MALINA_LIB and MALINA_TEST_MODEL. Skipped otherwise.
func TestGenerateImageSD15Smoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_TEST_MODEL")

	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath

	params := ImgGenParamsInit()
	params.Steps = 1

	assertGenerateSmoke(t, cparams, params)
}

// TestGenerateImageSDXLSmoke mirrors the sd-1.5 test against the sdxl-
// base-1.0 bundle. SDXL's native resolution is 1024x1024 but the FFI
// happy-path is independent of denoising quality, so we still run at
// 64x64 / 1 step to keep the test fast. The point is to verify the C
// library accepts the SDXL checkpoint format through our ContextParams
// and returns a buffer of the requested shape.
//
// Requires MALINA_LIB and MALINA_SDXL_TEST_MODEL. Skipped otherwise.
func TestGenerateImageSDXLSmoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_SDXL_TEST_MODEL")

	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath

	params := ImgGenParamsInit()
	params.Steps = 1

	assertGenerateSmoke(t, cparams, params)
}

// TestGenerateImageFlux2Smoke exercises the multi-file FLUX.2 [klein] 9B
// bundle: read manifest.json, wire DiffusionModelPath + VAEPath + LLMPath
// from the manifest, and generate one image. FLUX.2 [klein] is 4-step
// distilled so 4 is the lowest meaningful Steps value; we still bring
// width and height down to 256x256 to keep the test bounded on CPU. On
// Apple Silicon with Metal the whole thing completes in well under a
// minute.
//
// Requires MALINA_LIB and MALINA_FLUX2_TEST_DIR (a directory containing
// the bundle's three files plus the manifest.json that `malina model
// pull flux2-klein-9b` writes). Skipped otherwise.
func TestGenerateImageFlux2Smoke(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_FLUX2_TEST_DIR")

	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	cparams := ContextParamsInit()
	cparams.DiffusionModelPath = manifest.Files[string(download.RoleDiffusion)]
	cparams.VAEPath = manifest.Files[string(download.RoleVAE)]
	cparams.LLMPath = manifest.Files[string(download.RoleLLM)]

	params := ImgGenParamsInit()
	params.Width = 256
	params.Height = 256
	params.Steps = 4

	assertGenerateSmoke(t, cparams, params)
}

// TestGenerateImageImg2ImgSmoke exercises the InitImage path end-to-end:
// hand the C library a synthesized 128x128 RGB image as InitImage with a
// text prompt, and assert the returned buffer has the requested shape.
// The point is to verify the cImage binding survives the FFI call and
// stable-diffusion.cpp's IMG2IMG branch runs to completion; the produced
// pixels are not validated for content quality.
//
// VAEDecodeOnly is flipped off on the context — img2img needs the VAE
// encoder, which the C library skips by default to save memory.
//
// Requires MALINA_LIB and MALINA_TEST_MODEL. Skipped otherwise.
func TestGenerateImageImg2ImgSmoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_TEST_MODEL")

	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath
	cparams.VAEDecodeOnly = false

	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	const dim = 128
	initImg := &SDImage{
		Width:   dim,
		Height:  dim,
		Channel: 3,
		Data:    make([]byte, dim*dim*3),
	}
	for i := range initImg.Data {
		initImg.Data[i] = 128 // neutral grey
	}

	params := ImgGenParamsInit()
	params.Prompt = "a dog"
	params.Width = dim
	params.Height = dim
	params.Steps = 4
	params.Seed = 43
	params.Strength = 0.75
	params.InitImage = initImg

	img, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatalf("GenerateImage (img2img): %v", err)
	}
	if img == nil {
		t.Fatal("GenerateImage (img2img) returned nil image with nil error")
	}
	if img.Width != uint32(params.Width) || img.Height != uint32(params.Height) {
		t.Errorf("dims: got %dx%d, want %dx%d", img.Width, img.Height, params.Width, params.Height)
	}
	if img.Channel != 3 {
		t.Errorf("Channel: got %d, want 3", img.Channel)
	}
	want := int(img.Width) * int(img.Height) * int(img.Channel)
	if len(img.Data) != want {
		t.Fatalf("Data length: got %d, want %d", len(img.Data), want)
	}
}

// assertGenerateSmoke is the shared end-to-end body the per-bundle smoke
// tests delegate into. Caller supplies fully-prepared ContextParams (so
// each test wires whichever model paths its bundle needs) and the seed
// ImgGenParams; assertGenerateSmoke fills in the smoke defaults (small
// prompt, 64x64x1 if the caller didn't override, fixed seed), allocates
// a fresh Context, runs one generation, and asserts the returned image
// has the requested dimensions and survives a SavePNG / LoadPNG
// round-trip. Centralizing the body keeps the per-bundle tests focused
// on the bundle-specific configuration.
func assertGenerateSmoke(t *testing.T, cparams ContextParams, params ImgGenParams) {
	t.Helper()

	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	if params.Prompt == "" {
		params.Prompt = "a cat"
	}
	if params.Width == 0 {
		params.Width = 64
	}
	if params.Height == 0 {
		params.Height = 64
	}
	if params.Steps == 0 {
		params.Steps = 1
	}
	if params.Seed == 0 {
		params.Seed = 42
	}

	img, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatalf("GenerateImage: %v", err)
	}
	if img == nil {
		t.Fatal("GenerateImage returned nil image with nil error")
	}

	if img.Width != uint32(params.Width) {
		t.Errorf("Width: got %d, want %d", img.Width, params.Width)
	}
	if img.Height != uint32(params.Height) {
		t.Errorf("Height: got %d, want %d", img.Height, params.Height)
	}
	if img.Channel != 3 {
		t.Errorf("Channel: got %d, want 3", img.Channel)
	}
	wantSize := int(img.Width) * int(img.Height) * int(img.Channel)
	if len(img.Data) != wantSize {
		t.Fatalf("Data length: got %d, want %d", len(img.Data), wantSize)
	}

	// SavePNG / LoadPNG round-trip on a real generated buffer.
	outPath := filepath.Join(t.TempDir(), "smoke.png")
	if err := img.SavePNG(outPath); err != nil {
		t.Fatalf("SavePNG: %v", err)
	}

	got, err := LoadPNG(outPath)
	if err != nil {
		t.Fatalf("LoadPNG: %v", err)
	}
	if got.Width != img.Width || got.Height != img.Height || got.Channel != img.Channel {
		t.Fatalf("round-trip dims: got %dx%dx%d, want %dx%dx%d",
			got.Width, got.Height, got.Channel,
			img.Width, img.Height, img.Channel)
	}
	if !bytes.Equal(got.Data, img.Data) {
		t.Error("round-trip pixel data: mismatch after PNG encode/decode")
	}
}
