package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/jupiterrider/ffi"
)

// cAudio and cVideo mirror the live sd_audio_t and sd_video_t descriptors.
type cAudio struct {
	SampleRate uint32
	_          [4]byte
	Data       uintptr
}
type cVideo struct {
	Data  uintptr
	FPS   int32
	_     [4]byte
	Audio cAudio
}

// VideoGenerationResult owns frame pixels, audio, effective FPS and metadata.
type VideoGenerationResult struct {
	Frames         []*SDImage
	Audio          *Audio
	FPS            int32
	GenerationInfo string
}

// HighNoiseSampleParams contains only settings supported independently by the
// high-noise stage. Scheduler, sigmas, timestep and flow shift belong to Sample.
type HighNoiseSampleParams struct {
	Guidance  GuidanceParams
	Method    SampleMethod
	Steps     int32
	Eta       float32
	ExtraArgs string `sd:"args"`
}

// VideoGenParams configures video generation. Start with VideoGenParamsInit.
// Unchanged compatibility fields are unset; Options supplies explicit overrides.
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
	HighNoiseSample HighNoiseSampleParams
	MoeBoundary     float32
	Strength        float32
	Seed            int64
	VideoFrames     int32
	FPS             int32
	VaceStrength    float32
	VAETiling       TilingParams `sd:"vae_tiling"`
	Cache           CacheParams
	Hires           HiresParams
	CircularX       bool
	CircularY       bool
	ImagePreprocess ImagePreprocessParams
	RefImageArgs    string
	Options         Options
}

var generateVideoFunc ffi.Fun

func loadVideoFuncs(lib ffi.Lib) {
	generateVideoFunc = prepOptional(lib, "sd_generate_video", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer)
}

// VideoGenParamsInit returns Go compatibility defaults, not a native layout.
func VideoGenParamsInit() (VideoGenParams, error) {
	s := SampleParamsInit()
	return VideoGenParams{ClipSkip: -1, Width: 512, Height: 512, Sample: s, HighNoiseSample: HighNoiseSampleParams{Guidance: s.Guidance, Method: s.Method, Steps: -1, Eta: s.Eta}, MoeBoundary: .875, Strength: .75, Seed: -1, VideoFrames: 6, FPS: 16, VaceStrength: 1, VAETiling: TilingParams{TargetOverlap: .5}, Cache: CacheParamsInit(), Hires: HiresParamsInit()}, nil
}

func audioToC(audio *Audio, field string) (cAudio, error) {
	if audio == nil || audio.Channels == 0 {
		return cAudio{}, fmt.Errorf("%s: zero channels", field)
	}
	if len(audio.Data)%int(audio.Channels) != 0 {
		return cAudio{}, fmt.Errorf("%s: incomplete sample frame", field)
	}
	t, err := createTensor(4, []int64{int64(audio.Channels), int64(audio.SampleCount())}, unsafe.Pointer(unsafe.SliceData(audio.Data)), uint64(len(audio.Data))*4)
	runtime.KeepAlive(audio)
	return cAudio{SampleRate: audio.SampleRate, Data: t}, err
}

func audioFromC(raw *cAudio) (*Audio, error) {
	if raw == nil || raw.Data == 0 {
		return nil, nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	shape, err := tensorShape(raw.Data, 4, 2)
	if err != nil {
		return nil, err
	}
	var data *float32
	tensorDataFunc.Call(unsafe.Pointer(&data), unsafe.Pointer(&raw.Data))
	if data == nil {
		return nil, fmt.Errorf("reading audio tensor: %s", nativeDiagnostic())
	}
	out := Audio{SampleRate: raw.SampleRate, Channels: uint32(shape[0]), Data: append([]float32(nil), unsafe.Slice(data, int(shape[0]*shape[1]))...)}
	return &out, nil
}

// GenerateVideo returns Go-owned frame and audio copies.
func GenerateVideo(ctx Context, params VideoGenParams) ([]*SDImage, *Audio, error) {
	frames, audio, _, err := GenerateVideoWithFPS(ctx, params)
	return frames, audio, err
}

// GenerateVideoWithFPS additionally returns the actual native frame rate.
func GenerateVideoWithFPS(ctx Context, params VideoGenParams) ([]*SDImage, *Audio, int32, error) {
	r, err := GenerateVideoWithInfo(ctx, params)
	if err != nil {
		return nil, nil, 0, err
	}
	return r.Frames, r.Audio, r.FPS, nil
}

// GenerateVideoWithInfo returns all native outputs, independently owned by Go.
func GenerateVideoWithInfo(ctx Context, params VideoGenParams) (*VideoGenerationResult, error) {
	if ctx == 0 {
		return nil, errors.New("GenerateVideo: nil context")
	}
	if generateVideoFunc == (ffi.Fun{}) {
		return nil, unsupported("sd_generate_video")
	}
	o, err := newOptions("vid_gen")
	if err != nil {
		return nil, err
	}
	defer o.free()
	defaults, err := VideoGenParamsInit()
	if err != nil {
		return nil, err
	}
	if err := o.fields(params, defaults, ""); err != nil {
		return nil, err
	}
	result, err := callResult(generateVideoFunc, unsafe.Pointer(&ctx), unsafe.Pointer(&o.ptr))
	if err != nil {
		return nil, err
	}
	defer resultFreeFunc.Call(nil, unsafe.Pointer(&result))
	frames, err := resultImages(result)
	if err != nil {
		return nil, err
	}
	var raw *cAudio
	resultAudioFunc.Call(unsafe.Pointer(&raw), unsafe.Pointer(&result))
	audio, err := audioFromC(raw)
	if err != nil {
		return nil, err
	}
	var fps ffi.Arg
	resultFPSFunc.Call(&fps, unsafe.Pointer(&result))
	out := VideoGenerationResult{Frames: frames, Audio: audio, FPS: int32(fps), GenerationInfo: frames[0].GenerationInfo}
	return &out, nil
}
