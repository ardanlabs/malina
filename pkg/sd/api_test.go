package sd

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
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
		{"cImagePreprocessParams", unsafe.Sizeof(cImagePreprocessParams{}), 8},
		{"cImgGenParams", unsafe.Sizeof(cImgGenParams{}), 552},
		{"cAudio", unsafe.Sizeof(cAudio{}), 24},
		{"cRefVideo", unsafe.Sizeof(cRefVideo{}), 40},
		{"cVidGenParams", unsafe.Sizeof(cVidGenParams{}), 584},
		{"cADetailerParams", unsafe.Sizeof(cADetailerParams{}), 24},
		{"PreviewInfo", unsafe.Sizeof(PreviewInfo{}), 8},
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
		{"ImagePreprocess", unsafe.Offsetof(video.ImagePreprocess), 576},
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
		{"cContextParams.AudioEncoderPath", unsafe.Offsetof(context.AudioEncoderPath), 104},
		{"cContextParams.Embeddings", unsafe.Offsetof(context.Embeddings), 144},
		{"cContextParams.PhotoMakerPath", unsafe.Offsetof(context.PhotoMakerPath), 160},
		{"cContextParams.NThreads", unsafe.Offsetof(context.NThreads), 184},
		{"cContextParams.VaeFormat", unsafe.Offsetof(context.VaeFormat), 216},
		{"cContextParams.MaxVram", unsafe.Offsetof(context.MaxVram), 224},
		{"cContextParams.DisablePrefetch", unsafe.Offsetof(context.DisablePrefetch), 232},
		{"cContextParams.Backend", unsafe.Offsetof(context.Backend), 240},
		{"cContextParams.RPCServers", unsafe.Offsetof(context.RPCServers), 272},
		{"cContextParams.DisableSegmentedCompute", unsafe.Offsetof(context.DisableSegmentedCompute), 288},
		{"cContextParams.BatchedCFG", unsafe.Offsetof(context.BatchedCFG), 289},
		{"cContextParams.LinearScale", unsafe.Offsetof(context.LinearScale), 292},
		{"cContextParams.AttnScale", unsafe.Offsetof(context.AttnScale), 296},
		{"cContextParams.Tokenizer", unsafe.Offsetof(context.Tokenizer), 304},
		{"cContextParams.SageAttn", unsafe.Offsetof(context.SageAttn), 312},
		{"cContextParams.ConditioningCacheSize", unsafe.Offsetof(context.ConditioningCacheSize), 316},
		{"cContextParams.FlashAttnMode", unsafe.Offsetof(context.FlashAttnMode), 320},
		{"cContextParams.ConvDirectMode", unsafe.Offsetof(context.ConvDirectMode), 328},
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
		{"cImgGenParams.ImagePreprocess", unsafe.Offsetof(image.ImagePreprocess), 544},
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

func TestConvertWithComponentsLoras(t *testing.T) {
	testSetup(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.safetensors")
	lora := filepath.Join(dir, "lora.safetensors")
	// Small real safetensors fixtures exercise native merging, without a
	// licensed model download. W + 0.5 * up * down = [4, 8; 9, 14].
	writeTestSafetensors(t, model, `{"model.diffusion_model.input_blocks.0.0.weight":{"dtype":"F32","shape":[2,2],"data_offsets":[0,16]}}`, []float32{1, 2, 3, 2})
	writeTestSafetensors(t, lora, `{"lora_unet_input_blocks_0_0.lora_down.weight":{"dtype":"F32","shape":[1,2],"data_offsets":[0,8]},"lora_unet_input_blocks_0_0.lora_up.weight":{"dtype":"F32","shape":[2,1],"data_offsets":[8,16]}}`, []float32{2, 4, 3, 6})
	for _, tc := range []struct {
		name  string
		loras []Lora
		want  []float32
	}{
		{"plain", nil, []float32{1, 2, 3, 2}},
		{"merged", []Lora{{Path: lora, Multiplier: 0.5}}, []float32{4, 8, 9, 14}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(dir, tc.name+".safetensors")
			if err := ConvertWithComponents(ComponentConvertParams{ModelPath: model, OutputPath: output, OutputType: SDTypeF32, NThreads: 1, Loras: tc.loras}); err != nil {
				t.Fatalf("ConvertWithComponents: %v", err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) < 8 {
				t.Fatal("missing safetensors header")
			}
			headerEnd := 8 + binary.LittleEndian.Uint64(data[:8])
			if headerEnd > uint64(len(data)) {
				t.Fatal("truncated safetensors header")
			}
			var header map[string]struct {
				Offsets [2]int `json:"data_offsets"`
			}
			if err := json.Unmarshal(data[8:headerEnd], &header); err != nil {
				t.Fatal(err)
			}
			tensor, ok := header["model.diffusion_model.input_blocks.0.0.weight"]
			if !ok {
				t.Fatal("converted tensor missing")
			}
			got := make([]float32, 4)
			if _, err := binary.Decode(data[headerEnd+uint64(tensor.Offsets[0]):], binary.LittleEndian, got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("converted weights: got %v, want %v", got, tc.want)
			}
		})
	}
}

func writeTestSafetensors(t *testing.T, path, header string, weights []float32) {
	t.Helper()
	data := binary.LittleEndian.AppendUint64(nil, uint64(len(header)))
	data = append(data, header...)
	data, err := binary.Append(data, binary.LittleEndian, weights)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
