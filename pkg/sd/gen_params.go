package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// =============================================================================
// C-mirror structs (ABI-exact, used only across the FFI boundary)

// cSlgParams mirrors sd_slg_params_t.
// Size: 32 bytes (16+12, padded to 32 by 8-aligned final field).
type cSlgParams struct {
	Layers     uintptr // 0..8   (int*)
	LayerCount uint64  // 8..16  (size_t)
	LayerStart float32 // 16..20
	LayerEnd   float32 // 20..24
	Scale      float32 // 24..28
	_          [4]byte // 28..32 (trailing pad to size 32)
}

// cGuidanceParams mirrors sd_guidance_params_t.
// Size: 48 bytes.
type cGuidanceParams struct {
	TxtCfg            float32 // 0..4
	ImgCfg            float32 // 4..8
	DistilledGuidance float32 // 8..12
	_                 [4]byte // 12..16 (pad before SLG, which is 8-aligned)
	SLG               cSlgParams
}

// cSampleParams mirrors sd_sample_params_t.
// Size: 96 bytes.
type cSampleParams struct {
	Guidance          cGuidanceParams // 0..48
	Scheduler         int32           // 48..52
	SampleMethod      int32           // 52..56
	SampleSteps       int32           // 56..60
	Eta               float32         // 60..64
	ShiftedTimestep   int32           // 64..68
	_                 [4]byte         // 68..72 (pad before pointer)
	CustomSigmas      uintptr         // 72..80
	CustomSigmasCount int32           // 80..84
	FlowShift         float32         // 84..88
	ExtraSampleArgs   uintptr         // 88..96
}

// cPMParams mirrors sd_pm_params_t. Size: 32 bytes.
type cPMParams struct {
	IDImages      uintptr // 0..8
	IDImagesCount int32   // 8..12
	_             [4]byte // 12..16
	IDEmbedPath   uintptr // 16..24
	StyleStrength float32 // 24..28
	_             [4]byte // 28..32
}

// cPulidParams mirrors sd_pulid_params_t. Size: 16 bytes.
type cPulidParams struct {
	IDEmbeddingPath uintptr // 0..8
	IDWeight        float32 // 8..12
	_               [4]byte // 12..16
}

// cTilingParams mirrors sd_tiling_params_t. Size: 32 bytes.
type cTilingParams struct {
	Enabled         uint8   // 0
	TemporalTiling  uint8   // 1
	_               [2]byte // 2..4
	TileSizeX       int32   // 4..8
	TileSizeY       int32   // 8..12
	TargetOverlap   float32 // 12..16
	RelSizeX        float32 // 16..20
	RelSizeY        float32 // 20..24
	ExtraTilingArgs uintptr // 24..32
}

// cCacheParams mirrors sd_cache_params_t. Size: 96 bytes.
type cCacheParams struct {
	Mode                     int32   // 0..4
	ReuseThreshold           float32 // 4..8
	StartPercent             float32 // 8..12
	EndPercent               float32 // 12..16
	ErrorDecayRate           float32 // 16..20
	UseRelativeThreshold     uint8   // 20
	ResetErrorOnCompute      uint8   // 21
	_                        [2]byte // 22..24
	FnComputeBlocks          int32   // 24..28
	BnComputeBlocks          int32   // 28..32
	ResidualDiffThreshold    float32 // 32..36
	MaxWarmupSteps           int32   // 36..40
	MaxCachedSteps           int32   // 40..44
	MaxContinuousCachedSteps int32   // 44..48
	TaylorseerNDerivatives   int32   // 48..52
	TaylorseerSkipInterval   int32   // 52..56
	SCMMask                  uintptr // 56..64
	SCMPolicyDynamic         uint8   // 64
	_                        [3]byte // 65..68
	SpectrumW                float32 // 68..72
	SpectrumM                int32   // 72..76
	SpectrumLam              float32 // 76..80
	SpectrumWindowSize       int32   // 80..84
	SpectrumFlexWindow       float32 // 84..88
	SpectrumWarmupSteps      int32   // 88..92
	SpectrumStopPercent      float32 // 92..96
}

// cHiresParams mirrors sd_hires_params_t. Size: 56 bytes.
type cHiresParams struct {
	Enabled           uint8   // 0
	_                 [3]byte // 1..4
	Upscaler          int32   // 4..8
	ModelPath         uintptr // 8..16
	Scale             float32 // 16..20
	TargetWidth       int32   // 20..24
	TargetHeight      int32   // 24..28
	Steps             int32   // 28..32
	DenoisingStrength float32 // 32..36
	UpscaleTileSize   int32   // 36..40
	CustomSigmas      uintptr // 40..48
	CustomSigmasCount int32   // 48..52
	_                 [4]byte // 52..56 (trailing pad to size 56)
}

