package model

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
	"time"

	"github.com/ardanlabs/malina/sdk/malina/sd"
)

func TestNewConfig(t *testing.T) {
	cfg, err := NewConfig(WithModelPath("model.gguf"))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if cfg.QueueDepth != 2 || cfg.AdmissionTimeout != 3*time.Minute {
		t.Fatalf("defaults: got %d/%v, want 2/%v", cfg.QueueDepth, cfg.AdmissionTimeout, 3*time.Minute)
	}
	if _, err := NewConfig(); err == nil {
		t.Fatal("NewConfig without path: got nil error, want error")
	}
}

func TestNewConfigModelPaths(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{name: "single", opts: []Option{WithModelPath("model")}},
		{name: "multi", opts: []Option{WithDiffusionModelPath("diffusion"), WithVAEPath("vae"), WithLLMPath("llm")}},
		{name: "missing multi component", opts: []Option{WithDiffusionModelPath("diffusion"), WithVAEPath("vae")}, wantErr: true},
		{name: "ambiguous", opts: []Option{WithModelPath("model"), WithDiffusionModelPath("diffusion"), WithVAEPath("vae"), WithLLMPath("llm")}, wantErr: true},
		{name: "blank checkpoint", opts: []Option{WithModelPath(" \t")}, wantErr: true},
		{name: "blank multi component", opts: []Option{WithDiffusionModelPath("diffusion"), WithVAEPath(" "), WithLLMPath("llm")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConfig(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig: got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestInitImageValidationAndConversion(t *testing.T) {
	src := image.NewRGBA(image.Rect(3, 4, 5, 5))
	src.Set(3, 4, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	p := GenerateParams{Prompt: "cat", Width: 64, Height: 64, Steps: 1, CFGScale: 1, InitImage: src, Strength: 0.5}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	raw := imageToRGB(src)
	if raw.Width != 2 || raw.Height != 1 || len(raw.Data) != 6 || raw.Data[0] != 10 {
		t.Fatalf("imageToRGB: got %+v", raw)
	}

	invalid := []GenerateParams{p, p, p, p, p}
	invalid[0].InitImage = image.NewRGBA(image.Rect(0, 0, 0, 1))
	invalid[1].Strength = 0
	invalid[2].Strength = 1.1
	invalid[3].Strength = float32(math.NaN())
	invalid[4].Strength = float32(math.Inf(1))
	for i, params := range invalid {
		if err := params.Validate(); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Validate case %d: got %v, want ErrInvalidRequest", i, err)
		}
	}

	m := Model{config: Config{VAEEncoder: false}}
	if _, err := m.Generate(p); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Generate without VAE encoder: got %v, want ErrInvalidRequest", err)
	}
}

func TestGenerateParamsValidate(t *testing.T) {
	valid := GenerateParams{Prompt: "cat", Width: 64, Height: 64, Steps: 1, CFGScale: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate valid params: %v", err)
	}
	valid.Width = 63
	if err := valid.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate: got %v, want ErrInvalidRequest", err)
	}
	valid.Width = 64
	valid.Prompt = "cat\x00dog"
	if err := valid.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate NUL: got %v, want ErrInvalidRequest", err)
	}
	valid.Prompt = "cat"
	for _, scale := range []float32{0, float32(math.NaN()), float32(math.Inf(1))} {
		valid.CFGScale = scale
		if err := valid.Validate(); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("Validate CFG scale %v: got %v, want ErrInvalidRequest", scale, err)
		}
	}
}

func TestEncodeImage(t *testing.T) {
	raw := sd.SDImage{Width: 2, Height: 1, Channel: 3, Data: []byte{255, 0, 0, 0, 255, 0}}
	got, err := encodeImage(&raw, 42)
	if err != nil {
		t.Fatalf("encodeImage: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(got.PNG)); err != nil {
		t.Fatalf("decoding generated PNG: %v", err)
	}
	if got.Width != 2 || got.Height != 1 || got.Seed != 42 {
		t.Fatalf("metadata: got %+v", got)
	}
}
