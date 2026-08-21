package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

// =============================================================================
// C-mirror structs (ABI-exact, used only across the FFI boundary)

// cSlgParams mirrors sd_slg_params_t.
// Size: 32 bytes (16+12, padded to 32 by 8-aligned final field).
type cSlgParams struct {
	Layers     *int32  // 0..8   (int*)
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
	CustomSigmas      *float32        // 72..80
	CustomSigmasCount int32           // 80..84
	FlowShift         float32         // 84..88
	ExtraSampleArgs   *byte           // 88..96
}

// cPMParams mirrors sd_pm_params_t. Size: 32 bytes.
type cPMParams struct {
	IDImages      uintptr // 0..8
	IDImagesCount int32   // 8..12
	_             [4]byte // 12..16
	IDEmbedPath   *byte   // 16..24
	StyleStrength float32 // 24..28
	_             [4]byte // 28..32
}

// cPulidParams mirrors sd_pulid_params_t. Size: 16 bytes.
type cPulidParams struct {
	IDEmbeddingPath *byte   // 0..8
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
	ExtraTilingArgs *byte   // 24..32
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
	SCMMask                  *byte   // 56..64
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
	Enabled           uint8    // 0
	_                 [3]byte  // 1..4
	Upscaler          int32    // 4..8
	ModelPath         *byte    // 8..16
	Scale             float32  // 16..20
	TargetWidth       int32    // 20..24
	TargetHeight      int32    // 24..28
	Steps             int32    // 28..32
	DenoisingStrength float32  // 32..36
	UpscaleTileSize   int32    // 36..40
	CustomSigmas      *float32 // 40..48
	CustomSigmasCount int32    // 48..52
	_                 [4]byte  // 52..56 (trailing pad to size 56)
}

// cLora mirrors sd_lora_t. Size: 16 bytes.
type cLora struct {
	IsHighNoise uint8
	_           [3]byte
	Multiplier  float32
	Path        uintptr
}

