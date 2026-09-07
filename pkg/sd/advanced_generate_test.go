//go:build malina_model_tests

package sd

import (
	"bytes"
	"path/filepath"
	"strings"
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

	if err := LoadControlNet(ctx, manifest.Files[string(download.RoleControlNet)]); err != nil {
		t.Fatalf("LoadControlNet: %v", err)
	}
	hasControl, err := HasControlNet(ctx)
	if err != nil {
		t.Fatalf("HasControlNet: %v", err)
	}
	if !hasControl {
		t.Fatal("HasControlNet: got false after loading ControlNet")
	}

	control := &SDImage{Width: 64, Height: 64, Channel: 3, Data: make([]byte, 64*64*3)}
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
	if err := PreprocessCanny(control, CannyParams{HighThreshold: 0.08, LowThreshold: 0.08, Weak: 0.8, Strong: 1}); err != nil {
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
	if image == nil || image.Width != 64 || image.Height != 64 || len(image.Data) != 64*64*3 {
		t.Fatalf("GenerateImage with ControlNet returned invalid image: %#v", image)
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

func TestUpscaleImage(t *testing.T) {
	testSetup(t)
	bundleDir := testEnvBundleDir(t, "MALINA_UPSCALER_TEST_DIR")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	ctx, err := NewUpscalerContext(manifest.Files[string(download.RoleUpscaler)], false, NumPhysicalCores(), 0, "", "")
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

func TestADetailImage(t *testing.T) {
	testSetup(t)
	var logs []string
	SetLogCallback(func(_ LogLevel, text string) {
		if strings.Contains(text, "ADetailer") {
			logs = append(logs, text)
		}
	})
	t.Cleanup(func() { SetLogCallback(nil) })

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

	detailer, err := NewADetailerContext(manifest.Files[string(download.RoleADetailer)], NumPhysicalCores(), "cpu", "")
	if err != nil {
		t.Fatalf("NewADetailerContext: %v", err)
	}
	defer FreeADetailerContext(detailer)

	input, err := LoadPNG(filepath.Join("..", "..", "samples", "adetailer-face.png"))
	if err != nil {
		t.Fatalf("LoadPNG: %v", err)
	}
	before := append([]byte(nil), input.Data...)
	params := ImgGenParamsInit()
	params.Prompt = "a detailed portrait photo"
	params.Width = int32(input.Width)
	params.Height = int32(input.Height)
	params.Steps = 1
	params.Seed = 42
	images, err := ADetailImage(detailer, ctx, input, ADetailerParams{
		Prompt:    "a detailed portrait photo",
		ExtraArgs: "input_size=640,confidence=0.3,inpaint_width=64,inpaint_height=64",
	}, params)
	if err != nil {
		t.Fatalf("ADetailImage: %v", err)
	}
	if len(images) == 0 {
		t.Fatal("ADetailImage returned no images")
	}
	image := images[len(images)-1]
	if image == nil || image.Width != input.Width || image.Height != input.Height || len(image.Data) != len(input.Data) {
		t.Fatalf("ADetailImage returned invalid final image: %#v", image)
	}
	if bytes.Equal(image.Data, before) {
		t.Fatalf("ADetailImage did not refine the detected face; logs: %v", logs)
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
	cparams.MotionModulePath = manifest.Files[string(download.RoleMotionModule)]
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
	params.Width = 128
	params.Height = 128
	params.Sample.Steps = 4
	params.VideoFrames = 4
	params.FPS = 1
	params.Seed = 42
	frames, audio, err := GenerateVideo(ctx, params)
	if err != nil {
		t.Fatalf("GenerateVideo: %v", err)
	}
	if len(frames) != int(params.VideoFrames) {
		t.Fatalf("GenerateVideo returned %d frames, want %d", len(frames), params.VideoFrames)
	}
	for i, frame := range frames {
		if frame == nil || frame.Width != uint32(params.Width) || frame.Height != uint32(params.Height) || len(frame.Data) != int(params.Width*params.Height*3) {
			t.Fatalf("GenerateVideo frame %d is invalid: %#v", i, frame)
		}
	}
	if audio != nil && audio.Channels > 0 && len(audio.Data)%int(audio.Channels) != 0 {
		t.Fatalf("audio samples %d are not divisible by %d channels", len(audio.Data), audio.Channels)
	}
}
