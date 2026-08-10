package sd

import (
	"testing"
	"unsafe"
)

// TestContextParamsStructSize verifies the Go-side mirror of sd_ctx_params_t
// has the exact byte size the C ABI expects on this platform. Drift here
// means struct fields will misalign and NewContext will produce garbage
// pointers.
func TestContextParamsStructSize(t *testing.T) {
	const expectedSize = 280
	got := unsafe.Sizeof(cContextParams{})
	if got != expectedSize {
		t.Fatalf("cContextParams size: got %d, want %d", got, expectedSize)
	}
}

// TestContextParamsInitDefaults asserts that the Go-side ContextParamsInit
// returns values that match what sd_ctx_params_init populates in the C
// library. Each field below maps to an explicit assignment in
// src/stable-diffusion.cpp:sd_ctx_params_init.
func TestContextParamsInitDefaults(t *testing.T) {
	testSetup(t)

	p := ContextParamsInit()

	cores := NumPhysicalCores()

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"NThreads", p.NThreads, cores},
		{"Wtype", p.Wtype, SDTypeCount},
		{"RngType", p.RngType, RngCuda},
		{"SamplerRngType", p.SamplerRngType, RngTypeCount},
		{"Prediction", p.Prediction, PredictionCount},
		{"LoraApplyMode", p.LoraApplyMode, LoraApplyAuto},
		{"MaxVram", p.MaxVram, ""},
		{"EnableMmap", p.EnableMmap, false},
		{"FlashAttn", p.FlashAttn, false},
		{"DiffusionFlashAttn", p.DiffusionFlashAttn, false},
		{"TaePreviewOnly", p.TaePreviewOnly, false},
		{"DiffusionConvDirect", p.DiffusionConvDirect, false},
		{"VAEConvDirect", p.VAEConvDirect, false},
		{"ForceSDXLVAEConvScale", p.ForceSDXLVAEConvScale, false},
		{"VaeFormat", p.VaeFormat, SDVaeFormatAuto},
		{"StreamLayers", p.StreamLayers, false},
		{"EagerLoad", p.EagerLoad, false},
		{"AutoFit", p.AutoFit, false},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestNewContextEmptyFails verifies NewContext returns an error when no model
// paths are configured. The C library logs and returns NULL in that case.
func TestNewContextEmptyFails(t *testing.T) {
	testSetup(t)

	p := ContextParamsInit()
	ctx, err := NewContext(p)
	defer FreeContext(ctx)
	if err == nil {
		t.Fatal("NewContext with empty params: expected error, got nil")
	}
	if ctx != 0 {
		t.Errorf("NewContext with empty params: expected zero handle, got %d", ctx)
	}
}
