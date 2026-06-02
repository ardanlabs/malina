package sd

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

// cContextParams mirrors struct sd_ctx_params_t from include/stable-diffusion.h.
//
// The C struct contains 16 string pointers, one struct pointer, a uint32, two
// trailing string pointers, plus bools, ints, enums, and a float interleaved
// in between. On the LP64 / LLP64 platforms stable-diffusion.cpp targets, the
// C compiler applies natural alignment per type. Go's struct layout follows
// the same rules, so matching the field order with explicit _padN blank
// fields produces a binary-compatible struct on darwin/arm64, darwin/amd64,
// linux/amd64 and windows/amd64.
//
// Total size: 224 bytes on darwin/arm64.
type cContextParams struct {
	ModelPath                   uintptr // 0..8
	ClipLPath                   uintptr // 8..16
	ClipGPath                   uintptr // 16..24
	ClipVisionPath              uintptr // 24..32
	T5XXLPath                   uintptr // 32..40
	LLMPath                     uintptr // 40..48
	LLMVisionPath               uintptr // 48..56
	DiffusionModelPath          uintptr // 56..64
	HighNoiseDiffusionModelPath uintptr // 64..72
	EmbeddingsConnectorsPath    uintptr // 72..80
	VAEPath                     uintptr // 80..88
	AudioVAEPath                uintptr // 88..96
	TAESDPath                   uintptr // 96..104
	ControlNetPath              uintptr // 104..112
	Embeddings                  uintptr // 112..120
	EmbeddingCount              uint32  // 120..124
	_                           [4]byte // 124..128
	PhotoMakerPath              uintptr // 128..136
	TensorTypeRules             uintptr // 136..144

	VAEDecodeOnly         uint8 // 144
	FreeParamsImmediately uint8 // 145
	_                     [2]byte
	NThreads              int32 // 148..152

	Wtype          int32 // 152..156
	RngType        int32 // 156..160
	SamplerRngType int32 // 160..164
	Prediction     int32 // 164..168
	LoraApplyMode  int32 // 168..172

	OffloadParamsToCPU    uint8 // 172
	EnableMmap            uint8 // 173
	KeepClipOnCPU         uint8 // 174
	KeepControlNetOnCPU   uint8 // 175
	KeepVAEOnCPU          uint8 // 176
	FlashAttn             uint8 // 177
	DiffusionFlashAttn    uint8 // 178
	TaePreviewOnly        uint8 // 179
	DiffusionConvDirect   uint8 // 180
	VAEConvDirect         uint8 // 181
	CircularX             uint8 // 182
	CircularY             uint8 // 183
	ForceSDXLVAEConvScale uint8 // 184
	ChromaUseDitMask      uint8 // 185
	ChromaUseT5Mask       uint8 // 186
	_                     [1]byte
	ChromaT5MaskPad       int32 // 188..192

	QwenImageZeroCondT uint8 // 192
	_                  [3]byte
	VaeFormat          int32   // 196..200 (added in stable-diffusion.cpp master-666)
	MaxVram            float32 // 200..204
	_                  [4]byte // 204..208 (stream_layers + alignment pad to 8)

	Backend       uintptr // 208..216
	ParamsBackend uintptr // 216..224
}

// The C API takes sd_ctx_params_t by pointer (sd_ctx_params_init,
// new_sd_ctx), so we never need a libffi struct descriptor for it — every
// callsite uses &ffi.TypePointer for the argument and passes a
// pointer-to-pointer-to-struct through ffi.Fun.Call.

// =============================================================================

