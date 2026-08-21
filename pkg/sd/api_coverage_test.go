package sd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jupiterrider/ffi"
)

func TestTargetAPISymbolManifest(t *testing.T) {
	testSetup(t)

	symbols := []struct {
		name string
		fn   ffi.Fun
	}{
		{"sd_version", versionFunc}, {"sd_commit", commitFunc}, {"sd_get_system_info", getSystemInfoFunc},
		{"sd_get_num_physical_cores", getNumPhysicalCoresFunc}, {"sd_set_log_callback", setLogCallbackFunc},
		{"sd_set_progress_callback", setProgressCallbackFunc}, {"sd_set_preview_callback", setPreviewCallbackFunc},
		{"sd_set_backend_eval_callback", setBackendEvalCallbackFunc}, {"sd_type_name", typeNameFunc},
		{"str_to_sd_type", strToTypeFunc}, {"sd_rng_type_name", rngTypeNameFunc}, {"str_to_rng_type", strToRngTypeFunc},
		{"sd_sample_method_name", sampleMethodNameFunc}, {"str_to_sample_method", strToSampleMethodFunc},
		{"sd_scheduler_name", schedulerNameFunc}, {"str_to_scheduler", strToSchedulerFunc},
		{"sd_prediction_name", predictionNameFunc}, {"str_to_prediction", strToPredictionFunc},
		{"sd_preview_name", previewNameFunc}, {"str_to_preview", strToPreviewFunc},
		{"sd_lora_apply_mode_name", loraApplyModeNameFunc}, {"str_to_lora_apply_mode", strToLoraApplyModeFunc},
		{"sd_hires_upscaler_name", hiresUpscalerNameFunc}, {"str_to_sd_hires_upscaler", strToHiresUpscalerFunc},
		{"sd_ctx_params_init", ctxParamsInitFunc}, {"sd_sample_params_init", sampleParamsInitFunc},
		{"sd_cache_params_init", cacheParamsInitFunc}, {"sd_hires_params_init", hiresParamsInitFunc},
		{"sd_img_gen_params_init", imgGenParamsInitFunc}, {"sd_vid_gen_params_init", vidGenParamsInitFunc},
		{"new_sd_ctx", newSDCtxFunc}, {"free_sd_ctx", freeSDCtxFunc},
		{"sd_ctx_supports_image_generation", ctxSupportsImageFunc}, {"sd_ctx_supports_video_generation", ctxSupportsVideoFunc},
		{"sd_ctx_load_control_net", ctxLoadControlFunc}, {"sd_ctx_unload_control_net", ctxUnloadControlFunc},
		{"sd_ctx_has_control_net", ctxHasControlFunc}, {"sd_cancel_generation", cancelGenerationFunc},
		{"sd_get_default_sample_method", getDefaultSampleMethodFunc}, {"sd_get_default_scheduler", getDefaultSchedulerFunc},
		{"generate_image", generateImageFunc}, {"free_sd_images", freeSDImagesFunc},
		{"generate_video", generateVideoFunc}, {"free_sd_audio", freeSDAudioFunc},
		{"new_upscaler_ctx", newUpscalerCtxFunc}, {"free_upscaler_ctx", freeUpscalerCtxFunc},
		{"upscale", upscaleFunc}, {"get_upscale_factor", getUpscaleFactorFunc},
		{"new_adetailer_ctx", newADetailerCtxFunc}, {"free_adetailer_ctx", freeADetailerCtxFunc},
		{"adetail_image", adetailImageFunc}, {"convert", convertFunc}, {"convert_with_components", convertComponentsFunc},
		{"preprocess_canny", preprocessCannyFunc}, {"load_imatrix", loadImatrixFunc}, {"save_imatrix", saveImatrixFunc},
		{"enable_imatrix_collection", enableImatrixFunc}, {"disable_imatrix_collection", disableImatrixFunc},
		{"sd_list_devices", listDevicesFunc},
	}
	if len(symbols) != 59 {
		t.Fatalf("wrapped symbol count: got %d, want 59", len(symbols))
	}
	for _, symbol := range symbols {
		if symbol.fn == (ffi.Fun{}) {
			t.Errorf("target symbol %q was not prepared", symbol.name)
		}
	}

	omitted := []string{"sd_ctx_params_to_str", "sd_sample_params_to_str", "sd_img_gen_params_to_str"}
	if len(omitted) != 3 {
		t.Fatalf("intentional omission count: got %d, want 3", len(omitted))
	}
}

