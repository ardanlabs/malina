package sd

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

var (
	ctxSupportsImageFunc ffi.Fun
	ctxSupportsVideoFunc ffi.Fun
	ctxLoadControlFunc   ffi.Fun
	ctxUnloadControlFunc ffi.Fun
	ctxHasControlFunc    ffi.Fun
	cancelGenerationFunc ffi.Fun

	typeNameFunc               ffi.Fun
	strToTypeFunc              ffi.Fun
	rngTypeNameFunc            ffi.Fun
	strToRngTypeFunc           ffi.Fun
	sampleMethodNameFunc       ffi.Fun
	strToSampleMethodFunc      ffi.Fun
	schedulerNameFunc          ffi.Fun
	strToSchedulerFunc         ffi.Fun
	predictionNameFunc         ffi.Fun
	strToPredictionFunc        ffi.Fun
	previewNameFunc            ffi.Fun
	strToPreviewFunc           ffi.Fun
	loraApplyModeNameFunc      ffi.Fun
	strToLoraApplyModeFunc     ffi.Fun
	hiresUpscalerNameFunc      ffi.Fun
	strToHiresUpscalerFunc     ffi.Fun
	getDefaultSampleMethodFunc ffi.Fun
	getDefaultSchedulerFunc    ffi.Fun

	commitFunc            ffi.Fun
	listDevicesFunc       ffi.Fun
	convertFunc           ffi.Fun
	convertComponentsFunc ffi.Fun
	preprocessCannyFunc   ffi.Fun
	loadImatrixFunc       ffi.Fun
	saveImatrixFunc       ffi.Fun
	enableImatrixFunc     ffi.Fun
	disableImatrixFunc    ffi.Fun
)

