package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// cAudio mirrors sd_audio_t. Size: 24 bytes.
type cAudio struct {
	SampleRate  uint32
	Channels    uint32
	SampleCount uint64
	Data        *float32
}

// cRefVideo mirrors sd_ref_video_t. Size: 40 bytes.
type cRefVideo struct {
	Frames     uintptr
	FrameCount int32
	FPS        int32
	Audio      cAudio
}

// cVidGenParams mirrors sd_vid_gen_params_t. Size: 576 bytes.
type cVidGenParams struct {
	Loras                 uintptr
	LoraCount             uint32
	_                     [4]byte
	Prompt                *byte
	NegativePrompt        *byte
	ClipSkip              int32
	_                     [4]byte
	InitImage             cImage
	EndImage              cImage
	RefImages             uintptr
	RefImagesCount        int32
	_                     [4]byte
	RefVideos             uintptr
	RefVideosCount        int32
	_                     [4]byte
	RefAudios             uintptr
	RefAudiosCount        int32
	_                     [4]byte
	ControlFrames         uintptr
	ControlFramesSize     int32
	Width                 int32
	Height                int32
	_                     [4]byte
	SampleParams          cSampleParams
	HighNoiseSampleParams cSampleParams
	MoeBoundary           float32
	Strength              float32
	Seed                  int64
	VideoFrames           int32
	FPS                   int32
	VaceStrength          float32
	_                     [4]byte
	VAETilingParams       cTilingParams
	Cache                 cCacheParams
	Hires                 cHiresParams
	CircularX             uint8
	CircularY             uint8
	_                     [6]byte
}

// VideoGenParams is the Go-side representation of sd_vid_gen_params_t.
type VideoGenParams struct {
	Loras           []Lora
	Prompt          string
	NegativePrompt  string
	ClipSkip        int32
	InitImage       *SDImage
	EndImage        *SDImage
	RefImages       []*SDImage
	RefVideos       []RefVideo
	RefAudios       []Audio
	ControlFrames   []*SDImage
	Width           int32
	Height          int32
	Sample          SampleParams
	HighNoiseSample SampleParams
	MoeBoundary     float32
	Strength        float32
	Seed            int64
	VideoFrames     int32
	FPS             int32
	VaceStrength    float32
	VAETiling       TilingParams
	Cache           CacheParams
	Hires           HiresParams
	CircularX       bool
	CircularY       bool
}

var (
	vidGenParamsInitFunc ffi.Fun
	generateVideoFunc    ffi.Fun
	freeSDAudioFunc      ffi.Fun
)

