//go:build malina_model_tests

package sd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ardanlabs/malina/pkg/download"
)

func TestControlNetGeneration(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_CONTROLNET_TEST_DIR")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	cparams := ContextParamsInit()
	cparams.ModelPath = manifest.Files[string(download.RoleModel)]
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	cparams.ControlNetPath = manifest.Files[string(download.RoleControlNet)]
	if err := SetContextModelOptions(ctx, cparams); err != nil {
		t.Fatalf("SetContextModelOptions: %v", err)
	}

	control := SDImage{Width: 64, Height: 64, Channel: 3, Data: make([]byte, 64*64*3)}
	for y := range 64 {
		for x := range 64 {
			if x == 16 || x == 47 || y == 16 || y == 47 {
				i := (y*64 + x) * 3
				control.Data[i] = 255
				control.Data[i+1] = 255
				control.Data[i+2] = 255
			}
		}
	}
	control, err = PreprocessCanny(control, CannyParams{HighThreshold: 0.08, LowThreshold: 0.08, Weak: 0.8, Strong: 1})
	if err != nil {
		t.Fatalf("PreprocessCanny: %v", err)
	}
	params := ImgGenParamsInit()
	params.Prompt = "a framed landscape"
	params.Width = 64
	params.Height = 64
	params.Steps = 1
	params.Seed = 42
	params.ControlImage = control
	params.ControlStrength = 1
	image, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatalf("GenerateImage with ControlNet: %v", err)
	}
	if image.Width != 64 || image.Height != 64 || len(image.Data) != 64*64*3 {
		t.Fatalf("GenerateImage with ControlNet returned invalid image: %#v", image)
	}

	cparams.ControlNetPath = ""
	if err := SetContextModelOptions(ctx, cparams); err != nil {
		t.Fatalf("remove ControlNet: %v", err)
	}
	params.ControlImage = SDImage{}
	params.ControlStrength = .9
	uncontrolled, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(image.Data, uncontrolled.Data) {
		t.Fatal("replacing ControlNet configuration did not change output")
	}
}

// TestControlNetUnionTask verifies that task selection reaches the native
// Union network, rather than being ignored or only stored in Go state.
func TestControlNetUnionTask(t *testing.T) {
	testSetup(t)
	controlPath := testEnvModelFile(t, "MALINA_CONTROLNET_UNION_TEST_MODEL")
	modelPath := testEnvModelFile(t, "MALINA_SDXL_TEST_MODEL")
	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath
	cparams.ControlNetPath = controlPath
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	control := SDImage{Width: 64, Height: 64, Channel: 3, Data: make([]byte, 64*64*3)}
	for y := range 64 {
		for x := range 64 {
			for channel := range 3 {
				control.Data[(y*64+x)*3+channel] = byte(x*3 + y)
			}
		}
	}
	params := ImgGenParamsInit()
	params.Prompt = "a blue house on a hillside"
	params.Width, params.Height = 64, 64
	params.Steps, params.Seed = 2, 42
	params.ControlImage = control
	params.ControlStrength = 1
	if _, err := GenerateImage(ctx, params); err == nil {
		t.Fatal("Union generation accepted the default ControlTypeNone")
	}
	params.ControlType = ControlTypeDepth
	depth, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatalf("GenerateImage depth: %v", err)
	}
	params.ControlType = ControlTypeCanny
	canny, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatalf("GenerateImage canny: %v", err)
	}
	if bytes.Equal(depth.Data, canny.Data) {
		t.Fatal("depth and canny tasks produced identical pixels")
	}
	repeated, err := GenerateImage(ctx, params)
	if err != nil {
		t.Fatalf("GenerateImage repeated canny: %v", err)
	}
	if !bytes.Equal(canny.Data, repeated.Data) {
		t.Fatal("same seed and canny task produced different pixels; task comparison is inconclusive")
	}
}

