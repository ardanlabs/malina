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
		{"cPulidParams", unsafe.Sizeof(cPulidParams{}), 16},
		{"cTilingParams", unsafe.Sizeof(cTilingParams{}), 32},
		{"cCacheParams", unsafe.Sizeof(cCacheParams{}), 96},
		{"cHiresParams", unsafe.Sizeof(cHiresParams{}), 56},
		{"cImgGenParams", unsafe.Sizeof(cImgGenParams{}), 544},
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
		{"CircularX", p.CircularX, false},
		{"CircularY", p.CircularY, false},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	// Nested struct defaults round-trip from C to Go. Asserting non-trivial
	// values across every nested struct (Guidance, SLG, PMParams, VAETiling,
	// Cache, Hires) is the malina-side analogue of bucky's by-ref/by-value
	// cross-check in TestWhisperFullParamsSize: it locks down the field
	// positions inside each padded struct, so if any explicit _padN block
	// gets out of sync with the C ABI a representative field value here
	// will drift to whatever happens to live at that offset.
	raw := defaultCImgGenParams()

	nested := []struct {
		name string
		got  any
		want any
	}{
		// SampleParams.Guidance
		{"Guidance.TxtCfg", raw.SampleParams.Guidance.TxtCfg, float32(7.0)},
		{"Guidance.DistilledGuidance", raw.SampleParams.Guidance.DistilledGuidance, float32(3.5)},

		// SampleParams.Guidance.SLG (deeply nested struct-in-struct)
		{"SLG.LayerStart", raw.SampleParams.Guidance.SLG.LayerStart, float32(0.01)},
		{"SLG.LayerEnd", raw.SampleParams.Guidance.SLG.LayerEnd, float32(0.2)},

		// Top-level control + photo-maker defaults
		{"ControlStrength", raw.ControlStrength, float32(0.9)},
		{"IPAdapterStrength", raw.IPAdapterStrength, float32(1)},
		{"PMParams.StyleStrength", raw.PMParams.StyleStrength, float32(20)},
		{"PulidParams.IDWeight", raw.PulidParams.IDWeight, float32(1)},
		{"QwenImageLayers", raw.QwenImageLayers, int32(3)},

		// VAETilingParams
		{"VAETilingParams.TargetOverlap", raw.VAETilingParams.TargetOverlap, float32(0.5)},

		// Cache (largest nested struct, most likely to catch padding drift)
		{"Cache.StartPercent", raw.Cache.StartPercent, float32(0.15)},
		{"Cache.EndPercent", raw.Cache.EndPercent, float32(0.95)},
		{"Cache.ErrorDecayRate", raw.Cache.ErrorDecayRate, float32(1)},
		{"Cache.UseRelativeThreshold", raw.Cache.UseRelativeThreshold, uint8(1)},
		{"Cache.ResetErrorOnCompute", raw.Cache.ResetErrorOnCompute, uint8(1)},
		{"Cache.FnComputeBlocks", raw.Cache.FnComputeBlocks, int32(8)},
		{"Cache.ResidualDiffThreshold", raw.Cache.ResidualDiffThreshold, float32(0.08)},
		{"Cache.MaxWarmupSteps", raw.Cache.MaxWarmupSteps, int32(8)},
		{"Cache.MaxCachedSteps", raw.Cache.MaxCachedSteps, int32(-1)},
		{"Cache.SCMPolicyDynamic", raw.Cache.SCMPolicyDynamic, uint8(1)},
		{"Cache.SpectrumStopPercent", raw.Cache.SpectrumStopPercent, float32(0.9)},

		// Hires
		{"Hires.Upscaler", raw.Hires.Upscaler, int32(1)},
		{"Hires.Scale", raw.Hires.Scale, float32(2)},
		{"Hires.DenoisingStrength", raw.Hires.DenoisingStrength, float32(0.7)},
		{"Hires.UpscaleTileSize", raw.Hires.UpscaleTileSize, int32(128)},
	}
	for _, c := range nested {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	// sd_sample_params_init sets eta, flow_shift, img_cfg, and
	// cache.reuse_threshold to INFINITY; verify those survive intact.
	infs := []struct {
		name string
		got  float32
	}{
		{"SampleParams.Eta", raw.SampleParams.Eta},
		{"SampleParams.FlowShift", raw.SampleParams.FlowShift},
		{"Guidance.ImgCfg", raw.SampleParams.Guidance.ImgCfg},
		{"Cache.ReuseThreshold", raw.Cache.ReuseThreshold},
	}
	for _, c := range infs {
		if !math.IsInf(float64(c.got), 1) {
			t.Errorf("%s: got %v, want +Inf", c.name, c.got)
		}
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
