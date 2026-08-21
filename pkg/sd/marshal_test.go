package sd

import (
	"reflect"
	"testing"
	"unsafe"
)

func TestMarshalContextParams(t *testing.T) {
	params := ContextParams{
		ModelPath: "model", ClipLPath: "clip-l", ClipGPath: "clip-g", ClipVisionPath: "clip-vision",
		T5XXLPath: "t5", LLMPath: "llm", LLMVisionPath: "llm-vision", DiffusionModelPath: "diffusion",
		HighNoiseDiffusionModelPath: "high-noise", UncondDiffusionModelPath: "uncond", EmbeddingsConnectorsPath: "connectors",
		VAEPath: "vae", AudioVAEPath: "audio-vae", TAESDPath: "taesd", ControlNetPath: "control",
		IPAdapterPath: "ip-adapter", MotionModulePath: "motion", Embeddings: []Embedding{{Name: "one", Path: "one.pt"}, {Name: "two", Path: "two.pt"}},
		PhotoMakerPath: "photomaker", PulidWeightsPath: "pulid", TensorTypeRules: "rules", NThreads: 7,
		Wtype: SDTypeQ5K, RngType: RngCPU, SamplerRngType: RngCuda, Prediction: PredictionFlow, LoraApplyMode: LoraApplyAtRuntime,
		EnableMmap: true, FlashAttn: true, DiffusionFlashAttn: true, TaePreviewOnly: true,
		DiffusionConvDirect: true, VAEConvDirect: true, ForceSDXLVAEConvScale: true, VaeFormat: SDVaeFormatFlux2,
		MaxVram: "12", StreamLayers: true, EagerLoad: true, Backend: "metal", ParamsBackend: "cpu",
		SplitMode: "layer", AutoFit: true, RPCServers: "rpc", ModelArgs: "args",
	}

	state, err := marshalContextParams(params)
	if err != nil {
		t.Fatalf("marshalContextParams: %v", err)
	}
	raw := state.raw

	strings := []struct {
		name string
		got  uintptr
		want string
	}{
		{"ModelPath", raw.ModelPath, params.ModelPath}, {"ClipLPath", raw.ClipLPath, params.ClipLPath},
		{"ClipGPath", raw.ClipGPath, params.ClipGPath}, {"ClipVisionPath", raw.ClipVisionPath, params.ClipVisionPath},
		{"T5XXLPath", raw.T5XXLPath, params.T5XXLPath}, {"LLMPath", raw.LLMPath, params.LLMPath},
		{"LLMVisionPath", raw.LLMVisionPath, params.LLMVisionPath}, {"DiffusionModelPath", raw.DiffusionModelPath, params.DiffusionModelPath},
		{"HighNoiseDiffusionModelPath", raw.HighNoiseDiffusionModelPath, params.HighNoiseDiffusionModelPath},
		{"UncondDiffusionModelPath", raw.UncondDiffusionModelPath, params.UncondDiffusionModelPath},
		{"EmbeddingsConnectorsPath", raw.EmbeddingsConnectorsPath, params.EmbeddingsConnectorsPath},
		{"VAEPath", raw.VAEPath, params.VAEPath}, {"AudioVAEPath", raw.AudioVAEPath, params.AudioVAEPath},
		{"TAESDPath", raw.TAESDPath, params.TAESDPath}, {"ControlNetPath", raw.ControlNetPath, params.ControlNetPath},
		{"IPAdapterPath", raw.IPAdapterPath, params.IPAdapterPath}, {"MotionModulePath", raw.MotionModulePath, params.MotionModulePath},
		{"PhotoMakerPath", raw.PhotoMakerPath, params.PhotoMakerPath}, {"PulidWeightsPath", raw.PulidWeightsPath, params.PulidWeightsPath},
		{"TensorTypeRules", raw.TensorTypeRules, params.TensorTypeRules}, {"MaxVram", raw.MaxVram, params.MaxVram},
		{"Backend", raw.Backend, params.Backend}, {"ParamsBackend", raw.ParamsBackend, params.ParamsBackend},
		{"SplitMode", raw.SplitMode, params.SplitMode}, {"RPCServers", raw.RPCServers, params.RPCServers},
		{"ModelArgs", raw.ModelArgs, params.ModelArgs},
	}
	for i, item := range strings {
		wantPointer := uintptr(unsafe.Pointer(state.refs.keep[i]))
		if item.got != wantPointer {
			t.Errorf("%s pointer: got %#x, want %#x", item.name, item.got, wantPointer)
		}
		if got := cString(state.refs.keep[i]); got != item.want {
			t.Errorf("%s: got %q, want %q", item.name, got, item.want)
		}
	}

	values := []struct {
		name string
		got  any
		want any
	}{
		{"NThreads", raw.NThreads, params.NThreads}, {"Wtype", raw.Wtype, int32(params.Wtype)},
		{"RngType", raw.RngType, int32(params.RngType)}, {"SamplerRngType", raw.SamplerRngType, int32(params.SamplerRngType)},
		{"Prediction", raw.Prediction, int32(params.Prediction)}, {"LoraApplyMode", raw.LoraApplyMode, int32(params.LoraApplyMode)},
		{"EnableMmap", raw.EnableMmap, uint8(1)}, {"FlashAttn", raw.FlashAttn, uint8(1)},
		{"DiffusionFlashAttn", raw.DiffusionFlashAttn, uint8(1)}, {"TaePreviewOnly", raw.TaePreviewOnly, uint8(1)},
		{"DiffusionConvDirect", raw.DiffusionConvDirect, uint8(1)}, {"VAEConvDirect", raw.VAEConvDirect, uint8(1)},
		{"ForceSDXLVAEConvScale", raw.ForceSDXLVAEConvScale, uint8(1)}, {"VaeFormat", raw.VaeFormat, int32(params.VaeFormat)},
		{"StreamLayers", raw.StreamLayers, uint8(1)}, {"EagerLoad", raw.EagerLoad, uint8(1)}, {"AutoFit", raw.AutoFit, uint8(1)},
		{"EmbeddingCount", raw.EmbeddingCount, uint32(len(params.Embeddings))},
	}
	for _, item := range values {
		if item.got != item.want {
			t.Errorf("%s: got %v, want %v", item.name, item.got, item.want)
		}
	}

	if len(state.embeddings) != len(params.Embeddings) {
		t.Fatalf("embeddings length: got %d, want %d", len(state.embeddings), len(params.Embeddings))
	}
	for i, embedding := range state.embeddings {
		name := state.refs.keep[len(strings)+i*2]
		path := state.refs.keep[len(strings)+i*2+1]
		if embedding.Name != uintptr(unsafe.Pointer(name)) {
			t.Errorf("Embeddings[%d].Name pointer does not reference its retained string", i)
		}
		if embedding.Path != uintptr(unsafe.Pointer(path)) {
			t.Errorf("Embeddings[%d].Path pointer does not reference its retained string", i)
		}
		if got := cString(name); got != params.Embeddings[i].Name {
			t.Errorf("Embeddings[%d].Name: got %q, want %q", i, got, params.Embeddings[i].Name)
		}
		if got := cString(path); got != params.Embeddings[i].Path {
			t.Errorf("Embeddings[%d].Path: got %q, want %q", i, got, params.Embeddings[i].Path)
		}
	}
}

