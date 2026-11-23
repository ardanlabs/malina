package sd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
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
		{"DGL input size", unsafe.Sizeof(cDGLInput{}), 16}, {"DGL input tensor", unsafe.Offsetof(cDGLInput{}.Tensor), 8},
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
	image, err := PreprocessCanny(image, CannyParams{HighThreshold: .08, LowThreshold: .08, Weak: .8, Strong: 1})
	if err != nil {
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

func testMarshalImage(value byte) SDImage {
	return SDImage{Width: 1, Height: 1, Channel: 3, Data: []byte{value, value + 1, value + 2}}
}

func TestRunDGL(t *testing.T) {
	testSetup(t)
	ctx, err := NewContext(ContextParamsInit())
	if err != nil {
		t.Fatal(err)
	}
	input := makeGradient(17, 11, 37)
	source := `fn main(image) { return {cropped = tensor::crop(image, axes=[1,2], start=[3,2], size=[5,4]), count=7} }`
	outputs, err := RunDGL(ctx, source, map[string]SDImage{"image": input})
	if err != nil {
		FreeContext(ctx)
		t.Fatal(err)
	}
	if _, err := RunDGL(ctx, "fn main(image) { return image }", nil); err == nil {
		t.Error("missing input accepted")
	}
	if _, err := RunDGL(ctx, "not valid DGL", nil); err == nil {
		t.Error("invalid source accepted")
	}
	FreeContext(ctx)
	images, err := outputs["cropped"].Images()
	if err != nil {
		t.Fatal(err)
	}
	image := images[0]
	if image.Width != 5 || image.Height != 4 || image.Channel != 3 {
		t.Fatalf("crop dimensions: got %+v", image)
	}
	for y := range 4 {
		for x := range 5 {
			got := image.Data[(y*5+x)*3 : (y*5+x+1)*3]
			want := input.Data[((y+2)*17+x+3)*3 : ((y+2)*17+x+4)*3]
			if !bytes.Equal(got, want) {
				t.Fatalf("crop (%d,%d): got %v, want %v", x, y, got, want)
			}
		}
	}
	if got := outputs["count"]; got.Type != 3 || len(got.Data) != 8 || binary.LittleEndian.Uint64(got.Data) != 7 {
		t.Errorf("count: got %+v, want I64 7", got)
	}
	if metadata, err := GenerationMetadata("fn main() { return 7 }"); err != nil || metadata != "{}" {
		t.Errorf("non-image metadata: got %q, %v", metadata, err)
	}
}

func TestRunDGLProgressAndCancellation(t *testing.T) {
	testSetup(t)
	for _, asynchronous := range []bool{false, true} {
		t.Run(fmt.Sprintf("other_goroutine=%t", asynchronous), func(t *testing.T) {
			ctx, err := NewContext(ContextParamsInit())
			if err != nil {
				t.Fatal(err)
			}
			defer FreeContext(ctx)
			other, err := NewContext(ContextParamsInit())
			if err != nil {
				t.Fatal(err)
			}
			defer FreeContext(other)
			var events []GenerationProgress
			var cancelErr error
			if err := SetGenerationProgressCallback(ctx, func(event GenerationProgress) {
				events = append(events, event)
				if !asynchronous {
					cancelErr = CancelGeneration(ctx, CancelAll)
					return
				}
				done := make(chan error, 1)
				go func() { done <- CancelGeneration(ctx, CancelAll) }()
				cancelErr = <-done
			}); err != nil {
				t.Fatal(err)
			}
			const source = "fn main() { return 7 }"
			if _, err := RunDGL(other, source, nil); err != nil || len(events) != 0 {
				t.Fatalf("independent context: got %v, %d events, want nil and 0", err, len(events))
			}
			outputs, err := RunDGL(ctx, source, nil)
			if native, ok := errors.AsType[*NativeError](err); !ok || native.Status != StatusCancelled || outputs != nil {
				t.Fatalf("canceled run: got %v, %v, want no outputs and StatusCancelled", outputs, err)
			}
			if cancelErr != nil || len(events) != 1 || events[0].Stage != "executing" {
				t.Fatalf("progress cancellation: got %v, %+v, want nil and executing event", cancelErr, events)
			}
			if err := SetGenerationProgressCallback(ctx, nil); err != nil {
				t.Fatal(err)
			}
			if err := CancelGeneration(ctx, CancelAll); err != nil {
				t.Fatal(err)
			}
			outputs, err = RunDGL(ctx, source, nil)
			if err != nil || len(events) != 1 || len(outputs) != 1 {
				t.Fatalf("reuse after clearing callback and idle cancellation: got %v, %v, %+v", outputs, err, events)
			}
			for _, result := range outputs {
				if result.Type != 3 || len(result.Data) != 8 || binary.LittleEndian.Uint64(result.Data) != 7 {
					t.Errorf("reused result: got %+v, want I64 7", result)
				}
			}
		})
	}
}
