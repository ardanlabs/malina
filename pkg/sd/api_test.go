package sd

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func TestExtendedStructLayouts(t *testing.T) {
	tests := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"cEmbedding", unsafe.Sizeof(cEmbedding{}), 16},
		{"cImage", unsafe.Sizeof(cImage{}), 24},
		{"cSlgParams", unsafe.Sizeof(cSlgParams{}), 32},
		{"cGuidanceParams", unsafe.Sizeof(cGuidanceParams{}), 48},
		{"cSampleParams", unsafe.Sizeof(cSampleParams{}), 96},
		{"cPMParams", unsafe.Sizeof(cPMParams{}), 32},
		{"cPulidParams", unsafe.Sizeof(cPulidParams{}), 16},
		{"cTilingParams", unsafe.Sizeof(cTilingParams{}), 32},
		{"cCacheParams", unsafe.Sizeof(cCacheParams{}), 96},
		{"cLora", unsafe.Sizeof(cLora{}), 16},
		{"cHiresParams", unsafe.Sizeof(cHiresParams{}), 56},
		{"cImgGenParams", unsafe.Sizeof(cImgGenParams{}), 544},
		{"cAudio", unsafe.Sizeof(cAudio{}), 24},
		{"cRefVideo", unsafe.Sizeof(cRefVideo{}), 40},
		{"cVidGenParams", unsafe.Sizeof(cVidGenParams{}), 576},
		{"cADetailerParams", unsafe.Sizeof(cADetailerParams{}), 24},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("size = %d, want %d", tt.got, tt.want)
			}
		})
	}

	var video cVidGenParams
	offsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"SampleParams", unsafe.Offsetof(video.SampleParams), 160},
		{"HighNoiseSampleParams", unsafe.Offsetof(video.HighNoiseSampleParams), 256},
		{"MoeBoundary", unsafe.Offsetof(video.MoeBoundary), 352},
		{"VAETilingParams", unsafe.Offsetof(video.VAETilingParams), 384},
		{"Cache", unsafe.Offsetof(video.Cache), 416},
		{"Hires", unsafe.Offsetof(video.Hires), 512},
		{"CircularX", unsafe.Offsetof(video.CircularX), 568},
	}
	for _, tt := range offsets {
		if tt.got != tt.want {
			t.Errorf("cVidGenParams.%s offset = %d, want %d", tt.name, tt.got, tt.want)
		}
	}

	var context cContextParams
	var image cImgGenParams
	var sample cSampleParams
	var cache cCacheParams
	var hires cHiresParams
	criticalOffsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"cContextParams.Embeddings", unsafe.Offsetof(context.Embeddings), 136},
		{"cContextParams.PhotoMakerPath", unsafe.Offsetof(context.PhotoMakerPath), 152},
		{"cContextParams.NThreads", unsafe.Offsetof(context.NThreads), 176},
		{"cContextParams.VaeFormat", unsafe.Offsetof(context.VaeFormat), 208},
		{"cContextParams.MaxVram", unsafe.Offsetof(context.MaxVram), 216},
		{"cContextParams.Backend", unsafe.Offsetof(context.Backend), 232},
		{"cContextParams.RPCServers", unsafe.Offsetof(context.RPCServers), 264},
		{"cImgGenParams.InitImage", unsafe.Offsetof(image.InitImage), 40},
		{"cImgGenParams.RefImages", unsafe.Offsetof(image.RefImages), 64},
		{"cImgGenParams.MaskImage", unsafe.Offsetof(image.MaskImage), 88},
		{"cImgGenParams.SampleParams", unsafe.Offsetof(image.SampleParams), 120},
		{"cImgGenParams.Seed", unsafe.Offsetof(image.Seed), 224},
		{"cImgGenParams.ControlImage", unsafe.Offsetof(image.ControlImage), 240},
		{"cImgGenParams.PMParams", unsafe.Offsetof(image.PMParams), 304},
		{"cImgGenParams.Cache", unsafe.Offsetof(image.Cache), 384},
		{"cImgGenParams.Hires", unsafe.Offsetof(image.Hires), 480},
		{"cImgGenParams.CircularX", unsafe.Offsetof(image.CircularX), 540},
		{"cSampleParams.CustomSigmas", unsafe.Offsetof(sample.CustomSigmas), 72},
		{"cSampleParams.ExtraSampleArgs", unsafe.Offsetof(sample.ExtraSampleArgs), 88},
		{"cCacheParams.SCMMask", unsafe.Offsetof(cache.SCMMask), 56},
		{"cCacheParams.SpectrumW", unsafe.Offsetof(cache.SpectrumW), 68},
		{"cHiresParams.ModelPath", unsafe.Offsetof(hires.ModelPath), 8},
		{"cHiresParams.CustomSigmas", unsafe.Offsetof(hires.CustomSigmas), 40},
	}
	for _, tt := range criticalOffsets {
		if tt.got != tt.want {
			t.Errorf("%s offset = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestExtendedTargetAPI(t *testing.T) {
	testSetup(t)

	commit, err := Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if commit == "" {
		t.Fatal("Commit returned an empty string")
	}

	name, err := SampleMethodName(SampleEuler)
	if err != nil {
		t.Fatalf("SampleMethodName: %v", err)
	}
	parsed, err := ParseSampleMethod(name)
	if err != nil {
		t.Fatalf("ParseSampleMethod: %v", err)
	}
	if parsed != SampleEuler {
		t.Fatalf("ParseSampleMethod(%q) = %d, want %d", name, parsed, SampleEuler)
	}

	devices, err := ListDevices()
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	for i, device := range devices {
		if strings.TrimSpace(device.Name) == "" {
			t.Errorf("device %d has an empty name: %#v", i, device)
		}
	}

	imageParams := ImgGenParamsInit()
	if imageParams.Cache != CacheParamsInit() {
		t.Errorf("image cache defaults differ from sd_cache_params_init")
	}
	if !reflect.DeepEqual(imageParams.Hires, HiresParamsInit()) {
		t.Errorf("image hires defaults differ from sd_hires_params_init")
	}

	videoParams, err := VideoGenParamsInit()
	if err != nil {
		t.Fatalf("VideoGenParamsInit: %v", err)
	}
	if videoParams.Width == 0 || videoParams.Height == 0 {
		t.Fatalf("VideoGenParamsInit dimensions = %dx%d", videoParams.Width, videoParams.Height)
	}
}

func TestPreprocessCannyTargetAPI(t *testing.T) {
	testSetup(t)

	image := &SDImage{Width: 8, Height: 8, Channel: 3, Data: make([]byte, 8*8*3)}
	for y := range 8 {
		for x := range 8 {
			value := byte(0)
			if x >= 4 {
				value = 255
			}
			for channel := range 3 {
				image.Data[(y*8+x)*3+channel] = value
			}
		}
	}
	if err := PreprocessCanny(image, CannyParams{HighThreshold: 0.08, LowThreshold: 0.08, Weak: 0.8, Strong: 1}); err != nil {
		t.Fatalf("PreprocessCanny: %v", err)
	}
}