func TestMarshalImgGenParams(t *testing.T) {
	testSetup(t)

	image := testMarshalImage(1)
	refImage := testMarshalImage(2)
	photoImage := testMarshalImage(3)
	params := ImgGenParams{
		Loras: []Lora{{IsHighNoise: true, Multiplier: 1.25, Path: "lora"}}, Prompt: "prompt", NegativePrompt: "negative",
		ClipSkip: 2, Width: 320, Height: 192, Steps: 11, CFGScale: 4.5, Sampler: SampleHeun, Scheduler: SchedulerKarras,
		Seed: 42, BatchCount: 3, CircularX: true, CircularY: true, Strength: 0.6,
		InitImage: image, RefImages: []*SDImage{refImage}, RefImageArgs: "ref-args", MaskImage: image,
		ImageCFG: 1.5, DistilledGuidance: 2.5, SLG: SLGParams{Layers: []int32{1, 4}, LayerStart: 0.1, LayerEnd: 0.7, Scale: 0.8},
		Eta: 0.2, ShiftedTimestep: 5, CustomSigmas: []float32{0.9, 0.4}, FlowShift: 1.2, ExtraSampleArgs: "sample-args",
		ControlImage: refImage, ControlStrength: 0.75, IPAdapterImage: image, IPAdapterStrength: 0.65,
		PhotoMaker: PhotoMakerParams{IDImages: []*SDImage{photoImage}, IDEmbedPath: "id-embed", StyleStrength: 9},
		PuLID:      PuLIDParams{IDEmbeddingPath: "pulid-embed", IDWeight: 0.55},
		VAETiling:  TilingParams{Enabled: true, TemporalTiling: true, TileSizeX: 64, TileSizeY: 72, TargetOverlap: 0.25, RelativeSizeX: 0.4, RelativeSizeY: 0.5, ExtraArgs: "tile-args"},
		Cache:      testCacheParams(), Hires: testHiresParams(), QwenImageLayers: 6,
	}

	state, err := marshalImgGenParams(params)
	if err != nil {
		t.Fatalf("marshalImgGenParams: %v", err)
	}
	raw := state.raw

	values := []struct {
		name string
		got  any
		want any
	}{
		{"Prompt", cString(raw.Prompt), params.Prompt}, {"NegativePrompt", cString(raw.NegativePrompt), params.NegativePrompt},
		{"ClipSkip", raw.ClipSkip, params.ClipSkip}, {"Width", raw.Width, params.Width}, {"Height", raw.Height, params.Height},
		{"Strength", raw.Strength, params.Strength}, {"Seed", raw.Seed, params.Seed}, {"BatchCount", raw.BatchCount, params.BatchCount},
		{"CircularX", raw.CircularX, uint8(1)}, {"CircularY", raw.CircularY, uint8(1)},
		{"RefImageArgs", cString(raw.RefImageArgs), params.RefImageArgs}, {"RefImagesCount", raw.RefImagesCount, int32(1)},
		{"ControlStrength", raw.ControlStrength, params.ControlStrength}, {"IPAdapterStrength", raw.IPAdapterStrength, params.IPAdapterStrength},
		{"PhotoMaker.IDEmbedPath", cString(raw.PMParams.IDEmbedPath), params.PhotoMaker.IDEmbedPath},
		{"PhotoMaker.StyleStrength", raw.PMParams.StyleStrength, params.PhotoMaker.StyleStrength},
		{"PhotoMaker.IDImagesCount", raw.PMParams.IDImagesCount, int32(1)},
		{"PuLID.IDEmbeddingPath", cString(raw.PulidParams.IDEmbeddingPath), params.PuLID.IDEmbeddingPath},
		{"PuLID.IDWeight", raw.PulidParams.IDWeight, params.PuLID.IDWeight}, {"QwenImageLayers", raw.QwenImageLayers, params.QwenImageLayers},
		{"LoraCount", raw.LoraCount, uint32(1)},
	}
	for _, item := range values {
		if item.got != item.want {
			t.Errorf("%s: got %v, want %v", item.name, item.got, item.want)
		}
	}

	wantSample := SampleParams{
		Guidance:  GuidanceParams{TextCFG: params.CFGScale, ImageCFG: params.ImageCFG, DistilledGuidance: params.DistilledGuidance, SLG: params.SLG},
		Scheduler: params.Scheduler, Method: params.Sampler, Steps: params.Steps, Eta: params.Eta,
		ShiftedTimestep: params.ShiftedTimestep, CustomSigmas: params.CustomSigmas, FlowShift: params.FlowShift, ExtraArgs: params.ExtraSampleArgs,
	}
	if got := sampleParamsFromC(raw.SampleParams); !reflect.DeepEqual(got, wantSample) {
		t.Errorf("SampleParams: got %#v, want %#v", got, wantSample)
	}
	if got := tilingParamsFromC(raw.VAETilingParams); !reflect.DeepEqual(got, params.VAETiling) {
		t.Errorf("VAETiling: got %#v, want %#v", got, params.VAETiling)
	}
	if got := cacheParamsFromC(raw.Cache); !reflect.DeepEqual(got, params.Cache) {
		t.Errorf("Cache: got %#v, want %#v", got, params.Cache)
	}
	if got := hiresParamsFromC(raw.Hires); !reflect.DeepEqual(got, params.Hires) {
		t.Errorf("Hires: got %#v, want %#v", got, params.Hires)
	}
	loraPath := state.refs.keep[len(state.refs.keep)-1]
	if len(state.loras) != 1 || state.loras[0].IsHighNoise != 1 || state.loras[0].Multiplier != params.Loras[0].Multiplier || state.loras[0].Path != uintptr(unsafe.Pointer(loraPath)) || cString(loraPath) != params.Loras[0].Path {
		t.Errorf("Loras: got %#v, want %#v", state.loras, params.Loras)
	}
	assertCImage(t, "InitImage", raw.InitImage, image)
	assertCImage(t, "MaskImage", raw.MaskImage, image)
	assertCImage(t, "ControlImage", raw.ControlImage, refImage)
	assertCImage(t, "IPAdapterImage", raw.IPAdapterImage, image)
	assertCImage(t, "RefImages[0]", state.refImages[0], refImage)
	assertCImage(t, "PhotoMaker.IDImages[0]", state.photoMakerImages[0], photoImage)
}