func TestEnumValues(t *testing.T) {
	tests := []struct {
		name string
		got  []int32
		want []int32
	}{
		{"SDType", enumInts([]SDType{SDTypeF32, SDTypeF16, SDTypeQ4_0, SDTypeQ4_1, SDTypeQ5_0, SDTypeQ5_1, SDTypeQ8_0, SDTypeQ8_1, SDTypeQ2K, SDTypeQ3K, SDTypeQ4K, SDTypeQ5K, SDTypeQ6K, SDTypeQ8K, SDTypeIQ2XXS, SDTypeIQ2XS, SDTypeIQ3XXS, SDTypeIQ1S, SDTypeIQ4NL, SDTypeIQ3S, SDTypeIQ2S, SDTypeIQ4XS, SDTypeI8, SDTypeI16, SDTypeI32, SDTypeI64, SDTypeF64, SDTypeIQ1M, SDTypeBF16, SDTypeTQ1_0, SDTypeTQ2_0, SDTypeMXFP4, SDTypeNVFP4, SDTypeQ1_0, SDTypeCount}), []int32{0, 1, 2, 3, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 34, 35, 39, 40, 41, 42}},
		{"RngType", enumInts([]RngType{RngStdDefault, RngCuda, RngCPU, RngTypeCount}), sequence(0, 4)},
		{"SampleMethod", enumInts([]SampleMethod{SampleEuler, SampleEulerA, SampleHeun, SampleDPM2, SampleDPMPP2SA, SampleDPMPP2M, SampleDPMPP2Mv2, SampleIPNDM, SampleIPNDMV, SampleLCM, SampleDDIMTrailing, SampleTCD, SampleResMultistep, SampleRes2S, SampleERSDE, SampleEulerCFGPP, SampleEulerACFGPP, SampleEulerGE, SampleDPMPP2MSDE, SampleDPMPP2MSDEBT, SampleLMS, SampleMethodCount}), sequence(0, 22)},
		{"Scheduler", enumInts([]Scheduler{SchedulerDiscrete, SchedulerKarras, SchedulerExponential, SchedulerAys, SchedulerGits, SchedulerSgmUniform, SchedulerSimple, SchedulerSmoothstep, SchedulerKLOptimal, SchedulerLCM, SchedulerBongTangent, SchedulerLTX2, SchedulerLogitNormal, SchedulerFlux2, SchedulerFlux, SchedulerBeta, SchedulerCount}), sequence(0, 17)},
		{"Prediction", enumInts([]Prediction{PredictionEPS, PredictionV, PredictionEDMV, PredictionFlow, PredictionFluxFlow, PredictionSeFiFlow, PredictionMinit2IFlow, PredictionCount}), sequence(0, 8)},
		{"LogLevel", enumInts([]LogLevel{LogDebug, LogInfo, LogWarn, LogError}), sequence(0, 4)},
		{"SDVaeFormat", enumInts([]SDVaeFormat{SDVaeFormatAuto, SDVaeFormatFlux, SDVaeFormatSD3, SDVaeFormatFlux2, SDVaeFormatWan, SDVaeFormatCount}), []int32{-1, 0, 1, 2, 3, 4}},
		{"LoraApplyMode", enumInts([]LoraApplyMode{LoraApplyAuto, LoraApplyImmediately, LoraApplyAtRuntime, LoraApplyModeCount}), sequence(0, 4)},
		{"PreviewMode", enumInts([]PreviewMode{PreviewNone, PreviewProj, PreviewTAE, PreviewVAE, PreviewCount}), sequence(0, 5)},
		{"CacheMode", enumInts([]CacheMode{CacheDisabled, CacheEasyCache, CacheUCache, CacheDBCache, CacheTaylorseer, CacheDiT, CacheSpectrum}), sequence(0, 7)},
		{"HiresUpscaler", enumInts([]HiresUpscaler{HiresUpscalerNone, HiresUpscalerLatent, HiresUpscalerLatentNearest, HiresUpscalerLatentNearestExact, HiresUpscalerLatentAntialiased, HiresUpscalerLatentBicubic, HiresUpscalerLatentBicubicAntialiased, HiresUpscalerLanczos, HiresUpscalerNearest, HiresUpscalerModel, HiresUpscalerCount}), sequence(0, 11)},
		{"CancelMode", enumInts([]CancelMode{CancelAll, CancelNewLatents, CancelReset}), sequence(0, 3)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(test.got) != len(test.want) {
				t.Fatalf("value count: got %d, want %d", len(test.got), len(test.want))
			}
			for i := range test.want {
				if test.got[i] != test.want[i] {
					t.Errorf("value %d: got %d, want %d", i, test.got[i], test.want[i])
				}
			}
		})
	}
}

