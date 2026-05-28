package sd

import (
	"math"
	"testing"
	"unsafe"
)

// TestNestedStructSizes verifies every C-mirror struct used inside
// cImgGenParams has the exact byte size the C ABI expects. Any drift here
// causes nested fields in cImgGenParams to misalign and GenerateImage to
// crash.
func TestNestedStructSizes(t *testing.T) {
	cases := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"cImage", unsafe.Sizeof(cImage{}), 24},
		{"cSlgParams", unsafe.Sizeof(cSlgParams{}), 32},
		{"cGuidanceParams", unsafe.Sizeof(cGuidanceParams{}), 48},
		{"cSampleParams", unsafe.Sizeof(cSampleParams{}), 96},
		{"cPMParams", unsafe.Sizeof(cPMParams{}), 32},
		{"cTilingParams", unsafe.Sizeof(cTilingParams{}), 32},
		{"cCacheParams", unsafe.Sizeof(cCacheParams{}), 96},
		{"cHiresParams", unsafe.Sizeof(cHiresParams{}), 56},
		{"cImgGenParams", unsafe.Sizeof(cImgGenParams{}), 480},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s size: got %d, want %d", c.name, c.got, c.want)
		}
	}
}

// TestImgGenParamsInitDefaults asserts that ImgGenParamsInit returns the
// values populated by sd_img_gen_params_init in the C library.
func TestImgGenParamsInitDefaults(t *testing.T) {
	testSetup(t)

	p := ImgGenParamsInit()

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"ClipSkip", p.ClipSkip, int32(-1)},
		{"Width", p.Width, int32(512)},
		{"Height", p.Height, int32(512)},
		{"Steps", p.Steps, int32(20)},
		{"CFGScale", p.CFGScale, float32(7.0)},
		{"Sampler", p.Sampler, SampleMethodCount},
		{"Scheduler", p.Scheduler, SchedulerCount},
		{"Seed", p.Seed, int64(-1)},
		{"BatchCount", p.BatchCount, int32(1)},
		{"Strength", p.Strength, float32(0.75)},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	// sd_sample_params_init sets eta and flow_shift to INFINITY; verify the
	// nested defaults round-trip from C to Go intact.
	raw := defaultCImgGenParams()
	if !math.IsInf(float64(raw.SampleParams.Eta), 1) {
		t.Errorf("SampleParams.Eta: got %v, want +Inf", raw.SampleParams.Eta)
	}
	if !math.IsInf(float64(raw.SampleParams.FlowShift), 1) {
		t.Errorf("SampleParams.FlowShift: got %v, want +Inf", raw.SampleParams.FlowShift)
	}
}

// TestGenerateImageNoContext verifies GenerateImage rejects a nil context.
func TestGenerateImageNoContext(t *testing.T) {
	testSetup(t)

	p := ImgGenParamsInit()
	p.Prompt = "test"
	if _, err := GenerateImage(0, p); err == nil {
		t.Fatal("GenerateImage(0, ...): expected error, got nil")
	}
}
