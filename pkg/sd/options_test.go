package sd

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestEnumAdapters(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		get   func(int32) (string, error)
		parse func(string) (int32, error)
	}{
		{"rng", 3, func(v int32) (string, error) { return RngTypeName(RngType(v)) }, func(s string) (int32, error) { v, e := ParseRngType(s); return int32(v), e }},
		{"sampler", 21, func(v int32) (string, error) { return SampleMethodName(SampleMethod(v)) }, func(s string) (int32, error) { v, e := ParseSampleMethod(s); return int32(v), e }},
		{"scheduler", 17, func(v int32) (string, error) { return SchedulerName(Scheduler(v)) }, func(s string) (int32, error) { v, e := ParseScheduler(s); return int32(v), e }},
		{"prediction", 11, func(v int32) (string, error) { return PredictionName(Prediction(v)) }, func(s string) (int32, error) { v, e := ParsePrediction(s); return int32(v), e }},
		{"preview", 4, func(v int32) (string, error) { return PreviewModeName(PreviewMode(v)) }, func(s string) (int32, error) { v, e := ParsePreviewMode(s); return int32(v), e }},
		{"hires", 10, func(v int32) (string, error) { return HiresUpscalerName(HiresUpscaler(v)) }, func(s string) (int32, error) { v, e := ParseHiresUpscaler(s); return int32(v), e }},
		{"control", 9, func(v int32) (string, error) { return ControlTypeName(ControlType(v)) }, func(s string) (int32, error) { v, e := ParseControlType(s); return int32(v), e }},
		{"lora", 3, func(v int32) (string, error) { return LoraApplyModeName(LoraApplyMode(v)) }, func(s string) (int32, error) { v, e := ParseLoraApplyMode(s); return int32(v), e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i := range tc.count {
				name, err := tc.get(int32(i))
				if err != nil {
					t.Fatal(err)
				}
				got, err := tc.parse(name)
				if err != nil || got != int32(i) {
					t.Fatalf("round trip %q: got %d, %v, want %d", name, got, err, i)
				}
			}
			if _, err := tc.get(-1); err == nil {
				t.Error("negative enum accepted")
			}
			if _, err := tc.parse("bad\x00name"); err == nil {
				t.Error("NUL name accepted")
			}
		})
	}
	for _, tc := range []struct {
		value string
		want  ControlType
	}{{"hed", ControlTypeScribble}, {"pidi", ControlTypeScribble}, {"seg", ControlTypeSegment}, {"lineart", ControlTypeCanny}, {"outpaint", ControlTypeRepaint}} {
		got, err := ParseControlType(tc.value)
		if err != nil || got != tc.want {
			t.Errorf("alias %q: got %v, %v, want %v", tc.value, got, err, tc.want)
		}
	}
}

func TestOptionFieldKeysAgainstNative(t *testing.T) {
	testSetup(t)
	video, err := VideoGenParamsInit()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		purpose  string
		defaults any
	}{{"ctx", ContextParamsInit()}, {"img_gen", ImgGenParamsInit()}, {"vid_gen", video}} {
		t.Run(tc.purpose, func(t *testing.T) {
			value := reflect.New(reflect.TypeOf(tc.defaults)).Elem()
			value.Set(reflect.ValueOf(tc.defaults))
			changeOptionFields(value)
			o, err := newOptions(tc.purpose)
			if err != nil {
				t.Fatal(err)
			}
			defer o.free()
			if err := o.fields(value.Interface(), tc.defaults, ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Force every compatibility field through its actual typed setter. This catches
// missing/incorrect key mappings, rather than merely asserting a local name table.
func changeOptionFields(v reflect.Value) {
	for i := range v.NumField() {
		f := v.Field(i)
		if v.Type().Field(i).Name == "Options" {
			continue
		}
		switch f.Kind() {
		case reflect.Struct:
			changeOptionFields(f)
		case reflect.Bool:
			f.SetBool(!f.Bool())
		case reflect.String:
			f.SetString("test")
		case reflect.Int32, reflect.Int64:
			if _, ok, _ := enumOption(f.Interface()); ok {
				if f.Int() == 0 {
					f.SetInt(1)
				} else {
					f.SetInt(0)
				}
			} else {
				f.SetInt(3)
			}
		case reflect.Float32:
			f.SetFloat(.3)
		case reflect.Pointer:
			f.Set(reflect.ValueOf(testMarshalImage(1)))
		case reflect.Slice:
			switch f.Interface().(type) {
			case []float32:
				f.Set(reflect.ValueOf([]float32{1, .5, 0}))
			case []int32:
				f.Set(reflect.ValueOf([]int32{1, 3}))
			case []*SDImage:
				f.Set(reflect.ValueOf([]*SDImage{testMarshalImage(1), testMarshalImage(4)}))
			case []Lora:
				f.Set(reflect.ValueOf([]Lora{{Path: "lora", Multiplier: .25}}))
			case []Embedding:
				f.Set(reflect.ValueOf([]Embedding{{Name: "name", Path: "path"}}))
			case []Audio:
				f.Set(reflect.ValueOf([]Audio{{SampleRate: 44100, Channels: 2, Data: []float32{.1, .2, .3, .4}}}))
			case []RefVideo:
				f.Set(reflect.ValueOf([]RefVideo{{Frames: []*SDImage{testMarshalImage(7)}, FPS: 12, Audio: &Audio{SampleRate: 24000, Channels: 1, Data: []float32{.5, .75}}}}))
			}
		}
	}
}

func TestDiagnosticsStayOnCallingThread(t *testing.T) {
	testSetup(t)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			key := strings.Repeat("x", i+1)
			p := ContextParamsInit()
			p.Options = Options{key: true}
			ctx, err := NewContext(p)
			defer FreeContext(ctx)
			var native *NativeError
			if !errors.As(err, &native) || !strings.Contains(native.Message, key) {
				t.Errorf("diagnostic for %q: got %v", key, err)
			}
		})
	}
	wg.Wait()
}

func TestPreviewRegistrationValidation(t *testing.T) {
	testSetup(t)
	if err := SetPreviewCallback(nil, PreviewProj, -3, true, false); err != nil {
		t.Fatal(err)
	}
	if err := SetPreviewCallback(nil, PreviewMode(-1), 1, false, false); err == nil {
		t.Fatal("invalid mode accepted")
	}
	if err := SetPreviewCallback(nil, PreviewNone, 1, false, false); err != nil {
		t.Fatal(err)
	}
}