// ContextParams is the Go-side representation of sd_ctx_params_t. Use
// ContextParamsInit to obtain a value populated with the library defaults,
// then set the file paths and tuning knobs your model requires before passing
// it to NewContext.
type ContextParams struct {
	// File paths. Empty string means "not set".
	ModelPath                   string
	ClipLPath                   string
	ClipGPath                   string
	ClipVisionPath              string
	T5XXLPath                   string
	LLMPath                     string
	LLMVisionPath               string
	DiffusionModelPath          string
	HighNoiseDiffusionModelPath string
	EmbeddingsConnectorsPath    string
	VAEPath                     string
	AudioVAEPath                string
	TAESDPath                   string
	ControlNetPath              string
	PhotoMakerPath              string
	TensorTypeRules             string

	// VAEDecodeOnly skips loading the VAE encoder weights when only image
	// decoding is needed. Defaults to true.
	VAEDecodeOnly bool

	// FreeParamsImmediately releases model parameter memory after each
	// inference step. Defaults to true.
	FreeParamsImmediately bool

	// NThreads sets the CPU thread count used by ggml ops. Defaults to the
	// number of physical cores reported by sd_get_num_physical_cores.
	NThreads int32

	Wtype          SDType
	RngType        RngType
	SamplerRngType RngType
	Prediction     Prediction
	LoraApplyMode  LoraApplyMode

	OffloadParamsToCPU    bool
	EnableMmap            bool
	KeepClipOnCPU         bool
	KeepControlNetOnCPU   bool
	KeepVAEOnCPU          bool
	FlashAttn             bool
	DiffusionFlashAttn    bool
	TaePreviewOnly        bool
	DiffusionConvDirect   bool
	VAEConvDirect         bool
	CircularX             bool
	CircularY             bool
	ForceSDXLVAEConvScale bool

	// ChromaUseDitMask defaults to true. Used by Chroma-family models.
	ChromaUseDitMask bool
	ChromaUseT5Mask  bool
	ChromaT5MaskPad  int32

	QwenImageZeroCondT bool

	// VaeFormat selects the VAE numerical format. Defaults to
	// SDVaeFormatAuto (-1), which lets the C library pick the matching
	// format based on the loaded model checkpoint. Added in
	// stable-diffusion.cpp master-666.
	VaeFormat SDVaeFormat

	// MaxVram caps graph-cut segmented param offload in GiB. 0 disables;
	// -1 means "auto: free VRAM minus 1 GiB".
	MaxVram float32

	// Backend selects the ggml backend by name (e.g. "cuda", "metal",
	// "vulkan"). Empty means use the library default.
	Backend       string
	ParamsBackend string
}

var (
	// SD_API void sd_ctx_params_init(sd_ctx_params_t* sd_ctx_params);
	ctxParamsInitFunc ffi.Fun

	// SD_API sd_ctx_t* new_sd_ctx(const sd_ctx_params_t* sd_ctx_params);
	newSDCtxFunc ffi.Fun

	// SD_API void free_sd_ctx(sd_ctx_t* sd_ctx);
	freeSDCtxFunc ffi.Fun
)

func loadContextFuncs(lib ffi.Lib) error {
	var err error

	if ctxParamsInitFunc, err = lib.Prep("sd_ctx_params_init", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return loadError("sd_ctx_params_init", err)
	}

	if newSDCtxFunc, err = lib.Prep("new_sd_ctx", &ffi.TypePointer, &ffi.TypePointer); err != nil {
		return loadError("new_sd_ctx", err)
	}

	if freeSDCtxFunc, err = lib.Prep("free_sd_ctx", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return loadError("free_sd_ctx", err)
	}

	return nil
}

// ContextParamsInit returns a ContextParams populated with the defaults from
// sd_ctx_params_init. Callers may modify fields before passing to NewContext.
func ContextParamsInit() ContextParams {
	var raw cContextParams
	rawPtr := &raw
	ctxParamsInitFunc.Call(nil, unsafe.Pointer(&rawPtr))

	return ContextParams{
		VAEDecodeOnly:         raw.VAEDecodeOnly != 0,
		FreeParamsImmediately: raw.FreeParamsImmediately != 0,
		NThreads:              raw.NThreads,
		Wtype:                 SDType(raw.Wtype),
		RngType:               RngType(raw.RngType),
		SamplerRngType:        RngType(raw.SamplerRngType),
		Prediction:            Prediction(raw.Prediction),
		LoraApplyMode:         LoraApplyMode(raw.LoraApplyMode),
		OffloadParamsToCPU:    raw.OffloadParamsToCPU != 0,
		EnableMmap:            raw.EnableMmap != 0,
		KeepClipOnCPU:         raw.KeepClipOnCPU != 0,
		KeepControlNetOnCPU:   raw.KeepControlNetOnCPU != 0,
		KeepVAEOnCPU:          raw.KeepVAEOnCPU != 0,
		FlashAttn:             raw.FlashAttn != 0,
		DiffusionFlashAttn:    raw.DiffusionFlashAttn != 0,
		TaePreviewOnly:        raw.TaePreviewOnly != 0,
		DiffusionConvDirect:   raw.DiffusionConvDirect != 0,
		VAEConvDirect:         raw.VAEConvDirect != 0,
		CircularX:             raw.CircularX != 0,
		CircularY:             raw.CircularY != 0,
		ForceSDXLVAEConvScale: raw.ForceSDXLVAEConvScale != 0,
		ChromaUseDitMask:      raw.ChromaUseDitMask != 0,
		ChromaUseT5Mask:       raw.ChromaUseT5Mask != 0,
		ChromaT5MaskPad:       raw.ChromaT5MaskPad,
		QwenImageZeroCondT:    raw.QwenImageZeroCondT != 0,
		VaeFormat:             SDVaeFormat(raw.VaeFormat),
		MaxVram:               raw.MaxVram,
	}
}

