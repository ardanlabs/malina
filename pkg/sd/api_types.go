package sd

import "errors"

// ErrUnsupportedAPI reports that the loaded stable-diffusion.cpp library
// predates an optional API used by the caller.
var ErrUnsupportedAPI = errors.New("stable-diffusion.cpp API is not available")

// PreviewMode mirrors enum preview_t.
type PreviewMode int32

const (
	PreviewNone  PreviewMode = 0
	PreviewProj  PreviewMode = 1
	PreviewTAE   PreviewMode = 2
	PreviewVAE   PreviewMode = 3
	PreviewCount PreviewMode = 4
)

// CacheMode mirrors enum sd_cache_mode_t.
type CacheMode int32

const (
	CacheDisabled   CacheMode = 0
	CacheEasyCache  CacheMode = 1
	CacheUCache     CacheMode = 2
	CacheDBCache    CacheMode = 3
	CacheTaylorseer CacheMode = 4
	CacheDiT        CacheMode = 5
	CacheSpectrum   CacheMode = 6
)

// HiresUpscaler mirrors enum sd_hires_upscaler_t.
type HiresUpscaler int32

const (
	HiresUpscalerNone                     HiresUpscaler = 0
	HiresUpscalerLatent                   HiresUpscaler = 1
	HiresUpscalerLatentNearest            HiresUpscaler = 2
	HiresUpscalerLatentNearestExact       HiresUpscaler = 3
	HiresUpscalerLatentAntialiased        HiresUpscaler = 4
	HiresUpscalerLatentBicubic            HiresUpscaler = 5
	HiresUpscalerLatentBicubicAntialiased HiresUpscaler = 6
	HiresUpscalerLanczos                  HiresUpscaler = 7
	HiresUpscalerNearest                  HiresUpscaler = 8
	HiresUpscalerModel                    HiresUpscaler = 9
	HiresUpscalerCount                    HiresUpscaler = 10
)

// CancelMode mirrors enum sd_cancel_mode_t.
type CancelMode int32

const (
	CancelAll        CancelMode = 0
	CancelNewLatents CancelMode = 1
	CancelReset      CancelMode = 2
)

// Embedding identifies one textual-inversion embedding by name and path.
type Embedding struct {
	Name string
	Path string
}

// Lora configures one LoRA adapter for generation.
type Lora struct {
	IsHighNoise bool
	Multiplier  float32
	Path        string
}

// SLGParams configures skip-layer guidance.
type SLGParams struct {
	Layers     []int32
	LayerStart float32
	LayerEnd   float32
	Scale      float32
}

// GuidanceParams configures text, image, distilled, and skip-layer guidance.
type GuidanceParams struct {
	TextCFG           float32
	ImageCFG          float32
	DistilledGuidance float32
	SLG               SLGParams
}

// SampleParams configures one diffusion sampling stage.
type SampleParams struct {
	Guidance        GuidanceParams
	Scheduler       Scheduler
	Method          SampleMethod
	Steps           int32
	Eta             float32
	ShiftedTimestep int32
	CustomSigmas    []float32
	FlowShift       float32
	ExtraArgs       string
}

// PhotoMakerParams configures PhotoMaker identity conditioning.
type PhotoMakerParams struct {
	IDImages      []*SDImage
	IDEmbedPath   string
	StyleStrength float32
}

// PuLIDParams configures PuLID identity conditioning.
type PuLIDParams struct {
	IDEmbeddingPath string
	IDWeight        float32
}

// TilingParams configures VAE spatial and temporal tiling.
type TilingParams struct {
	Enabled        bool
	TemporalTiling bool
	TileSizeX      int32
	TileSizeY      int32
	TargetOverlap  float32
	RelativeSizeX  float32
	RelativeSizeY  float32
	ExtraArgs      string
}

// CacheParams configures diffusion-transformer inference caching.
type CacheParams struct {
	Mode                     CacheMode
	ReuseThreshold           float32
	StartPercent             float32
	EndPercent               float32
	ErrorDecayRate           float32
	UseRelativeThreshold     bool
	ResetErrorOnCompute      bool
	FnComputeBlocks          int32
	BnComputeBlocks          int32
	ResidualDiffThreshold    float32
	MaxWarmupSteps           int32
	MaxCachedSteps           int32
	MaxContinuousCachedSteps int32
	TaylorseerNDerivatives   int32
	TaylorseerSkipInterval   int32
	SCMMask                  string
	SCMPolicyDynamic         bool
	SpectrumW                float32
	SpectrumM                int32
	SpectrumLambda           float32
	SpectrumWindowSize       int32
	SpectrumFlexWindow       float32
	SpectrumWarmupSteps      int32
	SpectrumStopPercent      float32
}

// HiresParams configures the high-resolution refinement pass.
type HiresParams struct {
	Enabled           bool
	Upscaler          HiresUpscaler
	ModelPath         string
	Scale             float32
	TargetWidth       int32
	TargetHeight      int32
	Steps             int32
	DenoisingStrength float32
	UpscaleTileSize   int32
	CustomSigmas      []float32
}

// Audio is interleaved, frame-major PCM audio. Data contains
// SampleCount()*Channels float samples.
type Audio struct {
	SampleRate uint32
	Channels   uint32
	Data       []float32
}

// SampleCount returns the number of sample frames per channel.
func (a Audio) SampleCount() uint64 {
	if a.Channels == 0 {
		return 0
	}
	return uint64(len(a.Data)) / uint64(a.Channels)
}

// RefVideo supplies reference frames, playback metadata, and optional audio.
type RefVideo struct {
	Frames []*SDImage
	FPS    int32
	Audio  *Audio
}

// Device identifies one ggml backend device accepted by ContextParams.
type Device struct {
	Name        string
	Description string
}

// ADetailerParams configures an ADetailer refinement pass.
type ADetailerParams struct {
	Prompt         string
	NegativePrompt string
	ExtraArgs      string
}

// ConvertParams configures conversion of a single checkpoint.
type ConvertParams struct {
	InputPath       string
	VAEPath         string
	OutputPath      string
	OutputType      SDType
	TensorTypeRules string
	ConvertName     bool
}

// ComponentConvertParams configures conversion from separate model components.
type ComponentConvertParams struct {
	ModelPath          string
	ClipLPath          string
	ClipGPath          string
	T5XXLPath          string
	DiffusionModelPath string
	VAEPath            string
	OutputPath         string
	OutputType         SDType
	TensorTypeRules    string
	ConvertName        bool
	NThreads           int32
}

// CannyParams configures in-place Canny edge preprocessing.
type CannyParams struct {
	HighThreshold float32
	LowThreshold  float32
	Weak          float32
	Strong        float32
	Inverse       bool
}