// cImgGenParams mirrors sd_img_gen_params_t. Size: 544 bytes.
type cImgGenParams struct {
	Loras             uintptr // 0..8
	LoraCount         uint32  // 8..12
	_                 [4]byte // 12..16
	Prompt            uintptr // 16..24
	NegativePrompt    uintptr // 24..32
	ClipSkip          int32   // 32..36
	_                 [4]byte // 36..40
	InitImage         cImage  // 40..64
	RefImages         uintptr // 64..72
	RefImagesCount    int32   // 72..76
	_                 [4]byte // 76..80
	RefImageArgs      uintptr // 80..88
	MaskImage         cImage  // 88..112
	Width             int32   // 112..116
	Height            int32   // 116..120
	SampleParams      cSampleParams
	Strength          float32 // 216..220
	_                 [4]byte // 220..224
	Seed              int64   // 224..232
	BatchCount        int32   // 232..236
	_                 [4]byte // 236..240
	ControlImage      cImage  // 240..264
	ControlStrength   float32 // 264..268
	_                 [4]byte // 268..272
	IPAdapterImage    cImage  // 272..296
	IPAdapterStrength float32 // 296..300
	_                 [4]byte // 300..304
	PMParams          cPMParams
	PulidParams       cPulidParams
	VAETilingParams   cTilingParams
	Cache             cCacheParams
	Hires             cHiresParams
	QwenImageLayers   int32 // 536..540
	CircularX         uint8 // 540
	CircularY         uint8 // 541
	_                 [2]byte
}

// =============================================================================
// Public API

// ImgGenParams is the Go-side representation of sd_img_gen_params_t with the
// commonly-used text-to-image / image-to-image knobs surfaced as top-level
// fields. Advanced subsystems (LoRA, HiRes, Cache, PhotoMaker, ControlNet,
// reference images) are left at C library defaults in v1 and exposed in
// later milestones.
//
// Use ImgGenParamsInit to obtain a value populated with the library defaults,
// then set Prompt and any other fields before passing to GenerateImage.
type ImgGenParams struct {
	Prompt         string
	NegativePrompt string

	ClipSkip int32

	// Width and Height default to 512x512. Must be multiples of 8.
	Width  int32
	Height int32

	// Steps is the number of denoising iterations. Default 20. More steps =
	// slower but higher quality.
	Steps int32

	// CFGScale (txt_cfg) is the prompt classifier-free guidance scale.
	// Default 7.0. Higher = stricter prompt adherence; lower = more freedom.
	CFGScale float32

	// Sampler selects the denoising algorithm. Default sampler is chosen by
	// the C library based on the model family.
	Sampler SampleMethod

	// Scheduler selects the noise schedule. Default scheduler is chosen by
	// the C library based on the model family.
	Scheduler Scheduler

	// Seed seeds the RNG. -1 selects a random seed. Default -1.
	Seed int64

	// BatchCount is the number of images to generate per call. Default 1.
	BatchCount int32

	// CircularX and CircularY enable seamless tiling across the corresponding
	// image axis.
	CircularX bool
	CircularY bool

	// Strength is the noise strength for img2img. Default 0.75. Only takes
	// effect when InitImage is set: lower values preserve more of the
	// source image, 1.0 destroys it entirely.
	Strength float32

	// InitImage switches the generator into image-to-image mode: the image
	// is VAE-encoded as the starting latent and the prompt steers the
	// denoising. When nil (the default), GenerateImage runs text-to-image
	// from random noise. Must be 3-channel RGB; the C library bilinearly
	// resizes the pixels to (Width, Height) automatically.
	//
	InitImage *SDImage
}

var (
	// SD_API void sd_img_gen_params_init(sd_img_gen_params_t* sd_img_gen_params);
	imgGenParamsInitFunc ffi.Fun

	// SD_API bool generate_image(sd_ctx_t*, const sd_img_gen_params_t*, sd_image_t**, int*);
	generateImageFunc ffi.Fun

	// SD_API void free_sd_images(sd_image_t* result_images, int num_images);
	freeSDImagesFunc ffi.Fun
)

