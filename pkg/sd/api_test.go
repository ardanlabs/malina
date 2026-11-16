package sd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"
)

func TestLiveStructLayouts(t *testing.T) {
	// Target-header C probes on LP64 and LLP64 agree on these values.
	for _, tc := range []struct {
		name      string
		got, want uintptr
	}{
		{"audio size", unsafe.Sizeof(cAudio{}), 16}, {"audio data", unsafe.Offsetof(cAudio{}.Data), 8},
		{"video size", unsafe.Sizeof(cVideo{}), 32}, {"video fps", unsafe.Offsetof(cVideo{}.FPS), 8}, {"video audio", unsafe.Offsetof(cVideo{}.Audio), 16},
		{"embedding size", unsafe.Sizeof(cEmbedding{}), 16}, {"embedding path", unsafe.Offsetof(cEmbedding{}.Path), 8},
		{"lora size", unsafe.Sizeof(cLora{}), 16}, {"lora multiplier", unsafe.Offsetof(cLora{}.Multiplier), 4}, {"lora path", unsafe.Offsetof(cLora{}.Path), 8},
		{"progress size", unsafe.Sizeof(cGenerationProgress{}), 48}, {"progress completed", unsafe.Offsetof(cGenerationProgress{}.Completed), 32},
		{"preview size", unsafe.Sizeof(PreviewInfo{}), 8},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestPreprocessCannyTargetAPI(t *testing.T) {
	testSetup(t)
	image := makeGradient(17, 11, 50)
	before := append([]byte(nil), image.Data...)
	backing := image.Data
	if err := PreprocessCanny(image, CannyParams{HighThreshold: .08, LowThreshold: .08, Weak: .8, Strong: 1}); err != nil {
		t.Fatal(err)
	}
	if image.Width != 17 || image.Height != 11 || image.Channel != 3 {
		t.Fatalf("Canny dimensions: got %dx%dx%d", image.Width, image.Height, image.Channel)
	}
	if bytes.Equal(image.Data, before) {
		t.Fatal("Canny output merely echoes input")
	}
	if !bytes.Equal(backing, before) {
		t.Fatal("Canny mutated the old pixel backing")
	}
	if err := image.SavePNG(filepath.Join(t.TempDir(), "canny.png")); err != nil {
		t.Fatal(err)
	}
}

func TestConvertWithComponentsLoras(t *testing.T) {
	testSetup(t)
	dir := t.TempDir()
	model, lora := filepath.Join(dir, "model.safetensors"), filepath.Join(dir, "lora.safetensors")
	writeTestSafetensors(t, model, `{"model.diffusion_model.input_blocks.0.0.weight":{"dtype":"F32","shape":[2,2],"data_offsets":[0,16]}}`, []float32{1, 2, 3, 2})
	writeTestSafetensors(t, lora, `{"lora_unet_input_blocks_0_0.lora_down.weight":{"dtype":"F32","shape":[1,2],"data_offsets":[0,8]},"lora_unet_input_blocks_0_0.lora_up.weight":{"dtype":"F32","shape":[2,1],"data_offsets":[8,16]}}`, []float32{2, 4, 3, 6})
	for _, tc := range []struct {
		name  string
		loras []Lora
		want  []float32
	}{
		{"plain", nil, []float32{1, 2, 3, 2}}, {"merged", []Lora{{Path: lora, Multiplier: .5}}, []float32{4, 8, 9, 14}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(dir, tc.name+".safetensors")
			if err := ConvertWithComponents(ComponentConvertParams{ModelPath: model, OutputPath: output, OutputType: SDTypeF32, NThreads: 1, Loras: tc.loras}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) < 8 {
				t.Fatal("missing safetensors header")
			}
			end := 8 + binary.LittleEndian.Uint64(data[:8])
			if end > uint64(len(data)) {
				t.Fatal("truncated safetensors header")
			}
			var header map[string]struct {
				Offsets [2]int `json:"data_offsets"`
			}
			if err := json.Unmarshal(data[8:end], &header); err != nil {
				t.Fatal(err)
			}
			tensor, ok := header["model.diffusion_model.input_blocks.0.0.weight"]
			if !ok {
				t.Fatal("missing output tensor")
			}
			got := make([]float32, 4)
			if _, err := binary.Decode(data[end+uint64(tensor.Offsets[0]):], binary.LittleEndian, got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("weights: got %v, want %v", got, tc.want)
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

func testMarshalImage(value byte) *SDImage {
	return &SDImage{Width: 1, Height: 1, Channel: 3, Data: []byte{value, value + 1, value + 2}}
}
