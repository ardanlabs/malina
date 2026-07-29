// Package model configures and owns reusable stable-diffusion model contexts
// for the Malina SDK.
package model

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/ardanlabs/malina/sdk/malina/sd"
)

var (
	// ErrInvalidRequest identifies invalid generation parameters.
	ErrInvalidRequest = errors.New("invalid generation request")
	// ErrNativeGeneration identifies a failure returned by stable-diffusion.
	ErrNativeGeneration = errors.New("native generation failed")
)

// Config controls model loading and request admission.
type Config struct {
	ModelPath          string
	DiffusionModelPath string
	VAEPath            string
	LLMPath            string
	VAEEncoder         bool
	QueueDepth         int
	AdmissionTimeout   time.Duration
	CPUThreads         int
}

// Option modifies Config.
type Option func(*Config)

// WithDefaults applies SDK defaults.
func WithDefaults(cfg *Config) { cfg.QueueDepth = 2; cfg.AdmissionTimeout = 3 * time.Minute }

// WithModelPath sets the model checkpoint path.
func WithModelPath(path string) Option { return func(cfg *Config) { cfg.ModelPath = path } }

// WithDiffusionModelPath sets a multi-file diffusion model path.
func WithDiffusionModelPath(path string) Option {
	return func(cfg *Config) { cfg.DiffusionModelPath = path }
}

// WithVAEPath sets a multi-file VAE path.
func WithVAEPath(path string) Option { return func(cfg *Config) { cfg.VAEPath = path } }

// WithLLMPath sets a multi-file LLM text encoder path.
func WithLLMPath(path string) Option { return func(cfg *Config) { cfg.LLMPath = path } }

// WithVAEEncoder configures the model to load the VAE encoder for img2img.
func WithVAEEncoder() Option { return func(cfg *Config) { cfg.VAEEncoder = true } }

// WithQueueDepth sets total admitted capacity, including the running call.
func WithQueueDepth(depth int) Option { return func(cfg *Config) { cfg.QueueDepth = depth } }

// WithAdmissionTimeout sets the maximum admission wait.
func WithAdmissionTimeout(timeout time.Duration) Option {
	return func(cfg *Config) { cfg.AdmissionTimeout = timeout }
}

// WithCPUThreads sets native CPU worker threads.
func WithCPUThreads(threads int) Option { return func(cfg *Config) { cfg.CPUThreads = threads } }

// NewConfig constructs and validates Config.
func NewConfig(opts ...Option) (Config, error) {
	var cfg Config
	WithDefaults(&cfg)
	for _, opt := range opts {
		opt(&cfg)
	}
	single := strings.TrimSpace(cfg.ModelPath) != ""
	multiCount := 0
	for _, path := range []string{cfg.DiffusionModelPath, cfg.VAEPath, cfg.LLMPath} {
		if strings.TrimSpace(path) != "" {
			multiCount++
		}
	}
	if single && multiCount > 0 {
		return Config{}, errors.New("model configuration cannot combine a checkpoint with multi-file paths")
	}
	if !single && multiCount != 3 {
		return Config{}, errors.New("model configuration requires a checkpoint or all diffusion, VAE, and LLM paths")
	}
	if cfg.QueueDepth < 1 {
		return Config{}, errors.New("queue depth must be positive")
	}
	if cfg.AdmissionTimeout <= 0 {
		return Config{}, errors.New("admission timeout must be positive")
	}
	if cfg.CPUThreads < 0 {
		return Config{}, errors.New("CPU threads cannot be negative")
	}
	return cfg, nil
}

// GenerateParams controls one text-to-image or image-to-image generation.
type GenerateParams struct {
	Prompt, NegativePrompt string
	Width, Height, Steps   int
	CFGScale               float32
	Seed                   int64
	InitImage              image.Image
	Strength               float32
}

// NewGenerateParams returns generation defaults from stable-diffusion.
func NewGenerateParams() GenerateParams {
	p := sd.ImgGenParamsInit()
	return GenerateParams{Width: int(p.Width), Height: int(p.Height), Steps: int(p.Steps), CFGScale: p.CFGScale, Seed: p.Seed, Strength: p.Strength}
}

// Validate checks whether parameters describe a supported image-generation request.
func (p GenerateParams) Validate() error {
	if p.Prompt == "" {
		return errors.Join(ErrInvalidRequest, errors.New("prompt is required"))
	}
	if strings.IndexByte(p.Prompt, 0) >= 0 || strings.IndexByte(p.NegativePrompt, 0) >= 0 {
		return errors.Join(ErrInvalidRequest, errors.New("prompts cannot contain NUL bytes"))
	}
	if p.Width < 64 || p.Width > 1024 || p.Height < 64 || p.Height > 1024 || p.Width%8 != 0 || p.Height%8 != 0 || p.Width*p.Height > 1024*1024 {
		return errors.Join(ErrInvalidRequest, errors.New("dimensions must be multiples of 8 between 64 and 1024 and at most 1048576 pixels"))
	}
	if p.Steps < 1 || p.Steps > 1000 {
		return errors.Join(ErrInvalidRequest, errors.New("steps must be between 1 and 1000"))
	}
	if p.CFGScale <= 0 || math.IsNaN(float64(p.CFGScale)) || math.IsInf(float64(p.CFGScale), 0) {
		return errors.Join(ErrInvalidRequest, errors.New("CFG scale must be positive and finite"))
	}
	if p.InitImage != nil {
		bounds := p.InitImage.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			return errors.Join(ErrInvalidRequest, errors.New("init image dimensions must be positive"))
		}
		if p.Strength <= 0 || p.Strength > 1 || math.IsNaN(float64(p.Strength)) || math.IsInf(float64(p.Strength), 0) {
			return errors.Join(ErrInvalidRequest, errors.New("img2img strength must be finite and in (0,1]"))
		}
	}
	return nil
}