func TestMarshalVideoParams(t *testing.T) {
	testSetup(t)

	image := testMarshalImage(4)
	refImage := testMarshalImage(5)
	audio := Audio{SampleRate: 48_000, Channels: 2, Data: []float32{0.1, 0.2, 0.3, 0.4}}
	sample := SampleParams{Guidance: GuidanceParams{TextCFG: 1, ImageCFG: 2, DistilledGuidance: 3, SLG: SLGParams{Layers: []int32{2, 5}, LayerStart: 0.2, LayerEnd: 0.8, Scale: 0.7}}, Scheduler: SchedulerSimple, Method: SampleLCM, Steps: 4, Eta: 0.3, ShiftedTimestep: 6, CustomSigmas: []float32{0.8, 0.2}, FlowShift: 1.1, ExtraArgs: "sample"}
	highSample := SampleParams{Guidance: GuidanceParams{TextCFG: 4, ImageCFG: 5, DistilledGuidance: 6, SLG: SLGParams{Layers: []int32{3}, LayerStart: 0.3, LayerEnd: 0.9, Scale: 0.6}}, Scheduler: SchedulerFlux, Method: SampleEulerA, Steps: 5, Eta: 0.4, ShiftedTimestep: 7, CustomSigmas: []float32{0.7}, FlowShift: 1.3, ExtraArgs: "high"}
	params := VideoGenParams{
		Loras: []Lora{{IsHighNoise: true, Multiplier: 1.4, Path: "video-lora"}}, Prompt: "video", NegativePrompt: "negative", ClipSkip: 3,
		InitImage: image, EndImage: refImage, RefImages: []*SDImage{image},
		RefVideos: []RefVideo{{Frames: []*SDImage{refImage}, FPS: 12, Audio: &audio}}, RefAudios: []Audio{audio}, ControlFrames: []*SDImage{image, refImage},
		Width: 256, Height: 144, Sample: sample, HighNoiseSample: highSample, MoeBoundary: 0.45, Strength: 0.65,
		Seed: 99, VideoFrames: 8, FPS: 16, VaceStrength: 0.75,
		VAETiling: TilingParams{Enabled: true, TemporalTiling: true, TileSizeX: 32, TileSizeY: 40, TargetOverlap: 0.2, RelativeSizeX: 0.3, RelativeSizeY: 0.4, ExtraArgs: "video-tiling"},
		Cache:     testCacheParams(), Hires: testHiresParams(), CircularX: true, CircularY: true,
	}

	state, err := marshalVideoParams(params)
	if err != nil {
		t.Fatalf("marshalVideoParams: %v", err)
	}
	raw := state.raw
	values := []struct {
		name string
		got  any
		want any
	}{
		{"Prompt", cString(raw.Prompt), params.Prompt}, {"NegativePrompt", cString(raw.NegativePrompt), params.NegativePrompt},
		{"ClipSkip", raw.ClipSkip, params.ClipSkip}, {"Width", raw.Width, params.Width}, {"Height", raw.Height, params.Height},
		{"MoeBoundary", raw.MoeBoundary, params.MoeBoundary}, {"Strength", raw.Strength, params.Strength}, {"Seed", raw.Seed, params.Seed},
		{"VideoFrames", raw.VideoFrames, params.VideoFrames}, {"FPS", raw.FPS, params.FPS}, {"VaceStrength", raw.VaceStrength, params.VaceStrength},
		{"LoraCount", raw.LoraCount, uint32(1)}, {"RefImagesCount", raw.RefImagesCount, int32(1)},
		{"RefVideosCount", raw.RefVideosCount, int32(1)}, {"RefAudiosCount", raw.RefAudiosCount, int32(1)},
		{"ControlFramesSize", raw.ControlFramesSize, int32(2)}, {"CircularX", raw.CircularX, uint8(1)}, {"CircularY", raw.CircularY, uint8(1)},
	}
	for _, item := range values {
		if item.got != item.want {
			t.Errorf("%s: got %v, want %v", item.name, item.got, item.want)
		}
	}
	if got := sampleParamsFromC(raw.SampleParams); !reflect.DeepEqual(got, sample) {
		t.Errorf("SampleParams: got %#v, want %#v", got, sample)
	}
	if got := sampleParamsFromC(raw.HighNoiseSampleParams); !reflect.DeepEqual(got, highSample) {
		t.Errorf("HighNoiseSampleParams: got %#v, want %#v", got, highSample)
	}
	if got := tilingParamsFromC(raw.VAETilingParams); !reflect.DeepEqual(got, params.VAETiling) {
		t.Errorf("VAETiling: got %#v, want %#v", got, params.VAETiling)
	}
	if got := cacheParamsFromC(raw.Cache); !reflect.DeepEqual(got, params.Cache) {
		t.Errorf("Cache: got %#v, want %#v", got, params.Cache)
	}
	if got := hiresParamsFromC(raw.Hires); !reflect.DeepEqual(got, params.Hires) {
		t.Errorf("Hires: got %#v, want %#v", got, params.Hires)
	}
	if got := audioFromC(&state.refAudios[0]); !reflect.DeepEqual(got, &audio) {
		t.Errorf("RefAudios[0]: got %#v, want %#v", got, &audio)
	}
	if got := audioFromC(&state.refVideos[0].Audio); !reflect.DeepEqual(got, &audio) {
		t.Errorf("RefVideos[0].Audio: got %#v, want %#v", got, &audio)
	}
	assertCImage(t, "InitImage", raw.InitImage, image)
	assertCImage(t, "EndImage", raw.EndImage, refImage)
	assertCImage(t, "RefImages[0]", state.refImages[0], image)
	assertCImage(t, "RefVideos[0].Frames[0]", state.refVideoFrames[0][0], refImage)
	assertCImage(t, "ControlFrames[1]", state.controlFrames[1], refImage)
}

