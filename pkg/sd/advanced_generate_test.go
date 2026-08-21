//go:build malina_model_tests

package sd

import (
	"testing"
)

func TestControlNetHotSwapSmoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_TEST_MODEL")
	controlPath := testEnvModelFile(t, "MALINA_CONTROLNET_TEST_MODEL")

	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	if err := LoadControlNet(ctx, controlPath); err != nil {
		t.Fatalf("LoadControlNet: %v", err)
	}
	hasControl, err := HasControlNet(ctx)
	if err != nil {
		t.Fatalf("HasControlNet: %v", err)
	}
	if !hasControl {
		t.Fatal("HasControlNet: got false after loading ControlNet")
	}
	if err := UnloadControlNet(ctx); err != nil {
		t.Fatalf("UnloadControlNet: %v", err)
	}
	hasControl, err = HasControlNet(ctx)
	if err != nil {
		t.Fatalf("HasControlNet after unload: %v", err)
	}
	if hasControl {
		t.Fatal("HasControlNet: got true after unloading ControlNet")
	}
}

func TestUpscalerSmoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_UPSCALER_TEST_MODEL")

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
	input := &SDImage{Width: 8, Height: 8, Channel: 3, Data: make([]byte, 8*8*3)}
	for i := range input.Data {
		input.Data[i] = byte(i)
	}
	images, err := Upscale(ctx, input, uint32(factor))
	if err != nil {
		t.Fatalf("Upscale: %v", err)
	}
	if len(images) == 0 || images[0] == nil {
		t.Fatal("Upscale returned no image")
	}
	if images[0].Width != input.Width*uint32(factor) || images[0].Height != input.Height*uint32(factor) {
		t.Errorf("upscaled dimensions: got %dx%d, want %dx%d", images[0].Width, images[0].Height, input.Width*uint32(factor), input.Height*uint32(factor))
	}
}

func TestADetailerSmoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_TEST_MODEL")
	detectorPath := testEnvModelFile(t, "MALINA_ADETAILER_TEST_MODEL")

	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	detailer, err := NewADetailerContext(detectorPath, NumPhysicalCores(), "", "")
	if err != nil {
		t.Fatalf("NewADetailerContext: %v", err)
	}
	defer FreeADetailerContext(detailer)

	input := &SDImage{Width: 64, Height: 64, Channel: 3, Data: make([]byte, 64*64*3)}
	params := ImgGenParamsInit()
	params.Prompt = "a face"
	params.Width = 64
	params.Height = 64
	params.Steps = 1
	params.Seed = 42
	images, err := ADetailImage(detailer, ctx, input, ADetailerParams{Prompt: "a face"}, params)
	if err != nil {
		t.Fatalf("ADetailImage: %v", err)
	}
	for i, image := range images {
		if image == nil || len(image.Data) == 0 {
			t.Fatalf("ADetailImage result %d is empty", i)
		}
	}
}

func TestGenerateVideoSmoke(t *testing.T) {
	testSetup(t)
	modelPath := testEnvModelFile(t, "MALINA_VIDEO_TEST_MODEL")

	cparams := ContextParamsInit()
	cparams.ModelPath = modelPath
	ctx, err := NewContext(cparams)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	defer FreeContext(ctx)

	supportsVideo, err := ContextSupportsVideoGeneration(ctx)
	if err != nil {
		t.Fatalf("ContextSupportsVideoGeneration: %v", err)
	}
	if !supportsVideo {
		t.Fatal("video fixture does not report video-generation support")
	}
	params, err := VideoGenParamsInit()
	if err != nil {
		t.Fatalf("VideoGenParamsInit: %v", err)
	}
	params.Prompt = "a cat walking"
	params.Width = 64
	params.Height = 64
	params.Sample.Steps = 1
	params.VideoFrames = 1
	params.FPS = 1
	params.Seed = 42
	frames, audio, err := GenerateVideo(ctx, params)
	if err != nil {
		t.Fatalf("GenerateVideo: %v", err)
	}
	if len(frames) == 0 || frames[0] == nil || len(frames[0].Data) == 0 {
		t.Fatal("GenerateVideo returned no copied frame pixels")
	}
	if audio != nil && audio.Channels > 0 && len(audio.Data)%int(audio.Channels) != 0 {
		t.Fatalf("audio samples %d are not divisible by %d channels", len(audio.Data), audio.Channels)
	}
}