func TestUpscaleImage(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_UPSCALER_TEST_DIR")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	modelPath := manifest.Files[string(download.RoleUpscaler)]
	modelScale, err := GetUpscalerModelScale(modelPath)
	if err != nil {
		t.Fatalf("GetUpscalerModelScale: %v", err)
	}
	if modelScale != 4 {
		t.Fatalf("GetUpscalerModelScale: got %d, want 4", modelScale)
	}

	ctx, err := NewUpscalerContext(modelPath, false, NumPhysicalCores(), 0, "", "")
	if err != nil {
		t.Fatalf("NewUpscalerContext: %v", err)
	}
	defer FreeUpscalerContext(ctx)

	factor, err := GetUpscaleFactor(ctx)
	if err != nil {
		t.Fatalf("GetUpscaleFactor: %v", err)
	}
	if factor <= 1 {
		t.Fatalf("GetUpscaleFactor: got %d, want greater than 1", factor)
	}
	if factor != modelScale {
		t.Fatalf("model scale %d differs from loaded context factor %d", modelScale, factor)
	}
	input := SDImage{Width: 8, Height: 8, Channel: 3, Data: make([]byte, 8*8*3)}
	for i := range input.Data {
		input.Data[i] = byte(i)
	}
	images, err := Upscale(ctx, input, uint32(factor))
	if err != nil {
		t.Fatalf("Upscale: %v", err)
	}
	if len(images) == 0 {
		t.Fatal("Upscale returned no image")
	}
	if images[0].Width != input.Width*uint32(factor) || images[0].Height != input.Height*uint32(factor) {
		t.Errorf("upscaled dimensions: got %dx%d, want %dx%d", images[0].Width, images[0].Height, input.Width*uint32(factor), input.Height*uint32(factor))
	}
	replayCtx, err := NewContext(ContextParamsInit())
	if err != nil {
		t.Fatal(err)
	}
	defer FreeContext(replayCtx)
	outputs, err := RunDGL(replayCtx, images[0].GenerationInfo, map[string]SDImage{"input_image": input})
	if err != nil {
		t.Fatalf("replay upscale program: %v\n%s", err, images[0].GenerationInfo)
	}
	replayed, err := outputs["image"].Images()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(replayed[0].Data, images[0].Data) {
		t.Fatal("upscale replay pixels differ")
	}
}

func TestADetailImage(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_ADETAILER_TEST_DIR")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	cparams := ContextParamsInit()
	cparams.ModelPath = manifest.Files[string(download.RoleModel)]
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	input, err := LoadPNG(filepath.Join("..", "..", "samples", "adetailer-face.png"))
	if err != nil {
		t.Fatalf("LoadPNG: %v", err)
	}
	before := append([]byte(nil), input.Data...)
	params := FaceRefinementParamsInit()
	params.ModelPath = cparams.ModelPath
	params.DetectorPath = manifest.Files[string(download.RoleADetailer)]
	params.Prompt = "a detailed portrait photo"
	params.NegativePrompt = "blurry \"portrait\"\nlow quality"
	params.Steps, params.Seed = 4, 73
	params.InpaintHeight = 128
	params.CFGScale, params.Strength = 5.5, .5
	image, err := RefineFaces(ctx, input, params)
	if err != nil {
		t.Fatalf("RefineFaces: %v", err)
	}
	if image.Width != input.Width || image.Height != input.Height || len(image.Data) != len(input.Data) {
		t.Fatalf("ADetailImage returned invalid final image: %#v", image)
	}
	if bytes.Equal(image.Data, before) {
		t.Fatal("ADetailer DGL did not refine the detected face")
	}
	if !bytes.Equal(input.Data, before) {
		t.Fatal("RefineFaces mutated input")
	}
	params.Seed++
	changed, err := RefineFaces(ctx, input, params)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(changed.Data, image.Data) {
		t.Fatal("RefineFaces ignored seed override")
	}
	outputs, err := RunDGL(ctx, image.GenerationInfo, map[string]SDImage{"image": input})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := outputs["image"].Images()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(image.Data, replayed[0].Data) {
		t.Fatal("RefineFaces source did not reproduce output")
	}
}