func loadVideoFuncs(lib ffi.Lib) {
	vidGenParamsInitFunc = prepOptional(lib, "sd_vid_gen_params_init", &ffi.TypeVoid, &ffi.TypePointer)
	generateVideoFunc = prepOptional(lib, "generate_video", &ffi.TypeUint8,
		&ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
	freeSDAudioFunc = prepOptional(lib, "free_sd_audio", &ffi.TypeVoid, &ffi.TypePointer)
}

// VideoGenParamsInit returns VideoGenParams populated with native defaults.
func VideoGenParamsInit() (VideoGenParams, error) {
	if vidGenParamsInitFunc == (ffi.Fun{}) {
		return VideoGenParams{}, unsupported("sd_vid_gen_params_init")
	}
	var raw cVidGenParams
	ptr := &raw
	vidGenParamsInitFunc.Call(nil, unsafe.Pointer(&ptr))
	return VideoGenParams{
		ClipSkip:        raw.ClipSkip,
		Width:           raw.Width,
		Height:          raw.Height,
		Sample:          sampleParamsFromC(raw.SampleParams),
		HighNoiseSample: sampleParamsFromC(raw.HighNoiseSampleParams),
		MoeBoundary:     raw.MoeBoundary,
		Strength:        raw.Strength,
		Seed:            raw.Seed,
		VideoFrames:     raw.VideoFrames,
		FPS:             raw.FPS,
		VaceStrength:    raw.VaceStrength,
		VAETiling:       tilingParamsFromC(raw.VAETilingParams),
		Cache:           cacheParamsFromC(raw.Cache),
		Hires:           hiresParamsFromC(raw.Hires),
		CircularX:       raw.CircularX != 0,
		CircularY:       raw.CircularY != 0,
	}, nil
}

type marshaledVideoParams struct {
	raw            cVidGenParams
	refs           cStringRefs
	loras          []cLora
	refImages      []cImage
	controlFrames  []cImage
	refVideos      []cRefVideo
	refVideoFrames [][]cImage
	refAudios      []cAudio
	sampleLayers   []int32
	highLayers     []int32
}

func marshalVideoParams(params VideoGenParams) (*marshaledVideoParams, error) {
	state := &marshaledVideoParams{}
	if vidGenParamsInitFunc == (ffi.Fun{}) {
		return nil, unsupported("sd_vid_gen_params_init")
	}
	ptr := &state.raw
	vidGenParamsInitFunc.Call(nil, unsafe.Pointer(&ptr))
	raw := &state.raw
	raw.ClipSkip = params.ClipSkip
	raw.Width = params.Width
	raw.Height = params.Height
	raw.MoeBoundary = params.MoeBoundary
	raw.Strength = params.Strength
	raw.Seed = params.Seed
	raw.VideoFrames = params.VideoFrames
	raw.FPS = params.FPS
	raw.VaceStrength = params.VaceStrength
	raw.CircularX = boolToU8(params.CircularX)
	raw.CircularY = boolToU8(params.CircularY)

	for _, item := range []struct {
		dst   **byte
		value string
	}{
		{&raw.Prompt, params.Prompt},
		{&raw.NegativePrompt, params.NegativePrompt},
		{&raw.SampleParams.ExtraSampleArgs, params.Sample.ExtraArgs},
		{&raw.HighNoiseSampleParams.ExtraSampleArgs, params.HighNoiseSample.ExtraArgs},
		{&raw.VAETilingParams.ExtraTilingArgs, params.VAETiling.ExtraArgs},
		{&raw.Cache.SCMMask, params.Cache.SCMMask},
		{&raw.Hires.ModelPath, params.Hires.ModelPath},
	} {
		value, err := state.refs.addPointer(item.value)
		if err != nil {
			return nil, err
		}
		*item.dst = value
	}

	if err := bindOptionalCImage(&raw.InitImage, params.InitImage, "InitImage"); err != nil {
		return nil, err
	}
	if err := bindOptionalCImage(&raw.EndImage, params.EndImage, "EndImage"); err != nil {
		return nil, err
	}
	var err error
	state.refImages, err = bindCImages(params.RefImages, "RefImages")
	if err != nil {
		return nil, err
	}
	if len(state.refImages) > 0 {
		raw.RefImages = uintptr(unsafe.Pointer(&state.refImages[0]))
		raw.RefImagesCount = int32(len(state.refImages))
	}
	state.controlFrames, err = bindCImages(params.ControlFrames, "ControlFrames")
	if err != nil {
		return nil, err
	}
	if len(state.controlFrames) > 0 {
		raw.ControlFrames = uintptr(unsafe.Pointer(&state.controlFrames[0]))
		raw.ControlFramesSize = int32(len(state.controlFrames))
	}

	state.loras = make([]cLora, len(params.Loras))
	for i := range params.Loras {
		path, err := state.refs.add(params.Loras[i].Path)
		if err != nil {
			return nil, err
		}
		state.loras[i] = cLora{IsHighNoise: boolToU8(params.Loras[i].IsHighNoise), Multiplier: params.Loras[i].Multiplier, Path: path}
	}
	if len(state.loras) > 0 {
		raw.Loras = uintptr(unsafe.Pointer(&state.loras[0]))
		raw.LoraCount = uint32(len(state.loras))
	}

	state.refAudios = make([]cAudio, len(params.RefAudios))
	for i := range params.RefAudios {
		audio, err := audioToC(&params.RefAudios[i], fmt.Sprintf("RefAudios[%d]", i))
		if err != nil {
			return nil, err
		}
		state.refAudios[i] = audio
	}
	if len(state.refAudios) > 0 {
		raw.RefAudios = uintptr(unsafe.Pointer(&state.refAudios[0]))
		raw.RefAudiosCount = int32(len(state.refAudios))
	}

	state.refVideos = make([]cRefVideo, len(params.RefVideos))
	state.refVideoFrames = make([][]cImage, len(params.RefVideos))
	for i := range params.RefVideos {
		frames, err := bindCImages(params.RefVideos[i].Frames, fmt.Sprintf("RefVideos[%d].Frames", i))
		if err != nil {
			return nil, err
		}
		state.refVideoFrames[i] = frames
		video := cRefVideo{FrameCount: int32(len(frames)), FPS: params.RefVideos[i].FPS}
		if len(frames) > 0 {
			video.Frames = uintptr(unsafe.Pointer(&frames[0]))
		}
		if params.RefVideos[i].Audio != nil {
			audio, err := audioToC(params.RefVideos[i].Audio, fmt.Sprintf("RefVideos[%d].Audio", i))
			if err != nil {
				return nil, err
			}
			video.Audio = audio
		}
		state.refVideos[i] = video
	}
	if len(state.refVideos) > 0 {
		raw.RefVideos = uintptr(unsafe.Pointer(&state.refVideos[0]))
		raw.RefVideosCount = int32(len(state.refVideos))
	}

	state.sampleLayers = marshalSampleParams(&raw.SampleParams, params.Sample)
	state.highLayers = marshalSampleParams(&raw.HighNoiseSampleParams, params.HighNoiseSample)
	raw.VAETilingParams = tilingParamsToC(params.VAETiling, raw.VAETilingParams.ExtraTilingArgs)
	raw.Cache = cacheParamsToC(params.Cache, raw.Cache.SCMMask)
	raw.Hires = hiresParamsToC(params.Hires, raw.Hires.ModelPath)
	raw.Hires.CustomSigmas = float32SlicePtr(params.Hires.CustomSigmas)
	raw.Hires.CustomSigmasCount = int32(len(params.Hires.CustomSigmas))
	return state, nil
}

func marshalSampleParams(raw *cSampleParams, params SampleParams) []int32 {
	raw.Guidance.TxtCfg = params.Guidance.TextCFG
	raw.Guidance.ImgCfg = params.Guidance.ImageCFG
	raw.Guidance.DistilledGuidance = params.Guidance.DistilledGuidance
	raw.Guidance.SLG.LayerStart = params.Guidance.SLG.LayerStart
	raw.Guidance.SLG.LayerEnd = params.Guidance.SLG.LayerEnd
	raw.Guidance.SLG.Scale = params.Guidance.SLG.Scale
	raw.Scheduler = int32(params.Scheduler)
	raw.SampleMethod = int32(params.Method)
	raw.SampleSteps = params.Steps
	raw.Eta = params.Eta
	raw.ShiftedTimestep = params.ShiftedTimestep
	raw.CustomSigmas = float32SlicePtr(params.CustomSigmas)
	raw.CustomSigmasCount = int32(len(params.CustomSigmas))
	raw.FlowShift = params.FlowShift
	layers := append([]int32(nil), params.Guidance.SLG.Layers...)
	if len(layers) > 0 {
		raw.Guidance.SLG.Layers = &layers[0]
	}
	raw.Guidance.SLG.LayerCount = uint64(len(layers))
	return layers
}

func audioToC(audio *Audio, field string) (cAudio, error) {
	if audio.Channels == 0 {
		return cAudio{}, fmt.Errorf("%s: zero channels", field)
	}
	if len(audio.Data)%int(audio.Channels) != 0 {
		return cAudio{}, fmt.Errorf("%s: data length %d is not divisible by %d channels", field, len(audio.Data), audio.Channels)
	}
	raw := cAudio{SampleRate: audio.SampleRate, Channels: audio.Channels, SampleCount: audio.SampleCount()}
	if len(audio.Data) > 0 {
		raw.Data = &audio.Data[0]
	}
	return raw, nil
}

func audioFromC(raw *cAudio) *Audio {
	if raw == nil {
		return nil
	}
	audio := &Audio{SampleRate: raw.SampleRate, Channels: raw.Channels}
	if raw.Data != nil && raw.SampleCount > 0 && raw.Channels > 0 {
		count := int(raw.SampleCount) * int(raw.Channels)
		audio.Data = append([]float32(nil), unsafe.Slice(raw.Data, count)...)
	}
	return audio
}

// GenerateVideo runs native video generation and returns Go-owned frame and
// audio copies. Native results are released with their matched allocators.
func GenerateVideo(ctx Context, params VideoGenParams) ([]*SDImage, *Audio, error) {
	if ctx == 0 {
		return nil, nil, errors.New("GenerateVideo: nil context")
	}
	if generateVideoFunc == (ffi.Fun{}) {
		return nil, nil, unsupported("generate_video")
	}
	if freeSDAudioFunc == (ffi.Fun{}) {
		return nil, nil, unsupported("free_sd_audio")
	}
	state, err := marshalVideoParams(params)
	if err != nil {
		return nil, nil, err
	}
	rawPtr := &state.raw
	var framePtr *cImage
	var frameCount int32
	var audioPtr *cAudio
	framePtrPtr := &framePtr
	frameCountPtr := &frameCount
	audioPtrPtr := &audioPtr
	var result ffi.Arg
	generateVideoFunc.Call(&result, unsafe.Pointer(&ctx), unsafe.Pointer(&rawPtr), unsafe.Pointer(&framePtrPtr), unsafe.Pointer(&frameCountPtr), unsafe.Pointer(&audioPtrPtr))
	runtime.KeepAlive(state)
	runtime.KeepAlive(params)
	if byte(result) == 0 {
		if last := LastError(); last != "" {
			return nil, nil, fmt.Errorf("generate_video failed: %s", last)
		}
		return nil, nil, errors.New("generate_video failed (no log message captured)")
	}
	frames := make([]*SDImage, frameCount)
	if framePtr != nil && frameCount > 0 {
		rawFrames := unsafe.Slice(framePtr, int(frameCount))
		for i := range rawFrames {
			frames[i] = sdImageFromC(&rawFrames[i])
		}
		freeSDImagesFunc.Call(nil, unsafe.Pointer(&framePtr), unsafe.Pointer(&frameCount))
	}
	audio := audioFromC(audioPtr)
	if audioPtr != nil {
		freeSDAudioFunc.Call(nil, unsafe.Pointer(&audioPtr))
	}
	return frames, audio, nil
}