// cImgGenParams mirrors sd_img_gen_params_t. Size: 544 bytes.
type cImgGenParams struct {
	Loras             uintptr // 0..8
	LoraCount         uint32  // 8..12
	_                 [4]byte // 12..16
	Prompt            *byte   // 16..24
	NegativePrompt    *byte   // 24..32
	ClipSkip          int32   // 32..36
	_                 [4]byte // 36..40
	InitImage         cImage  // 40..64
	RefImages         uintptr // 64..72
	RefImagesCount    int32   // 72..76
	_                 [4]byte // 76..80
	RefImageArgs      *byte   // 80..88
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

// ImgGenParams is the Go-side representation of sd_img_gen_params_t.
//
// Use ImgGenParamsInit to obtain a value populated with the library defaults,
// then set Prompt and any other fields before passing to GenerateImage.
type ImgGenParams struct {
	Loras          []Lora
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
	InitImage    *SDImage
	RefImages    []*SDImage
	RefImageArgs string
	MaskImage    *SDImage

	ImageCFG          float32
	DistilledGuidance float32
	SLG               SLGParams
	Eta               float32
	ShiftedTimestep   int32
	CustomSigmas      []float32
	FlowShift         float32
	ExtraSampleArgs   string

	ControlImage      *SDImage
	ControlStrength   float32
	IPAdapterImage    *SDImage
	IPAdapterStrength float32
	PhotoMaker        PhotoMakerParams
	PuLID             PuLIDParams
	VAETiling         TilingParams
	Cache             CacheParams
	Hires             HiresParams
	QwenImageLayers   int32
}

var (
	// SD_API void sd_img_gen_params_init(sd_img_gen_params_t* sd_img_gen_params);
	imgGenParamsInitFunc ffi.Fun

	// SD_API bool generate_image(sd_ctx_t*, const sd_img_gen_params_t*, sd_image_t**, int*);
	generateImageFunc ffi.Fun

	// SD_API void free_sd_images(sd_image_t* result_images, int num_images);
	freeSDImagesFunc ffi.Fun

	sampleParamsInitFunc ffi.Fun
	cacheParamsInitFunc  ffi.Fun
	hiresParamsInitFunc  ffi.Fun
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

	sampleParamsInitFunc = prepOptional(lib, "sd_sample_params_init", &ffi.TypeVoid, &ffi.TypePointer)
	cacheParamsInitFunc = prepOptional(lib, "sd_cache_params_init", &ffi.TypeVoid, &ffi.TypePointer)
	hiresParamsInitFunc = prepOptional(lib, "sd_hires_params_init", &ffi.TypeVoid, &ffi.TypePointer)

	return nil
}

// ImgGenParamsInit returns an ImgGenParams populated with the defaults from
// sd_img_gen_params_init. Callers must at minimum set Prompt before passing
// to GenerateImage.
func ImgGenParamsInit() ImgGenParams {
	raw := defaultCImgGenParams()
	return ImgGenParams{
		ClipSkip:          raw.ClipSkip,
		Width:             raw.Width,
		Height:            raw.Height,
		Steps:             raw.SampleParams.SampleSteps,
		CFGScale:          raw.SampleParams.Guidance.TxtCfg,
		ImageCFG:          raw.SampleParams.Guidance.ImgCfg,
		DistilledGuidance: raw.SampleParams.Guidance.DistilledGuidance,
		SLG:               slgParamsFromC(raw.SampleParams.Guidance.SLG),
		Sampler:           SampleMethod(raw.SampleParams.SampleMethod),
		Scheduler:         Scheduler(raw.SampleParams.Scheduler),
		Eta:               raw.SampleParams.Eta,
		ShiftedTimestep:   raw.SampleParams.ShiftedTimestep,
		FlowShift:         raw.SampleParams.FlowShift,
		ExtraSampleArgs:   cString(raw.SampleParams.ExtraSampleArgs),
		CustomSigmas:      copyCFloat32s(raw.SampleParams.CustomSigmas, raw.SampleParams.CustomSigmasCount),
		Seed:              raw.Seed,
		BatchCount:        raw.BatchCount,
		Strength:          raw.Strength,
		CircularX:         raw.CircularX != 0,
		CircularY:         raw.CircularY != 0,
		ControlStrength:   raw.ControlStrength,
		IPAdapterStrength: raw.IPAdapterStrength,
		PhotoMaker:        photoMakerParamsFromC(raw.PMParams),
		PuLID:             pulidParamsFromC(raw.PulidParams),
		VAETiling:         tilingParamsFromC(raw.VAETilingParams),
		Cache:             cacheParamsFromC(raw.Cache),
		Hires:             hiresParamsFromC(raw.Hires),
		QwenImageLayers:   raw.QwenImageLayers,
	}
}

func defaultCImgGenParams() cImgGenParams {
	var raw cImgGenParams
	rawPtr := &raw
	imgGenParamsInitFunc.Call(nil, unsafe.Pointer(&rawPtr))
	return raw
}

// SampleParamsInit returns stable-diffusion.cpp's sampling defaults.
func SampleParamsInit() SampleParams {
	var raw cSampleParams
	if sampleParamsInitFunc != (ffi.Fun{}) {
		ptr := &raw
		sampleParamsInitFunc.Call(nil, unsafe.Pointer(&ptr))
	} else {
		raw = defaultCImgGenParams().SampleParams
	}
	return sampleParamsFromC(raw)
}

// CacheParamsInit returns stable-diffusion.cpp's cache defaults.
func CacheParamsInit() CacheParams {
	var raw cCacheParams
	if cacheParamsInitFunc != (ffi.Fun{}) {
		ptr := &raw
		cacheParamsInitFunc.Call(nil, unsafe.Pointer(&ptr))
	} else {
		raw = defaultCImgGenParams().Cache
	}
	return cacheParamsFromC(raw)
}

// HiresParamsInit returns stable-diffusion.cpp's high-resolution pass defaults.
func HiresParamsInit() HiresParams {
	var raw cHiresParams
	if hiresParamsInitFunc != (ffi.Fun{}) {
		ptr := &raw
		hiresParamsInitFunc.Call(nil, unsafe.Pointer(&ptr))
	} else {
		raw = defaultCImgGenParams().Hires
	}
	return hiresParamsFromC(raw)
}

// GenerateImage runs the diffusion pipeline configured by ctx and params and
// returns a single decoded image. The returned SDImage owns its pixel data
// (copied out of the C heap), so it remains valid after the context is
// freed. The underlying result batch is released by stable-diffusion.cpp's
// free_sd_images before this function returns.
func GenerateImage(ctx Context, params ImgGenParams) (*SDImage, error) {
	images, err := GenerateImages(ctx, params)
	if err != nil {
		return nil, err
	}
	return images[0], nil
}

// GenerateImages runs the diffusion pipeline and returns every image produced
// by the requested batch. All returned pixel buffers are copied into Go-owned
// memory before the native result array is freed with free_sd_images.
func GenerateImages(ctx Context, params ImgGenParams) ([]*SDImage, error) {
	if ctx == 0 {
		return nil, errors.New("GenerateImages: nil context")
	}

	state, err := marshalImgGenParams(params)
	if err != nil {
		return nil, err
	}
	raw := state.raw

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
	runtime.KeepAlive(state)
	runtime.KeepAlive(params)
	runtime.KeepAlive(&raw)

	if byte(result) == 0 || resultPtr == nil || resultCount == 0 {
		if resultPtr != nil && resultCount > 0 {
			freeSDImagesFunc.Call(nil, unsafe.Pointer(&resultPtr), unsafe.Pointer(&resultCount))
		}
		if last := LastError(); last != "" {
			return nil, fmt.Errorf("generate_image failed: %s", last)
		}
		return nil, errors.New("generate_image failed (no log message captured)")
	}

	rawImages := unsafe.Slice(resultPtr, int(resultCount))
	out := make([]*SDImage, resultCount)
	for i := range rawImages {
		out[i] = sdImageFromC(&rawImages[i])
	}
	freeSDImagesFunc.Call(nil, unsafe.Pointer(&resultPtr), unsafe.Pointer(&resultCount))

	return out, nil
}

type marshaledImgGenParams struct {
	raw              cImgGenParams
	refs             cStringRefs
	loras            []cLora
	refImages        []cImage
	photoMakerImages []cImage
	slgLayers        []int32
}

func marshalImgGenParams(params ImgGenParams) (*marshaledImgGenParams, error) {
	state := &marshaledImgGenParams{raw: defaultCImgGenParams()}
	raw := &state.raw
	raw.ClipSkip = params.ClipSkip
	raw.Width = params.Width
	raw.Height = params.Height
	raw.Seed = params.Seed
	raw.BatchCount = params.BatchCount
	raw.Strength = params.Strength
	raw.SampleParams.SampleSteps = params.Steps
	raw.SampleParams.Guidance.TxtCfg = params.CFGScale
	raw.SampleParams.Guidance.ImgCfg = params.ImageCFG
	raw.SampleParams.Guidance.DistilledGuidance = params.DistilledGuidance
	raw.SampleParams.SampleMethod = int32(params.Sampler)
	raw.SampleParams.Scheduler = int32(params.Scheduler)
	raw.SampleParams.Eta = params.Eta
	raw.SampleParams.ShiftedTimestep = params.ShiftedTimestep
	raw.SampleParams.FlowShift = params.FlowShift
	raw.CircularX = boolToU8(params.CircularX)
	raw.CircularY = boolToU8(params.CircularY)
	raw.ControlStrength = params.ControlStrength
	raw.IPAdapterStrength = params.IPAdapterStrength
	raw.QwenImageLayers = params.QwenImageLayers

	for _, item := range []struct {
		dst   **byte
		value string
	}{
		{&raw.Prompt, params.Prompt},
		{&raw.NegativePrompt, params.NegativePrompt},
		{&raw.RefImageArgs, params.RefImageArgs},
		{&raw.SampleParams.ExtraSampleArgs, params.ExtraSampleArgs},
		{&raw.PMParams.IDEmbedPath, params.PhotoMaker.IDEmbedPath},
		{&raw.PulidParams.IDEmbeddingPath, params.PuLID.IDEmbeddingPath},
		{&raw.VAETilingParams.ExtraTilingArgs, params.VAETiling.ExtraArgs},
		{&raw.Cache.SCMMask, params.Cache.SCMMask},
		{&raw.Hires.ModelPath, params.Hires.ModelPath},
	} {
		ptr, err := state.refs.addPointer(item.value)
		if err != nil {
			return nil, err
		}
		*item.dst = ptr
	}

	var err error
	if err = bindOptionalCImage(&raw.InitImage, params.InitImage, "InitImage"); err != nil {
		return nil, err
	}
	if err = bindOptionalCImage(&raw.MaskImage, params.MaskImage, "MaskImage"); err != nil {
		return nil, err
	}
	if err = bindOptionalCImage(&raw.ControlImage, params.ControlImage, "ControlImage"); err != nil {
		return nil, err
	}
	if err = bindOptionalCImage(&raw.IPAdapterImage, params.IPAdapterImage, "IPAdapterImage"); err != nil {
		return nil, err
	}
	state.refImages, err = bindCImages(params.RefImages, "RefImages")
	if err != nil {
		return nil, err
	}
	if len(state.refImages) > 0 {
		raw.RefImages = uintptr(unsafe.Pointer(&state.refImages[0]))
		raw.RefImagesCount = int32(len(state.refImages))
	}
	state.photoMakerImages, err = bindCImages(params.PhotoMaker.IDImages, "PhotoMaker.IDImages")
	if err != nil {
		return nil, err
	}
	if len(state.photoMakerImages) > 0 {
		raw.PMParams.IDImages = uintptr(unsafe.Pointer(&state.photoMakerImages[0]))
		raw.PMParams.IDImagesCount = int32(len(state.photoMakerImages))
	}
	raw.PMParams.StyleStrength = params.PhotoMaker.StyleStrength
	raw.PulidParams.IDWeight = params.PuLID.IDWeight

	state.loras = make([]cLora, len(params.Loras))
	for i := range params.Loras {
		path, err := state.refs.add(params.Loras[i].Path)
		if err != nil {
			return nil, err
		}
		state.loras[i] = cLora{
			IsHighNoise: boolToU8(params.Loras[i].IsHighNoise),
			Multiplier:  params.Loras[i].Multiplier,
			Path:        path,
		}
	}
	if len(state.loras) > 0 {
		raw.Loras = uintptr(unsafe.Pointer(&state.loras[0]))
		raw.LoraCount = uint32(len(state.loras))
	}

	state.slgLayers = append([]int32(nil), params.SLG.Layers...)
	if len(state.slgLayers) > 0 {
		raw.SampleParams.Guidance.SLG.Layers = &state.slgLayers[0]
	}
	raw.SampleParams.Guidance.SLG.LayerCount = uint64(len(state.slgLayers))
	raw.SampleParams.Guidance.SLG.LayerStart = params.SLG.LayerStart
	raw.SampleParams.Guidance.SLG.LayerEnd = params.SLG.LayerEnd
	raw.SampleParams.Guidance.SLG.Scale = params.SLG.Scale
	raw.SampleParams.CustomSigmas = float32SlicePtr(params.CustomSigmas)
	raw.SampleParams.CustomSigmasCount = int32(len(params.CustomSigmas))

	raw.VAETilingParams = tilingParamsToC(params.VAETiling, raw.VAETilingParams.ExtraTilingArgs)
	raw.Cache = cacheParamsToC(params.Cache, raw.Cache.SCMMask)
	raw.Hires = hiresParamsToC(params.Hires, raw.Hires.ModelPath)
	raw.Hires.CustomSigmas = float32SlicePtr(params.Hires.CustomSigmas)
	raw.Hires.CustomSigmasCount = int32(len(params.Hires.CustomSigmas))

	return state, nil
}

func bindOptionalCImage(dst *cImage, src *SDImage, field string) error {
	if src == nil {
		*dst = cImage{}
		return nil
	}
	return bindCImage(dst, src, field)
}

func bindCImages(images []*SDImage, field string) ([]cImage, error) {
	result := make([]cImage, len(images))
	for i := range images {
		if images[i] == nil {
			return nil, fmt.Errorf("%s[%d]: nil image", field, i)
		}
		if err := bindCImage(&result[i], images[i], fmt.Sprintf("%s[%d]", field, i)); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func float32SlicePtr(values []float32) *float32 {
	if len(values) == 0 {
		return nil
	}
	return &values[0]
}

func sampleParamsFromC(raw cSampleParams) SampleParams {
	return SampleParams{
		Guidance: GuidanceParams{
			TextCFG:           raw.Guidance.TxtCfg,
			ImageCFG:          raw.Guidance.ImgCfg,
			DistilledGuidance: raw.Guidance.DistilledGuidance,
			SLG:               slgParamsFromC(raw.Guidance.SLG),
		},
		Scheduler:       Scheduler(raw.Scheduler),
		Method:          SampleMethod(raw.SampleMethod),
		Steps:           raw.SampleSteps,
		Eta:             raw.Eta,
		ShiftedTimestep: raw.ShiftedTimestep,
		FlowShift:       raw.FlowShift,
		ExtraArgs:       cString(raw.ExtraSampleArgs),
		CustomSigmas:    copyCFloat32s(raw.CustomSigmas, raw.CustomSigmasCount),
	}
}

func slgParamsFromC(raw cSlgParams) SLGParams {
	params := SLGParams{LayerStart: raw.LayerStart, LayerEnd: raw.LayerEnd, Scale: raw.Scale}
	if raw.Layers != nil && raw.LayerCount > 0 {
		params.Layers = append([]int32(nil), unsafe.Slice(raw.Layers, int(raw.LayerCount))...)
	}
	return params
}

func photoMakerParamsFromC(raw cPMParams) PhotoMakerParams {
	return PhotoMakerParams{IDEmbedPath: cString(raw.IDEmbedPath), StyleStrength: raw.StyleStrength}
}

func pulidParamsFromC(raw cPulidParams) PuLIDParams {
	return PuLIDParams{IDEmbeddingPath: cString(raw.IDEmbeddingPath), IDWeight: raw.IDWeight}
}

func tilingParamsFromC(raw cTilingParams) TilingParams {
	return TilingParams{
		Enabled: raw.Enabled != 0, TemporalTiling: raw.TemporalTiling != 0,
		TileSizeX: raw.TileSizeX, TileSizeY: raw.TileSizeY, TargetOverlap: raw.TargetOverlap,
		RelativeSizeX: raw.RelSizeX, RelativeSizeY: raw.RelSizeY, ExtraArgs: cString(raw.ExtraTilingArgs),
	}
}

func tilingParamsToC(params TilingParams, extra *byte) cTilingParams {
	return cTilingParams{
		Enabled: boolToU8(params.Enabled), TemporalTiling: boolToU8(params.TemporalTiling),
		TileSizeX: params.TileSizeX, TileSizeY: params.TileSizeY, TargetOverlap: params.TargetOverlap,
		RelSizeX: params.RelativeSizeX, RelSizeY: params.RelativeSizeY, ExtraTilingArgs: extra,
	}
}

func cacheParamsFromC(raw cCacheParams) CacheParams {
	return CacheParams{
		Mode: CacheMode(raw.Mode), ReuseThreshold: raw.ReuseThreshold, StartPercent: raw.StartPercent,
		EndPercent: raw.EndPercent, ErrorDecayRate: raw.ErrorDecayRate,
		UseRelativeThreshold: raw.UseRelativeThreshold != 0, ResetErrorOnCompute: raw.ResetErrorOnCompute != 0,
		FnComputeBlocks: raw.FnComputeBlocks, BnComputeBlocks: raw.BnComputeBlocks,
		ResidualDiffThreshold: raw.ResidualDiffThreshold, MaxWarmupSteps: raw.MaxWarmupSteps,
		MaxCachedSteps: raw.MaxCachedSteps, MaxContinuousCachedSteps: raw.MaxContinuousCachedSteps,
		TaylorseerNDerivatives: raw.TaylorseerNDerivatives, TaylorseerSkipInterval: raw.TaylorseerSkipInterval,
		SCMMask: cString(raw.SCMMask), SCMPolicyDynamic: raw.SCMPolicyDynamic != 0,
		SpectrumW: raw.SpectrumW, SpectrumM: raw.SpectrumM, SpectrumLambda: raw.SpectrumLam,
		SpectrumWindowSize: raw.SpectrumWindowSize, SpectrumFlexWindow: raw.SpectrumFlexWindow,
		SpectrumWarmupSteps: raw.SpectrumWarmupSteps, SpectrumStopPercent: raw.SpectrumStopPercent,
	}
}

func cacheParamsToC(params CacheParams, mask *byte) cCacheParams {
	return cCacheParams{
		Mode: int32(params.Mode), ReuseThreshold: params.ReuseThreshold, StartPercent: params.StartPercent,
		EndPercent: params.EndPercent, ErrorDecayRate: params.ErrorDecayRate,
		UseRelativeThreshold: boolToU8(params.UseRelativeThreshold), ResetErrorOnCompute: boolToU8(params.ResetErrorOnCompute),
		FnComputeBlocks: params.FnComputeBlocks, BnComputeBlocks: params.BnComputeBlocks,
		ResidualDiffThreshold: params.ResidualDiffThreshold, MaxWarmupSteps: params.MaxWarmupSteps,
		MaxCachedSteps: params.MaxCachedSteps, MaxContinuousCachedSteps: params.MaxContinuousCachedSteps,
		TaylorseerNDerivatives: params.TaylorseerNDerivatives, TaylorseerSkipInterval: params.TaylorseerSkipInterval,
		SCMMask: mask, SCMPolicyDynamic: boolToU8(params.SCMPolicyDynamic),
		SpectrumW: params.SpectrumW, SpectrumM: params.SpectrumM, SpectrumLam: params.SpectrumLambda,
		SpectrumWindowSize: params.SpectrumWindowSize, SpectrumFlexWindow: params.SpectrumFlexWindow,
		SpectrumWarmupSteps: params.SpectrumWarmupSteps, SpectrumStopPercent: params.SpectrumStopPercent,
	}
}

func hiresParamsFromC(raw cHiresParams) HiresParams {
	return HiresParams{
		Enabled: raw.Enabled != 0, Upscaler: HiresUpscaler(raw.Upscaler), ModelPath: cString(raw.ModelPath),
		Scale: raw.Scale, TargetWidth: raw.TargetWidth, TargetHeight: raw.TargetHeight, Steps: raw.Steps,
		DenoisingStrength: raw.DenoisingStrength, UpscaleTileSize: raw.UpscaleTileSize,
		CustomSigmas: copyCFloat32s(raw.CustomSigmas, raw.CustomSigmasCount),
	}
}

func hiresParamsToC(params HiresParams, modelPath *byte) cHiresParams {
	return cHiresParams{
		Enabled: boolToU8(params.Enabled), Upscaler: int32(params.Upscaler), ModelPath: modelPath,
		Scale: params.Scale, TargetWidth: params.TargetWidth, TargetHeight: params.TargetHeight, Steps: params.Steps,
		DenoisingStrength: params.DenoisingStrength, UpscaleTileSize: params.UpscaleTileSize,
	}
}

func cString(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	return utils.BytePtrToString(ptr)
}

func copyCFloat32s(ptr *float32, count int32) []float32 {
	if ptr == nil || count <= 0 {
		return nil
	}
	return append([]float32(nil), unsafe.Slice(ptr, int(count))...)
}
