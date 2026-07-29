// Context tests lock down the stable-diffusion C ABI mirror.
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
	const expectedSize = 224
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
		{"VAEDecodeOnly", p.VAEDecodeOnly, true},
		{"FreeParamsImmediately", p.FreeParamsImmediately, true},
		{"NThreads", p.NThreads, cores},
		{"Wtype", p.Wtype, SDTypeCount},
		{"RngType", p.RngType, RngCuda},
		{"SamplerRngType", p.SamplerRngType, RngTypeCount},
		{"Prediction", p.Prediction, PredictionCount},
		{"LoraApplyMode", p.LoraApplyMode, LoraApplyAuto},
		{"OffloadParamsToCPU", p.OffloadParamsToCPU, false},
		{"MaxVram", p.MaxVram, float32(0)},
		{"EnableMmap", p.EnableMmap, false},
		{"KeepClipOnCPU", p.KeepClipOnCPU, false},
		{"KeepControlNetOnCPU", p.KeepControlNetOnCPU, false},
		{"KeepVAEOnCPU", p.KeepVAEOnCPU, false},
		{"FlashAttn", p.FlashAttn, false},
		{"DiffusionFlashAttn", p.DiffusionFlashAttn, false},
		{"TaePreviewOnly", p.TaePreviewOnly, false},
		{"DiffusionConvDirect", p.DiffusionConvDirect, false},
		{"VAEConvDirect", p.VAEConvDirect, false},
		{"CircularX", p.CircularX, false},
		{"CircularY", p.CircularY, false},
		{"ForceSDXLVAEConvScale", p.ForceSDXLVAEConvScale, false},
		{"ChromaUseDitMask", p.ChromaUseDitMask, true},
		{"ChromaUseT5Mask", p.ChromaUseT5Mask, false},
		{"ChromaT5MaskPad", p.ChromaT5MaskPad, int32(1)},
		{"QwenImageZeroCondT", p.QwenImageZeroCondT, false},
		{"VaeFormat", p.VaeFormat, SDVaeFormatAuto},
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