func TestEditMOSSImage(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_MOSS_EDIT_TEST_DIR")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := NewContext(ContextParamsInit())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { FreeContext(ctx) }()
	params := MOSSImageEditParamsInit()
	params.DiffusionModelPath = manifest.Files[string(download.RoleDiffusion)]
	params.LLMPath = manifest.Files[string(download.RoleLLM)]
	params.LLMVisionPath = manifest.Files[string(download.RoleLLMVision)]
	params.TokenizerPath = manifest.Files[string(download.RoleTokenizer)]
	params.VAEPath = manifest.Files[string(download.RoleVAE)]
	params.LoraPath = manifest.Files[string(download.RoleLora)]
	params.Prompt = "Make the image blue."
	params.NegativePrompt = "blurry \"image\"\nlow quality"
	params.Width, params.Height = 96, 64
	params.Steps, params.Seed = 2, 73
	input := makeGradient(80, 48, 37)
	before := append([]byte(nil), input.Data...)
	image, err := EditMOSSImage(ctx, input, params)
	if err != nil {
		t.Fatal(err)
	}
	if image.Width != 96 || image.Height != 64 || image.Channel != 3 || len(image.Data) != 96*64*3 {
		t.Fatalf("edited image: got %dx%dx%d, %d bytes", image.Width, image.Height, image.Channel, len(image.Data))
	}
	if !bytes.Equal(input.Data, before) {
		t.Fatal("EditMOSSImage mutated the reference")
	}
	outputs, err := RunDGL(ctx, image.GenerationInfo, map[string]SDImage{"image": input})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := outputs["result"].Images()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(image.Data, replayed[0].Data) {
		t.Fatal("MOSS editing source did not reproduce pixels")
	}
	for i := range input.Data {
		input.Data[i] = 255 - input.Data[i]
	}
	changed, err := EditMOSSImage(ctx, input, params)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(changed.Data, image.Data) {
		t.Fatal("MOSS editing ignored the reference image")
	}
	FreeContext(ctx)
	ctx = 0
	path := filepath.Join(t.TempDir(), "moss-edit.png")
	if err := image.SavePNG(path); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := LoadPNG(path)
	if err != nil || !bytes.Equal(roundTrip.Data, image.Data) {
		t.Fatalf("MOSS PNG round trip after context release: %v", err)
	}
}

func TestGenerateVideoAnimateDiff(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_VIDEO_TEST_DIR")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	cparams := ContextParamsInit()
	cparams.ModelPath = manifest.Files[string(download.RoleModel)]
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	params := AnimateDiffParamsInit()
	params.ModelPath = cparams.ModelPath
	params.MotionModulePath = manifest.Files[string(download.RoleMotionModule)]
	params.Prompt = "a cat walking"
	params.NegativePrompt = "blurry \"cat\"\nlow quality"
	params.Width, params.Height = 128, 64
	params.Frames, params.Steps = 3, 2
	params.FPS, params.Seed, params.CFGScale = 11, 73, 5.5
	video, err := GenerateAnimateDiff(ctx, params)
	if err != nil {
		t.Fatalf("GenerateAnimateDiff: %v", err)
	}
	frames := video.Frames
	if len(frames) != 3 {
		t.Fatalf("frames: got %d, want 3", len(frames))
	}
	if video.FPS != 11 {
		t.Fatalf("FPS: got %d, want 11", video.FPS)
	}
	for i, frame := range frames {
		if frame.Width != 128 || frame.Height != 64 || len(frame.Data) != 128*64*3 {
			t.Fatalf("GenerateVideoWithFPS frame %d is invalid: %#v", i, frame)
		}
		if frame.GenerationInfo != video.GenerationInfo || frame.GenerationInfo == "" {
			t.Fatalf("frame %d is missing its replayable generation source", i)
		}
	}
	params.Seed++
	changed, err := GenerateAnimateDiff(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(changed.Frames[0].Data, frames[0].Data) {
		t.Fatal("GenerateAnimateDiff ignored seed override")
	}
	outputs, err := RunDGL(ctx, video.GenerationInfo, nil)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := outputs["video"].Images()
	if err != nil {
		t.Fatal(err)
	}
	for i := range frames {
		if !bytes.Equal(frames[i].Data, replayed[i].Data) {
			t.Fatalf("AnimateDiff source did not reproduce frame %d", i)
		}
	}
}