// NewContext loads the configured model files and returns a Context handle
// that must be released with FreeContext when no longer needed. Returns an
// error if the underlying new_sd_ctx returns NULL.
func NewContext(params ContextParams) (Context, error) {
	var refs cStringRefs

	raw := cContextParams{
		VAEDecodeOnly:         boolToU8(params.VAEDecodeOnly),
		FreeParamsImmediately: boolToU8(params.FreeParamsImmediately),
		NThreads:              params.NThreads,
		Wtype:                 int32(params.Wtype),
		RngType:               int32(params.RngType),
		SamplerRngType:        int32(params.SamplerRngType),
		Prediction:            int32(params.Prediction),
		LoraApplyMode:         int32(params.LoraApplyMode),
		OffloadParamsToCPU:    boolToU8(params.OffloadParamsToCPU),
		EnableMmap:            boolToU8(params.EnableMmap),
		KeepClipOnCPU:         boolToU8(params.KeepClipOnCPU),
		KeepControlNetOnCPU:   boolToU8(params.KeepControlNetOnCPU),
		KeepVAEOnCPU:          boolToU8(params.KeepVAEOnCPU),
		FlashAttn:             boolToU8(params.FlashAttn),
		DiffusionFlashAttn:    boolToU8(params.DiffusionFlashAttn),
		TaePreviewOnly:        boolToU8(params.TaePreviewOnly),
		DiffusionConvDirect:   boolToU8(params.DiffusionConvDirect),
		VAEConvDirect:         boolToU8(params.VAEConvDirect),
		CircularX:             boolToU8(params.CircularX),
		CircularY:             boolToU8(params.CircularY),
		ForceSDXLVAEConvScale: boolToU8(params.ForceSDXLVAEConvScale),
		ChromaUseDitMask:      boolToU8(params.ChromaUseDitMask),
		ChromaUseT5Mask:       boolToU8(params.ChromaUseT5Mask),
		ChromaT5MaskPad:       params.ChromaT5MaskPad,
		QwenImageZeroCondT:    boolToU8(params.QwenImageZeroCondT),
		VaeFormat:             int32(params.VaeFormat),
		MaxVram:               params.MaxVram,
	}

	for _, m := range []struct {
		dst *uintptr
		s   string
	}{
		{&raw.ModelPath, params.ModelPath},
		{&raw.ClipLPath, params.ClipLPath},
		{&raw.ClipGPath, params.ClipGPath},
		{&raw.ClipVisionPath, params.ClipVisionPath},
		{&raw.T5XXLPath, params.T5XXLPath},
		{&raw.LLMPath, params.LLMPath},
		{&raw.LLMVisionPath, params.LLMVisionPath},
		{&raw.DiffusionModelPath, params.DiffusionModelPath},
		{&raw.HighNoiseDiffusionModelPath, params.HighNoiseDiffusionModelPath},
		{&raw.EmbeddingsConnectorsPath, params.EmbeddingsConnectorsPath},
		{&raw.VAEPath, params.VAEPath},
		{&raw.AudioVAEPath, params.AudioVAEPath},
		{&raw.TAESDPath, params.TAESDPath},
		{&raw.ControlNetPath, params.ControlNetPath},
		{&raw.PhotoMakerPath, params.PhotoMakerPath},
		{&raw.TensorTypeRules, params.TensorTypeRules},
		{&raw.Backend, params.Backend},
		{&raw.ParamsBackend, params.ParamsBackend},
	} {
		p, err := refs.add(m.s)
		if err != nil {
			return 0, err
		}
		*m.dst = p
	}

	clearLastLog()

	rawPtr := &raw
	var handle Context
	newSDCtxFunc.Call(unsafe.Pointer(&handle), unsafe.Pointer(&rawPtr))
	runtime.KeepAlive(refs.keep)
	runtime.KeepAlive(&raw)

	if handle == 0 {
		if last := LastError(); last != "" {
			return 0, fmt.Errorf("new_sd_ctx returned NULL: %s", last)
		}
		return 0, errors.New("new_sd_ctx returned NULL (no log message captured; check that ModelPath / DiffusionModelPath exist and are readable)")
	}
	return handle, nil
}

// FreeContext releases a Context previously returned by NewContext.
func FreeContext(ctx Context) {
	if ctx == 0 {
		return
	}
	freeSDCtxFunc.Call(nil, unsafe.Pointer(&ctx))
}

// =============================================================================

// cStringRefs holds Go byte-slice backings for C-string pointers handed off
// through the FFI boundary. Use add to allocate a null-terminated copy of a
// Go string and obtain the matching uintptr to store in a struct field.
// runtime.KeepAlive(refs.keep) keeps the backings alive across the FFI call.
type cStringRefs struct {
	keep []*byte
}

func (r *cStringRefs) add(s string) (uintptr, error) {
	if s == "" {
		return 0, nil
	}
	p, err := utils.BytePtrFromString(s)
	if err != nil {
		return 0, err
	}
	r.keep = append(r.keep, p)
	return uintptr(unsafe.Pointer(p)), nil
}

func boolToU8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