func TestEnumNameRoundTrips(t *testing.T) {
	testSetup(t)

	assertEnumRoundTrips(t, "SDType", []SDType{SDTypeF32, SDTypeF16, SDTypeQ4_0, SDTypeQ4_1, SDTypeQ5_0, SDTypeQ5_1, SDTypeQ8_0, SDTypeQ8_1, SDTypeQ2K, SDTypeQ3K, SDTypeQ4K, SDTypeQ5K, SDTypeQ6K, SDTypeQ8K, SDTypeIQ2XXS, SDTypeIQ2XS, SDTypeIQ3XXS, SDTypeIQ1S, SDTypeIQ4NL, SDTypeIQ3S, SDTypeIQ2S, SDTypeIQ4XS, SDTypeI8, SDTypeI16, SDTypeI32, SDTypeI64, SDTypeF64, SDTypeIQ1M, SDTypeBF16, SDTypeTQ1_0, SDTypeTQ2_0, SDTypeMXFP4, SDTypeNVFP4, SDTypeQ1_0}, SDTypeName, ParseSDType)
	assertEnumRoundTrips(t, "RngType", []RngType{RngStdDefault, RngCuda, RngCPU}, RngTypeName, ParseRngType)
	assertEnumRoundTrips(t, "SampleMethod", []SampleMethod{SampleEuler, SampleEulerA, SampleHeun, SampleDPM2, SampleDPMPP2SA, SampleDPMPP2M, SampleDPMPP2Mv2, SampleIPNDM, SampleIPNDMV, SampleLCM, SampleDDIMTrailing, SampleTCD, SampleResMultistep, SampleRes2S, SampleERSDE, SampleEulerCFGPP, SampleEulerACFGPP, SampleEulerGE, SampleDPMPP2MSDE, SampleDPMPP2MSDEBT, SampleLMS}, SampleMethodName, ParseSampleMethod)
	assertEnumRoundTrips(t, "Scheduler", []Scheduler{SchedulerDiscrete, SchedulerKarras, SchedulerExponential, SchedulerAys, SchedulerGits, SchedulerSgmUniform, SchedulerSimple, SchedulerSmoothstep, SchedulerKLOptimal, SchedulerLCM, SchedulerBongTangent, SchedulerLTX2, SchedulerLogitNormal, SchedulerFlux2, SchedulerFlux, SchedulerBeta}, SchedulerName, ParseScheduler)
	assertEnumRoundTrips(t, "Prediction", []Prediction{PredictionEPS, PredictionV, PredictionEDMV, PredictionFlow, PredictionFluxFlow, PredictionSeFiFlow, PredictionMinit2IFlow}, PredictionName, ParsePrediction)
	assertEnumRoundTrips(t, "PreviewMode", []PreviewMode{PreviewNone, PreviewProj, PreviewTAE, PreviewVAE}, PreviewModeName, ParsePreviewMode)
	assertEnumRoundTrips(t, "LoraApplyMode", []LoraApplyMode{LoraApplyAuto, LoraApplyImmediately, LoraApplyAtRuntime}, LoraApplyModeName, ParseLoraApplyMode)
	assertEnumRoundTrips(t, "HiresUpscaler", []HiresUpscaler{HiresUpscalerNone, HiresUpscalerLatent, HiresUpscalerLatentNearest, HiresUpscalerLatentNearestExact, HiresUpscalerLatentAntialiased, HiresUpscalerLatentBicubic, HiresUpscalerLatentBicubicAntialiased, HiresUpscalerLanczos, HiresUpscalerNearest, HiresUpscalerModel}, HiresUpscalerName, ParseHiresUpscaler)
}

