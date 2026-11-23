package sd

import (
	"errors"
	"math"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// cLora mirrors the live sd_lora_t descriptor.
type cLora struct {
	IsHighNoise uint8
	_           [3]byte
	Multiplier  float32
	Path        uintptr
}

// ImgGenParams configures image generation. Start with ImgGenParamsInit.
// Fields equal to the helper defaults remain unset; use Options to explicitly
// supply default-looking values or unset a key. Positive infinity in ImageCFG,
// Eta, FlowShift and Cache.ReuseThreshold is a Go-only automatic sentinel, never
// passed to the C setters. Other nonfinite values are errors.
type ImgGenParams struct {
	Loras             []Lora
	Prompt            string
	NegativePrompt    string
	ClipSkip          int32
	Width             int32
	Height            int32
	Steps             int32        `sd:"sample.steps"`
	CFGScale          float32      `sd:"sample.guidance.txt_cfg"`
	Sampler           SampleMethod `sd:"sample.method"`
	Scheduler         Scheduler    `sd:"sample.scheduler"`
	Seed              int64
	BatchCount        int32
	CircularX         bool
	CircularY         bool
	Strength          float32
	InitImage         SDImage
	RefImages         []SDImage
	MaskImage         SDImage
	ImageCFG          float32   `sd:"sample.guidance.img_cfg"`
	DistilledGuidance float32   `sd:"sample.guidance.distilled_guidance"`
	Eta               float32   `sd:"sample.eta"`
	CustomSigmas      []float32 `sd:"sample.sigmas"`
	FlowShift         float32   `sd:"sample.flow_shift"`
	ExtraSampleArgs   string    `sd:"sample.args"`
	ControlImage      SDImage
	ControlStrength   float32
	VAETiling         TilingParams `sd:"vae_tiling"`
	Cache             CacheParams
	Hires             HiresParams
	QwenImageLayers   int32
	ControlType       ControlType
	Options           Options
}

var generateImageFunc ffi.Fun

func loadGenFuncs(lib ffi.Lib) error {
	var err error
	generateImageFunc, err = lib.Prep("sd_generate_image", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
	if err != nil {
		return loadError("sd_generate_image", err)
	}
	return loadResultFuncs(lib)
}

// ImgGenParamsInit returns compatibility defaults; native defaults are selected
// by leaving unchanged fields unset.
func ImgGenParamsInit() ImgGenParams {
	s := SampleParamsInit()
	return ImgGenParams{ClipSkip: -1, Width: 512, Height: 512, Steps: s.Steps, CFGScale: s.Guidance.TextCFG, Sampler: s.Method, Scheduler: s.Scheduler, Seed: -1, BatchCount: 1, Strength: .75, ImageCFG: s.Guidance.ImageCFG, DistilledGuidance: s.Guidance.DistilledGuidance, Eta: s.Eta, FlowShift: s.FlowShift, ControlStrength: .9, VAETiling: TilingParams{TargetOverlap: .5}, Cache: CacheParamsInit(), Hires: HiresParamsInit(), QwenImageLayers: 3}
}

// SampleParamsInit returns compatibility sampling defaults. Positive infinity
// represents native automatic selection for eta, flow shift and image CFG.
func SampleParamsInit() SampleParams {
	return SampleParams{Guidance: GuidanceParams{TextCFG: 7, ImageCFG: float32(math.Inf(1)), DistilledGuidance: 3.5}, Scheduler: SchedulerCount, Method: SampleMethodCount, Steps: 20, Eta: float32(math.Inf(1)), FlowShift: float32(math.Inf(1))}
}

// CacheParamsInit returns compatibility cache defaults.
func CacheParamsInit() CacheParams {
	return CacheParams{ReuseThreshold: float32(math.Inf(1)), StartPercent: .15, EndPercent: .95, ErrorDecayRate: 1, UseRelativeThreshold: true, ResetErrorOnCompute: true, FnComputeBlocks: 8, ResidualDiffThreshold: .08, MaxWarmupSteps: 8, MaxCachedSteps: -1, MaxContinuousCachedSteps: -1, TaylorseerNDerivatives: 1, TaylorseerSkipInterval: 1, SCMPolicyDynamic: true, SpectrumW: .4, SpectrumM: 3, SpectrumLambda: 1, SpectrumWindowSize: 2, SpectrumFlexWindow: .5, SpectrumWarmupSteps: 4, SpectrumStopPercent: .9}
}

// HiresParamsInit returns compatibility high-resolution defaults.
func HiresParamsInit() HiresParams {
	return HiresParams{Upscaler: HiresUpscalerLatent, Scale: 2, DenoisingStrength: .7, UpscaleTileSize: 128}
}

// GenerateImage returns the first image in the generated batch. Use
// GenerateImages to consume all outputs. Pixels and metadata are Go-owned.
func GenerateImage(ctx Context, params ImgGenParams) (SDImage, error) {
	images, err := GenerateImages(ctx, params)
	if err != nil {
		return SDImage{}, err
	}
	if len(images) == 0 {
		return SDImage{}, errors.New("generate image: empty native result")
	}
	return images[0], nil
}

// GenerateImages returns every image and its native generation metadata.
func GenerateImages(ctx Context, params ImgGenParams) ([]SDImage, error) {
	if ctx == 0 {
		return nil, errors.New("GenerateImages: nil context")
	}
	o, err := newOptions("img_gen")
	if err != nil {
		return nil, err
	}
	defer o.free()
	if err := o.fields(params, ImgGenParamsInit(), ""); err != nil {
		return nil, err
	}
	return callImageArray(generateImageFunc, unsafe.Pointer(&ctx), unsafe.Pointer(&o.ptr))
}