func TestAudioCopyAndValidation(t *testing.T) {
	samples := []float32{0.1, 0.2, 0.3, 0.4}
	raw := cAudio{SampleRate: 44_100, Channels: 2, SampleCount: 2, Data: &samples[0]}
	got := audioFromC(&raw)
	samples[0] = 9
	if got.Data[0] != 0.1 {
		t.Errorf("copied sample: got %v, want 0.1", got.Data[0])
	}
	if got.SampleCount() != 2 {
		t.Errorf("SampleCount: got %d, want 2", got.SampleCount())
	}

	tests := []struct {
		name  string
		audio Audio
	}{
		{"zero channels", Audio{SampleRate: 44_100}},
		{"partial frame", Audio{SampleRate: 44_100, Channels: 2, Data: []float32{1, 2, 3}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := audioToC(&test.audio, "audio"); err == nil {
				t.Fatal("audioToC: got nil error, want validation error")
			}
		})
	}
}

func testMarshalImage(value byte) *SDImage {
	return &SDImage{Width: 1, Height: 1, Channel: 3, Data: []byte{value, value + 1, value + 2}}
}

func testCacheParams() CacheParams {
	return CacheParams{
		Mode: CacheSpectrum, ReuseThreshold: 0.1, StartPercent: 0.2, EndPercent: 0.8, ErrorDecayRate: 0.3,
		UseRelativeThreshold: true, ResetErrorOnCompute: true, FnComputeBlocks: 1, BnComputeBlocks: 2,
		ResidualDiffThreshold: 0.4, MaxWarmupSteps: 3, MaxCachedSteps: 4, MaxContinuousCachedSteps: 5,
		TaylorseerNDerivatives: 6, TaylorseerSkipInterval: 7, SCMMask: "mask", SCMPolicyDynamic: true,
		SpectrumW: 0.5, SpectrumM: 8, SpectrumLambda: 0.6, SpectrumWindowSize: 9,
		SpectrumFlexWindow: 0.7, SpectrumWarmupSteps: 10, SpectrumStopPercent: 0.9,
	}
}

func testHiresParams() HiresParams {
	return HiresParams{Enabled: true, Upscaler: HiresUpscalerLanczos, ModelPath: "upscaler", Scale: 1.5, TargetWidth: 640, TargetHeight: 384, Steps: 12, DenoisingStrength: 0.45, UpscaleTileSize: 96, CustomSigmas: []float32{0.8, 0.3}}
}

func assertCImage(t *testing.T, name string, got cImage, want *SDImage) {
	t.Helper()
	if got.Width != want.Width || got.Height != want.Height || got.Channel != want.Channel {
		t.Errorf("%s dimensions: got %dx%dx%d, want %dx%dx%d", name, got.Width, got.Height, got.Channel, want.Width, want.Height, want.Channel)
		return
	}
	if got.Data == nil || *got.Data != want.Data[0] {
		t.Errorf("%s first byte: got %v, want %d", name, got.Data, want.Data[0])
	}
}