func loadGenFuncs(lib ffi.Lib) error {
	var err error

	if imgGenParamsInitFunc, err = lib.Prep("sd_img_gen_params_init", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return loadError("sd_img_gen_params_init", err)
	}

	if generateImageFunc, err = lib.Prep("generate_image", &ffi.TypeUint8, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer); err != nil {
		return loadError("generate_image", err)
	}

	if freeSDImagesFunc, err = lib.Prep("free_sd_images", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypeSint32); err != nil {
		return loadError("free_sd_images", err)
	}

	return nil
}

// ImgGenParamsInit returns an ImgGenParams populated with the defaults from
// sd_img_gen_params_init. Callers must at minimum set Prompt before passing
// to GenerateImage.
func ImgGenParamsInit() ImgGenParams {
	raw := defaultCImgGenParams()
	return ImgGenParams{
		ClipSkip:   raw.ClipSkip,
		Width:      raw.Width,
		Height:     raw.Height,
		Steps:      raw.SampleParams.SampleSteps,
		CFGScale:   raw.SampleParams.Guidance.TxtCfg,
		Sampler:    SampleMethod(raw.SampleParams.SampleMethod),
		Scheduler:  Scheduler(raw.SampleParams.Scheduler),
		Seed:       raw.Seed,
		BatchCount: raw.BatchCount,
		Strength:   raw.Strength,
		CircularX:  raw.CircularX != 0,
		CircularY:  raw.CircularY != 0,
	}
}

func defaultCImgGenParams() cImgGenParams {
	var raw cImgGenParams
	rawPtr := &raw
	imgGenParamsInitFunc.Call(nil, unsafe.Pointer(&rawPtr))
	return raw
}

// GenerateImage runs the diffusion pipeline configured by ctx and params and
// returns a single decoded image. The returned SDImage owns its pixel data
// (copied out of the C heap), so it remains valid after the context is
// freed. The underlying result batch is released by stable-diffusion.cpp's
// free_sd_images before this function returns.
func GenerateImage(ctx Context, params ImgGenParams) (*SDImage, error) {
	if ctx == 0 {
		return nil, errors.New("GenerateImage: nil context")
	}

	// Start from the C library's defaults so every nested struct is filled
	// in correctly, then overlay the user-controllable fields.
	raw := defaultCImgGenParams()
	raw.ClipSkip = params.ClipSkip
	raw.Width = params.Width
	raw.Height = params.Height
	raw.Seed = params.Seed
	raw.BatchCount = params.BatchCount
	raw.Strength = params.Strength
	raw.SampleParams.SampleSteps = params.Steps
	raw.SampleParams.Guidance.TxtCfg = params.CFGScale
	raw.SampleParams.SampleMethod = int32(params.Sampler)
	raw.SampleParams.Scheduler = int32(params.Scheduler)
	raw.CircularX = boolToU8(params.CircularX)
	raw.CircularY = boolToU8(params.CircularY)

	var refs cStringRefs
	p, err := refs.add(params.Prompt)
	if err != nil {
		return nil, err
	}
	raw.Prompt = p

	np, err := refs.add(params.NegativePrompt)
	if err != nil {
		return nil, err
	}
	raw.NegativePrompt = np

	if params.InitImage != nil {
		if err := bindCImage(&raw.InitImage, params.InitImage, "InitImage"); err != nil {
			return nil, err
		}
	}

	clearLastLog()

	rawPtr := &raw
	var resultPtr *cImage
	var resultCount int32
	resultPtrPtr := &resultPtr
	resultCountPtr := &resultCount
	var result ffi.Arg
	generateImageFunc.Call(
		&result,
		unsafe.Pointer(&ctx),
		unsafe.Pointer(&rawPtr),
		unsafe.Pointer(&resultPtrPtr),
		unsafe.Pointer(&resultCountPtr),
	)
	runtime.KeepAlive(refs.keep)
	runtime.KeepAlive(params.InitImage)
	runtime.KeepAlive(&raw)

	if byte(result) == 0 || resultPtr == nil || resultCount == 0 {
		if last := LastError(); last != "" {
			return nil, fmt.Errorf("generate_image failed: %s", last)
		}
		return nil, errors.New("generate_image failed (no log message captured)")
	}

	// Copy the first pixel buffer into a Go-owned slice, then ask the library
	// to release the complete result batch with its own allocator.
	out := sdImageFromC(resultPtr)
	freeSDImagesFunc.Call(nil, unsafe.Pointer(&resultPtr), unsafe.Pointer(&resultCount))

	return out, nil
}