// GeneratedImage contains an owned PNG and generation metadata.
type GeneratedImage struct {
	PNG           []byte
	Width, Height int
	Seed          int64
}

// ModelInfo describes a loaded model.
type ModelInfo struct {
	ModelPath  string
	CPUThreads int
}

// Model owns exactly one native context.
type Model struct {
	mu       sync.Mutex
	config   Config
	ctx      sd.Context
	unloaded bool
}

// NewModel loads one reusable native model context.
func NewModel(ctx context.Context, cfg Config) (*Model, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	p := sd.ContextParamsInit()
	p.ModelPath = cfg.ModelPath
	p.DiffusionModelPath = cfg.DiffusionModelPath
	p.VAEPath = cfg.VAEPath
	p.LLMPath = cfg.LLMPath
	p.VAEDecodeOnly = !cfg.VAEEncoder
	if cfg.CPUThreads > 0 {
		p.NThreads = int32(cfg.CPUThreads)
	}
	p.FreeParamsImmediately = false
	handle, err := sd.NewContext(p)
	if err != nil {
		return nil, err
	}
	m := Model{config: cfg, ctx: handle}
	return &m, nil
}

// Generate runs synchronous image generation.
func (m *Model) Generate(params GenerateParams) (GeneratedImage, error) {
	if err := params.Validate(); err != nil {
		return GeneratedImage{}, err
	}
	if params.InitImage != nil && !m.config.VAEEncoder {
		return GeneratedImage{}, errors.Join(ErrInvalidRequest, errors.New("img2img requires the VAE encoder option"))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unloaded {
		return GeneratedImage{}, errors.New("model is unloaded")
	}
	p := sd.ImgGenParamsInit()
	p.Prompt = params.Prompt
	p.NegativePrompt = params.NegativePrompt
	p.Width = int32(params.Width)
	p.Height = int32(params.Height)
	p.Steps = int32(params.Steps)
	p.CFGScale = params.CFGScale
	p.Seed = params.Seed
	p.BatchCount = 1
	p.Strength = params.Strength
	if params.InitImage != nil {
		p.InitImage = imageToRGB(params.InitImage)
	}
	raw, err := sd.GenerateImage(m.ctx, p)
	if err != nil {
		return GeneratedImage{}, errors.Join(ErrNativeGeneration, err)
	}
	return encodeImage(raw, params.Seed)
}

func validateConfig(cfg Config) error {
	_, err := NewConfig(
		WithModelPath(cfg.ModelPath), WithDiffusionModelPath(cfg.DiffusionModelPath),
		WithVAEPath(cfg.VAEPath), WithLLMPath(cfg.LLMPath),
		WithQueueDepth(cfg.QueueDepth), WithAdmissionTimeout(cfg.AdmissionTimeout), WithCPUThreads(cfg.CPUThreads),
	)
	return err
}

func imageToRGB(src image.Image) *sd.SDImage {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	raw := &sd.SDImage{Width: uint32(w), Height: uint32(h), Channel: 3, Data: make([]byte, w*h*3)}
	for y := range h {
		for x := range w {
			r, g, b, _ := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			offset := (y*w + x) * 3
			raw.Data[offset] = byte(r >> 8)
			raw.Data[offset+1] = byte(g >> 8)
			raw.Data[offset+2] = byte(b >> 8)
		}
	}
	return raw
}

func encodeImage(raw *sd.SDImage, seed int64) (GeneratedImage, error) {
	if raw == nil || raw.Channel != 3 || len(raw.Data) != int(raw.Width*raw.Height*3) {
		return GeneratedImage{}, errors.New("encoding PNG: invalid RGB image")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, int(raw.Width), int(raw.Height)))
	for i, j := 0, 0; i < len(raw.Data); i, j = i+3, j+4 {
		copy(rgba.Pix[j:j+3], raw.Data[i:i+3])
		rgba.Pix[j+3] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return GeneratedImage{}, fmt.Errorf("encoding PNG: %w", err)
	}
	return GeneratedImage{PNG: buf.Bytes(), Width: int(raw.Width), Height: int(raw.Height), Seed: seed}, nil
}

// Unload releases the native context exactly once.
func (m *Model) Unload() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unloaded {
		return nil
	}
	sd.FreeContext(m.ctx)
	m.ctx = 0
	m.unloaded = true
	return nil
}

// Config returns immutable model configuration.
func (m *Model) Config() Config { return m.config }

// Info returns descriptive model information.
func (m *Model) Info() ModelInfo {
	return ModelInfo{ModelPath: m.config.ModelPath, CPUThreads: m.config.CPUThreads}
}