func loadExtendedFuncs(lib ffi.Lib) {
	ctxSupportsImageFunc = prepOptional(lib, "sd_ctx_supports_image_generation", &ffi.TypeUint8, &ffi.TypePointer)
	ctxSupportsVideoFunc = prepOptional(lib, "sd_ctx_supports_video_generation", &ffi.TypeUint8, &ffi.TypePointer)
	ctxLoadControlFunc = prepOptional(lib, "sd_ctx_load_control_net", &ffi.TypeUint8, &ffi.TypePointer, &ffi.TypePointer)
	ctxUnloadControlFunc = prepOptional(lib, "sd_ctx_unload_control_net", &ffi.TypeUint8, &ffi.TypePointer)
	ctxHasControlFunc = prepOptional(lib, "sd_ctx_has_control_net", &ffi.TypeUint8, &ffi.TypePointer)
	cancelGenerationFunc = prepOptional(lib, "sd_cancel_generation", &ffi.TypeVoid, &ffi.TypePointer, &ffi.TypeSint32)

	typeNameFunc = prepOptional(lib, "sd_type_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToTypeFunc = prepOptional(lib, "str_to_sd_type", &ffi.TypeSint32, &ffi.TypePointer)
	rngTypeNameFunc = prepOptional(lib, "sd_rng_type_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToRngTypeFunc = prepOptional(lib, "str_to_rng_type", &ffi.TypeSint32, &ffi.TypePointer)
	sampleMethodNameFunc = prepOptional(lib, "sd_sample_method_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToSampleMethodFunc = prepOptional(lib, "str_to_sample_method", &ffi.TypeSint32, &ffi.TypePointer)
	schedulerNameFunc = prepOptional(lib, "sd_scheduler_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToSchedulerFunc = prepOptional(lib, "str_to_scheduler", &ffi.TypeSint32, &ffi.TypePointer)
	predictionNameFunc = prepOptional(lib, "sd_prediction_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToPredictionFunc = prepOptional(lib, "str_to_prediction", &ffi.TypeSint32, &ffi.TypePointer)
	previewNameFunc = prepOptional(lib, "sd_preview_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToPreviewFunc = prepOptional(lib, "str_to_preview", &ffi.TypeSint32, &ffi.TypePointer)
	loraApplyModeNameFunc = prepOptional(lib, "sd_lora_apply_mode_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToLoraApplyModeFunc = prepOptional(lib, "str_to_lora_apply_mode", &ffi.TypeSint32, &ffi.TypePointer)
	hiresUpscalerNameFunc = prepOptional(lib, "sd_hires_upscaler_name", &ffi.TypePointer, &ffi.TypeSint32)
	strToHiresUpscalerFunc = prepOptional(lib, "str_to_sd_hires_upscaler", &ffi.TypeSint32, &ffi.TypePointer)
	getDefaultSampleMethodFunc = prepOptional(lib, "sd_get_default_sample_method", &ffi.TypeSint32, &ffi.TypePointer)
	getDefaultSchedulerFunc = prepOptional(lib, "sd_get_default_scheduler", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypeSint32)

	commitFunc = prepOptional(lib, "sd_commit", &ffi.TypePointer)
	listDevicesFunc = prepOptional(lib, "sd_list_devices", &ffi.TypeUint64, &ffi.TypePointer, &ffi.TypeUint64)
	convertFunc = prepOptional(lib, "convert", &ffi.TypeUint8, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypeUint8)
	convertComponentsFunc = prepOptional(lib, "convert_with_components", &ffi.TypeUint8, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypePointer, &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypeUint8, &ffi.TypeSint32)
	preprocessCannyFunc = prepOptional(lib, "preprocess_canny", &ffi.TypeUint8, &ffiTypeImage, &ffi.TypeFloat, &ffi.TypeFloat, &ffi.TypeFloat, &ffi.TypeFloat, &ffi.TypeUint8)
	loadImatrixFunc = prepOptional(lib, "load_imatrix", &ffi.TypeUint8, &ffi.TypePointer)
	saveImatrixFunc = prepOptional(lib, "save_imatrix", &ffi.TypeVoid, &ffi.TypePointer)
	enableImatrixFunc = prepOptional(lib, "enable_imatrix_collection", &ffi.TypeVoid)
	disableImatrixFunc = prepOptional(lib, "disable_imatrix_collection", &ffi.TypeVoid)
}

func prepOptional(lib ffi.Lib, name string, result *ffi.Type, args ...*ffi.Type) ffi.Fun {
	fn, err := lib.Prep(name, result, args...)
	if err != nil {
		return ffi.Fun{}
	}
	return fn
}

func unsupported(name string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedAPI, name)
}

func callBool(fn ffi.Fun, args ...any) bool {
	var result ffi.Arg
	fn.Call(&result, args...)
	return byte(result) != 0
}

func callEnumName(fn ffi.Fun, value int32, name string) (string, error) {
	if fn == (ffi.Fun{}) {
		return "", unsupported(name)
	}
	var ptr *byte
	fn.Call(unsafe.Pointer(&ptr), unsafe.Pointer(&value))
	if ptr == nil {
		return "", nil
	}
	return utils.BytePtrToString(ptr), nil
}

func callParseEnum(fn ffi.Fun, value string, name string) (int32, error) {
	if fn == (ffi.Fun{}) {
		return 0, unsupported(name)
	}
	ptr, err := utils.BytePtrFromString(value)
	if err != nil {
		return 0, err
	}
	var result ffi.Arg
	fn.Call(&result, unsafe.Pointer(&ptr))
	runtime.KeepAlive(ptr)
	return int32(result), nil
}

// SDTypeName returns the stable-diffusion.cpp name for a tensor type.
func SDTypeName(value SDType) (string, error) {
	return callEnumName(typeNameFunc, int32(value), "sd_type_name")
}

// ParseSDType converts a stable-diffusion.cpp tensor type name.
func ParseSDType(value string) (SDType, error) {
	result, err := callParseEnum(strToTypeFunc, value, "str_to_sd_type")
	return SDType(result), err
}

// RngTypeName returns the stable-diffusion.cpp name for an RNG type.
func RngTypeName(value RngType) (string, error) {
	return callEnumName(rngTypeNameFunc, int32(value), "sd_rng_type_name")
}

// ParseRngType converts a stable-diffusion.cpp RNG type name.
func ParseRngType(value string) (RngType, error) {
	result, err := callParseEnum(strToRngTypeFunc, value, "str_to_rng_type")
	return RngType(result), err
}

// SampleMethodName returns the stable-diffusion.cpp name for a sampler.
func SampleMethodName(value SampleMethod) (string, error) {
	return callEnumName(sampleMethodNameFunc, int32(value), "sd_sample_method_name")
}

// ParseSampleMethod converts a stable-diffusion.cpp sampler name.
func ParseSampleMethod(value string) (SampleMethod, error) {
	result, err := callParseEnum(strToSampleMethodFunc, value, "str_to_sample_method")
	return SampleMethod(result), err
}

// SchedulerName returns the stable-diffusion.cpp name for a scheduler.
func SchedulerName(value Scheduler) (string, error) {
	return callEnumName(schedulerNameFunc, int32(value), "sd_scheduler_name")
}

// ParseScheduler converts a stable-diffusion.cpp scheduler name.
func ParseScheduler(value string) (Scheduler, error) {
	result, err := callParseEnum(strToSchedulerFunc, value, "str_to_scheduler")
	return Scheduler(result), err
}

// PredictionName returns the stable-diffusion.cpp name for a prediction type.
func PredictionName(value Prediction) (string, error) {
	return callEnumName(predictionNameFunc, int32(value), "sd_prediction_name")
}

// ParsePrediction converts a stable-diffusion.cpp prediction name.
func ParsePrediction(value string) (Prediction, error) {
	result, err := callParseEnum(strToPredictionFunc, value, "str_to_prediction")
	return Prediction(result), err
}

// PreviewModeName returns the stable-diffusion.cpp name for a preview mode.
func PreviewModeName(value PreviewMode) (string, error) {
	return callEnumName(previewNameFunc, int32(value), "sd_preview_name")
}

// ParsePreviewMode converts a stable-diffusion.cpp preview mode name.
func ParsePreviewMode(value string) (PreviewMode, error) {
	result, err := callParseEnum(strToPreviewFunc, value, "str_to_preview")
	return PreviewMode(result), err
}

// LoraApplyModeName returns the stable-diffusion.cpp name for a LoRA mode.
func LoraApplyModeName(value LoraApplyMode) (string, error) {
	return callEnumName(loraApplyModeNameFunc, int32(value), "sd_lora_apply_mode_name")
}

// ParseLoraApplyMode converts a stable-diffusion.cpp LoRA mode name.
func ParseLoraApplyMode(value string) (LoraApplyMode, error) {
	result, err := callParseEnum(strToLoraApplyModeFunc, value, "str_to_lora_apply_mode")
	return LoraApplyMode(result), err
}

// HiresUpscalerName returns the stable-diffusion.cpp name for an upscaler.
func HiresUpscalerName(value HiresUpscaler) (string, error) {
	return callEnumName(hiresUpscalerNameFunc, int32(value), "sd_hires_upscaler_name")
}

// ParseHiresUpscaler converts a stable-diffusion.cpp upscaler name.
func ParseHiresUpscaler(value string) (HiresUpscaler, error) {
	result, err := callParseEnum(strToHiresUpscalerFunc, value, "str_to_sd_hires_upscaler")
	return HiresUpscaler(result), err
}

// ContextSupportsImageGeneration reports whether the loaded context can generate images.
func ContextSupportsImageGeneration(ctx Context) (bool, error) {
	if ctx == 0 {
		return false, errors.New("ContextSupportsImageGeneration: nil context")
	}
	if ctxSupportsImageFunc == (ffi.Fun{}) {
		return false, unsupported("sd_ctx_supports_image_generation")
	}
	return callBool(ctxSupportsImageFunc, unsafe.Pointer(&ctx)), nil
}

// ContextSupportsVideoGeneration reports whether the loaded context can generate video.
func ContextSupportsVideoGeneration(ctx Context) (bool, error) {
	if ctx == 0 {
		return false, errors.New("ContextSupportsVideoGeneration: nil context")
	}
	if ctxSupportsVideoFunc == (ffi.Fun{}) {
		return false, unsupported("sd_ctx_supports_video_generation")
	}
	return callBool(ctxSupportsVideoFunc, unsafe.Pointer(&ctx)), nil
}

// LoadControlNet hot-swaps a ControlNet model onto a context.
func LoadControlNet(ctx Context, path string) error {
	if ctx == 0 {
		return errors.New("LoadControlNet: nil context")
	}
	if ctxLoadControlFunc == (ffi.Fun{}) {
		return unsupported("sd_ctx_load_control_net")
	}
	ptr, err := utils.BytePtrFromString(path)
	if err != nil {
		return err
	}
	if !callBool(ctxLoadControlFunc, unsafe.Pointer(&ctx), unsafe.Pointer(&ptr)) {
		return errors.New("sd_ctx_load_control_net failed")
	}
	runtime.KeepAlive(ptr)
	return nil
}

// UnloadControlNet removes the hot-swapped ControlNet model from a context.
func UnloadControlNet(ctx Context) error {
	if ctx == 0 {
		return errors.New("UnloadControlNet: nil context")
	}
	if ctxUnloadControlFunc == (ffi.Fun{}) {
		return unsupported("sd_ctx_unload_control_net")
	}
	if !callBool(ctxUnloadControlFunc, unsafe.Pointer(&ctx)) {
		return errors.New("sd_ctx_unload_control_net failed")
	}
	return nil
}

// HasControlNet reports whether a context currently has a ControlNet model.
func HasControlNet(ctx Context) (bool, error) {
	if ctx == 0 {
		return false, errors.New("HasControlNet: nil context")
	}
	if ctxHasControlFunc == (ffi.Fun{}) {
		return false, unsupported("sd_ctx_has_control_net")
	}
	return callBool(ctxHasControlFunc, unsafe.Pointer(&ctx)), nil
}

// CancelGeneration updates the cancellation state for a context.
func CancelGeneration(ctx Context, mode CancelMode) error {
	if ctx == 0 {
		return errors.New("CancelGeneration: nil context")
	}
	if cancelGenerationFunc == (ffi.Fun{}) {
		return unsupported("sd_cancel_generation")
	}
	value := int32(mode)
	cancelGenerationFunc.Call(nil, unsafe.Pointer(&ctx), unsafe.Pointer(&value))
	return nil
}

// DefaultSampleMethod returns the sampler selected by a loaded context.
func DefaultSampleMethod(ctx Context) (SampleMethod, error) {
	if ctx == 0 {
		return 0, errors.New("DefaultSampleMethod: nil context")
	}
	if getDefaultSampleMethodFunc == (ffi.Fun{}) {
		return 0, unsupported("sd_get_default_sample_method")
	}
	var result ffi.Arg
	getDefaultSampleMethodFunc.Call(&result, unsafe.Pointer(&ctx))
	return SampleMethod(int32(result)), nil
}

// DefaultScheduler returns the scheduler selected for a context and sampler.
func DefaultScheduler(ctx Context, method SampleMethod) (Scheduler, error) {
	if ctx == 0 {
		return 0, errors.New("DefaultScheduler: nil context")
	}
	if getDefaultSchedulerFunc == (ffi.Fun{}) {
		return 0, unsupported("sd_get_default_scheduler")
	}
	value := int32(method)
	var result ffi.Arg
	getDefaultSchedulerFunc.Call(&result, unsafe.Pointer(&ctx), unsafe.Pointer(&value))
	return Scheduler(int32(result)), nil
}

// Commit returns the upstream source commit embedded in the loaded library.
func Commit() (string, error) {
	if commitFunc == (ffi.Fun{}) {
		return "", unsupported("sd_commit")
	}
	var ptr *byte
	commitFunc.Call(unsafe.Pointer(&ptr))
	if ptr == nil {
		return "", nil
	}
	return utils.BytePtrToString(ptr), nil
}

// ListDevices returns every stable-diffusion.cpp backend device.
func ListDevices() ([]Device, error) {
	if listDevicesFunc == (ffi.Fun{}) {
		return nil, unsupported("sd_list_devices")
	}
	var nilBuffer *byte
	var zero uint64
	var required ffi.Arg
	listDevicesFunc.Call(&required, unsafe.Pointer(&nilBuffer), unsafe.Pointer(&zero))
	if required == 0 {
		return nil, nil
	}
	buffer := make([]byte, int(required)+1)
	ptr := &buffer[0]
	size := uint64(len(buffer))
	var written ffi.Arg
	listDevicesFunc.Call(&written, unsafe.Pointer(&ptr), unsafe.Pointer(&size))
	runtime.KeepAlive(buffer)

	lines := strings.Split(strings.TrimSpace(string(buffer[:int(written)])), "\n")
	devices := make([]Device, 0, len(lines))
	for _, line := range lines {
		name, description, _ := strings.Cut(line, "\t")
		devices = append(devices, Device{Name: name, Description: description})
	}
	return devices, nil
}

// Convert converts a checkpoint using stable-diffusion.cpp.
func Convert(params ConvertParams) error {
	if convertFunc == (ffi.Fun{}) {
		return unsupported("convert")
	}
	var refs cStringRefs
	input, err := refs.add(params.InputPath)
	if err != nil {
		return err
	}
	vae, err := refs.add(params.VAEPath)
	if err != nil {
		return err
	}
	output, err := refs.add(params.OutputPath)
	if err != nil {
		return err
	}
	rules, err := refs.add(params.TensorTypeRules)
	if err != nil {
		return err
	}
	typeValue := int32(params.OutputType)
	convertName := boolToU8(params.ConvertName)
	if !callBool(convertFunc, unsafe.Pointer(&input), unsafe.Pointer(&vae), unsafe.Pointer(&output), unsafe.Pointer(&typeValue), unsafe.Pointer(&rules), unsafe.Pointer(&convertName)) {
		return errors.New("convert failed")
	}
	runtime.KeepAlive(refs.keep)
	return nil
}

// ConvertWithComponents converts separate model components into one checkpoint.
func ConvertWithComponents(params ComponentConvertParams) error {
	if convertComponentsFunc == (ffi.Fun{}) {
		return unsupported("convert_with_components")
	}
	var refs cStringRefs
	values := make([]uintptr, 0, 8)
	for _, value := range []string{params.ModelPath, params.ClipLPath, params.ClipGPath, params.T5XXLPath, params.DiffusionModelPath, params.VAEPath, params.OutputPath, params.TensorTypeRules} {
		ptr, err := refs.add(value)
		if err != nil {
			return err
		}
		values = append(values, ptr)
	}
	typeValue := int32(params.OutputType)
	convertName := boolToU8(params.ConvertName)
	threads := params.NThreads
	if !callBool(convertComponentsFunc,
		unsafe.Pointer(&values[0]), unsafe.Pointer(&values[1]), unsafe.Pointer(&values[2]), unsafe.Pointer(&values[3]),
		unsafe.Pointer(&values[4]), unsafe.Pointer(&values[5]), unsafe.Pointer(&values[6]), unsafe.Pointer(&typeValue),
		unsafe.Pointer(&values[7]), unsafe.Pointer(&convertName), unsafe.Pointer(&threads)) {
		return errors.New("convert_with_components failed")
	}
	runtime.KeepAlive(refs.keep)
	return nil
}

// PreprocessCanny applies Canny edge detection to an image in place.
func PreprocessCanny(image *SDImage, params CannyParams) error {
	if preprocessCannyFunc == (ffi.Fun{}) {
		return unsupported("preprocess_canny")
	}
	var raw cImage
	if err := bindCImage(&raw, image, "image"); err != nil {
		return err
	}
	high, low, weak, strong := params.HighThreshold, params.LowThreshold, params.Weak, params.Strong
	inverse := boolToU8(params.Inverse)
	if !callBool(preprocessCannyFunc, unsafe.Pointer(&raw), unsafe.Pointer(&high), unsafe.Pointer(&low), unsafe.Pointer(&weak), unsafe.Pointer(&strong), unsafe.Pointer(&inverse)) {
		return errors.New("preprocess_canny failed")
	}
	runtime.KeepAlive(image)
	return nil
}

// LoadImatrix loads an importance matrix file.
func LoadImatrix(path string) error {
	if loadImatrixFunc == (ffi.Fun{}) {
		return unsupported("load_imatrix")
	}
	ptr, err := utils.BytePtrFromString(path)
	if err != nil {
		return err
	}
	if !callBool(loadImatrixFunc, unsafe.Pointer(&ptr)) {
		return errors.New("load_imatrix failed")
	}
	runtime.KeepAlive(ptr)
	return nil
}

// SaveImatrix saves the current importance matrix collection.
func SaveImatrix(path string) error {
	if saveImatrixFunc == (ffi.Fun{}) {
		return unsupported("save_imatrix")
	}
	ptr, err := utils.BytePtrFromString(path)
	if err != nil {
		return err
	}
	saveImatrixFunc.Call(nil, unsafe.Pointer(&ptr))
	runtime.KeepAlive(ptr)
	return nil
}

// EnableImatrixCollection enables process-wide importance matrix collection.
func EnableImatrixCollection() error {
	if enableImatrixFunc == (ffi.Fun{}) {
		return unsupported("enable_imatrix_collection")
	}
	enableImatrixFunc.Call(nil)
	return nil
}

// DisableImatrixCollection disables process-wide importance matrix collection.
func DisableImatrixCollection() error {
	if disableImatrixFunc == (ffi.Fun{}) {
		return unsupported("disable_imatrix_collection")
	}
	disableImatrixFunc.Call(nil)
	return nil
}