func TestOptionalAPIsReturnUnsupportedSentinel(t *testing.T) {
	testSetup(t)

	image := testMarshalImage(1)
	tests := []struct {
		name string
		fn   *ffi.Fun
		call func() error
	}{
		{"SDTypeName", &typeNameFunc, func() error { _, err := SDTypeName(SDTypeF32); return err }},
		{"ParseSDType", &strToTypeFunc, func() error { _, err := ParseSDType("f32"); return err }},
		{"ContextSupportsImageGeneration", &ctxSupportsImageFunc, func() error { _, err := ContextSupportsImageGeneration(1); return err }},
		{"ContextSupportsVideoGeneration", &ctxSupportsVideoFunc, func() error { _, err := ContextSupportsVideoGeneration(1); return err }},
		{"LoadControlNet", &ctxLoadControlFunc, func() error { return LoadControlNet(1, "control") }},
		{"UnloadControlNet", &ctxUnloadControlFunc, func() error { return UnloadControlNet(1) }},
		{"HasControlNet", &ctxHasControlFunc, func() error { _, err := HasControlNet(1); return err }},
		{"CancelGeneration", &cancelGenerationFunc, func() error { return CancelGeneration(1, CancelReset) }},
		{"DefaultSampleMethod", &getDefaultSampleMethodFunc, func() error { _, err := DefaultSampleMethod(1); return err }},
		{"DefaultScheduler", &getDefaultSchedulerFunc, func() error { _, err := DefaultScheduler(1, SampleEuler); return err }},
		{"Commit", &commitFunc, func() error { _, err := Commit(); return err }},
		{"ListDevices", &listDevicesFunc, func() error { _, err := ListDevices(); return err }},
		{"Convert", &convertFunc, func() error { return Convert(ConvertParams{}) }},
		{"ConvertWithComponents", &convertComponentsFunc, func() error { return ConvertWithComponents(ComponentConvertParams{}) }},
		{"PreprocessCanny", &preprocessCannyFunc, func() error { return PreprocessCanny(image, CannyParams{}) }},
		{"LoadImatrix", &loadImatrixFunc, func() error { return LoadImatrix("matrix") }},
		{"SaveImatrix", &saveImatrixFunc, func() error { return SaveImatrix("matrix") }},
		{"EnableImatrixCollection", &enableImatrixFunc, EnableImatrixCollection},
		{"DisableImatrixCollection", &disableImatrixFunc, DisableImatrixCollection},
		{"SetPreviewCallback", &setPreviewCallbackFunc, func() error { return SetPreviewCallback(nil, PreviewNone, 1, false, false) }},
		{"SetBackendEvalCallback", &setBackendEvalCallbackFunc, func() error { return SetBackendEvalCallback(nil) }},
		{"VideoGenParamsInit", &vidGenParamsInitFunc, func() error { _, err := VideoGenParamsInit(); return err }},
		{"GenerateVideo", &generateVideoFunc, func() error { _, _, err := GenerateVideo(1, VideoGenParams{}); return err }},
		{"NewUpscalerContext", &newUpscalerCtxFunc, func() error { _, err := NewUpscalerContext("model", false, 1, 0, "", ""); return err }},
		{"GetUpscaleFactor", &getUpscaleFactorFunc, func() error { _, err := GetUpscaleFactor(1); return err }},
		{"Upscale", &upscaleFunc, func() error { _, err := Upscale(1, image, 2); return err }},
		{"NewADetailerContext", &newADetailerCtxFunc, func() error { _, err := NewADetailerContext("model", 1, "", ""); return err }},
		{"ADetailImage", &adetailImageFunc, func() error { _, err := ADetailImage(1, 1, image, ADetailerParams{}, ImgGenParams{}); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := *test.fn
			*test.fn = ffi.Fun{}
			defer func() { *test.fn = previous }()

			if err := test.call(); !errors.Is(err, ErrUnsupportedAPI) {
				t.Errorf("error: got %v, want ErrUnsupportedAPI", err)
			}
		})
	}
}

func enumInts[T ~int32](values []T) []int32 {
	result := make([]int32, len(values))
	for i, value := range values {
		result[i] = int32(value)
	}
	return result
}

func sequence(first int32, count int) []int32 {
	values := make([]int32, count)
	for i := range count {
		values[i] = first + int32(i)
	}
	return values
}

func assertEnumRoundTrips[T ~int32](t *testing.T, name string, values []T, nativeName func(T) (string, error), parse func(string) (T, error)) {
	t.Helper()
	for _, value := range values {
		t.Run(fmt.Sprintf("%s/%d", name, value), func(t *testing.T) {
			text, err := nativeName(value)
			if err != nil {
				t.Fatalf("native name for %d: %v", value, err)
			}
			if text == "" {
				t.Fatalf("native name for %d is empty", value)
			}
			got, err := parse(text)
			if err != nil {
				t.Fatalf("parse %q: %v", text, err)
			}
			if got != value {
				t.Errorf("round trip %q: got %d, want %d", text, got, value)
			}
		})
	}
}
