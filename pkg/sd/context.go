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
// On the LP64 / LLP64 platforms stable-diffusion.cpp targets, the C compiler
// applies natural alignment per type. Go's struct layout follows the same
// rules, so matching the field order with explicit padding produces a
// binary-compatible struct on darwin/arm64, darwin/amd64, linux/amd64 and
// windows/amd64.
//
// Total size: 280 bytes.
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
	UncondDiffusionModelPath    uintptr // 72..80
	EmbeddingsConnectorsPath    uintptr // 80..88
	VAEPath                     uintptr // 88..96
	AudioVAEPath                uintptr // 96..104
	TAESDPath                   uintptr // 104..112
	ControlNetPath              uintptr // 112..120
	IPAdapterPath               uintptr // 120..128
	MotionModulePath            uintptr // 128..136
	Embeddings                  uintptr // 136..144
	EmbeddingCount              uint32  // 144..148
	_                           [4]byte // 148..152
	PhotoMakerPath              uintptr // 152..160
	PulidWeightsPath            uintptr // 160..168
	TensorTypeRules             uintptr // 168..176

	NThreads       int32 // 176..180
	Wtype          int32 // 180..184
	RngType        int32 // 184..188
	SamplerRngType int32 // 188..192
	Prediction     int32 // 192..196
	LoraApplyMode  int32 // 196..200

	EnableMmap            uint8 // 200
	FlashAttn             uint8 // 201
	DiffusionFlashAttn    uint8 // 202
	TaePreviewOnly        uint8 // 203
	DiffusionConvDirect   uint8 // 204
	VAEConvDirect         uint8 // 205
	ForceSDXLVAEConvScale uint8 // 206
	_                     [1]byte
	VaeFormat             int32   // 208..212
	_                     [4]byte // 212..216
	MaxVram               uintptr // 216..224
	StreamLayers          uint8   // 224
	EagerLoad             uint8   // 225
	_                     [6]byte // 226..232
	Backend               uintptr // 232..240
	ParamsBackend         uintptr // 240..248
	SplitMode             uintptr // 248..256
	AutoFit               uint8   // 256
	_                     [7]byte // 257..264
	RPCServers            uintptr // 264..272
	ModelArgs             uintptr // 272..280
}

// cEmbedding mirrors sd_embedding_t. Size: 16 bytes.
type cEmbedding struct {
	Name uintptr
	Path uintptr
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
	UncondDiffusionModelPath    string
	EmbeddingsConnectorsPath    string
	VAEPath                     string
	AudioVAEPath                string
	TAESDPath                   string
	ControlNetPath              string
	IPAdapterPath               string
	MotionModulePath            string
	Embeddings                  []Embedding
	PhotoMakerPath              string
	PulidWeightsPath            string
	TensorTypeRules             string

	// NThreads sets the CPU thread count used by ggml ops. Defaults to the
	// number of physical cores reported by sd_get_num_physical_cores.
	NThreads int32

	Wtype          SDType
	RngType        RngType
	SamplerRngType RngType
	Prediction     Prediction
	LoraApplyMode  LoraApplyMode

	EnableMmap            bool
	FlashAttn             bool
	DiffusionFlashAttn    bool
	TaePreviewOnly        bool
	DiffusionConvDirect   bool
	VAEConvDirect         bool
	ForceSDXLVAEConvScale bool

	// VaeFormat selects the VAE numerical format. Defaults to
	// SDVaeFormatAuto (-1), which lets the C library pick the matching
	// format based on the loaded model checkpoint.
	VaeFormat SDVaeFormat

	// MaxVram sets the GiB budget or backend assignment specification for
	// graph-cut segmented parameter offload. Empty disables it; "-1" selects
	// automatic sizing.
	MaxVram      string
	StreamLayers bool
	EagerLoad    bool

	// Backend selects the ggml backend by name (e.g. "cuda", "metal",
	// "vulkan"). Empty means use the library default.
	Backend       string
	ParamsBackend string
	SplitMode     string
	AutoFit       bool
	RPCServers    string
	ModelArgs     string
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
		NThreads:              raw.NThreads,
		Wtype:                 SDType(raw.Wtype),
		RngType:               RngType(raw.RngType),
		SamplerRngType:        RngType(raw.SamplerRngType),
		Prediction:            Prediction(raw.Prediction),
		LoraApplyMode:         LoraApplyMode(raw.LoraApplyMode),
		EnableMmap:            raw.EnableMmap != 0,
		FlashAttn:             raw.FlashAttn != 0,
		DiffusionFlashAttn:    raw.DiffusionFlashAttn != 0,
		TaePreviewOnly:        raw.TaePreviewOnly != 0,
		DiffusionConvDirect:   raw.DiffusionConvDirect != 0,
		VAEConvDirect:         raw.VAEConvDirect != 0,
		ForceSDXLVAEConvScale: raw.ForceSDXLVAEConvScale != 0,
		VaeFormat:             SDVaeFormat(raw.VaeFormat),
		StreamLayers:          raw.StreamLayers != 0,
		EagerLoad:             raw.EagerLoad != 0,
		AutoFit:               raw.AutoFit != 0,
	}
}

// NewContext loads the configured model files and returns a Context handle
// that must be released with FreeContext when no longer needed. Returns an
// error if the underlying new_sd_ctx returns NULL.
func NewContext(params ContextParams) (Context, error) {
	state, err := marshalContextParams(params)
	if err != nil {
		return 0, err
	}
	raw := state.raw

	clearLastLog()

	rawPtr := &raw
	var handle Context
	newSDCtxFunc.Call(unsafe.Pointer(&handle), unsafe.Pointer(&rawPtr))
	runtime.KeepAlive(state)
	runtime.KeepAlive(params)
	runtime.KeepAlive(&raw)

	if handle == 0 {
		if last := LastError(); last != "" {
			return 0, fmt.Errorf("new_sd_ctx returned NULL: %s", last)
		}
		return 0, errors.New("new_sd_ctx returned NULL (no log message captured; check that ModelPath / DiffusionModelPath exist and are readable)")
	}
	return handle, nil
}

type marshaledContextParams struct {
	raw        cContextParams
	refs       cStringRefs
	embeddings []cEmbedding
}

func marshalContextParams(params ContextParams) (*marshaledContextParams, error) {
	state := &marshaledContextParams{}
	raw := &state.raw
	*raw = cContextParams{
		NThreads:              params.NThreads,
		Wtype:                 int32(params.Wtype),
		RngType:               int32(params.RngType),
		SamplerRngType:        int32(params.SamplerRngType),
		Prediction:            int32(params.Prediction),
		LoraApplyMode:         int32(params.LoraApplyMode),
		EnableMmap:            boolToU8(params.EnableMmap),
		FlashAttn:             boolToU8(params.FlashAttn),
		DiffusionFlashAttn:    boolToU8(params.DiffusionFlashAttn),
		TaePreviewOnly:        boolToU8(params.TaePreviewOnly),
		DiffusionConvDirect:   boolToU8(params.DiffusionConvDirect),
		VAEConvDirect:         boolToU8(params.VAEConvDirect),
		ForceSDXLVAEConvScale: boolToU8(params.ForceSDXLVAEConvScale),
		VaeFormat:             int32(params.VaeFormat),
		StreamLayers:          boolToU8(params.StreamLayers),
		EagerLoad:             boolToU8(params.EagerLoad),
		AutoFit:               boolToU8(params.AutoFit),
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
		{&raw.UncondDiffusionModelPath, params.UncondDiffusionModelPath},
		{&raw.EmbeddingsConnectorsPath, params.EmbeddingsConnectorsPath},
		{&raw.VAEPath, params.VAEPath},
		{&raw.AudioVAEPath, params.AudioVAEPath},
		{&raw.TAESDPath, params.TAESDPath},
		{&raw.ControlNetPath, params.ControlNetPath},
		{&raw.IPAdapterPath, params.IPAdapterPath},
		{&raw.MotionModulePath, params.MotionModulePath},
		{&raw.PhotoMakerPath, params.PhotoMakerPath},
		{&raw.PulidWeightsPath, params.PulidWeightsPath},
		{&raw.TensorTypeRules, params.TensorTypeRules},
		{&raw.MaxVram, params.MaxVram},
		{&raw.Backend, params.Backend},
		{&raw.ParamsBackend, params.ParamsBackend},
		{&raw.SplitMode, params.SplitMode},
		{&raw.RPCServers, params.RPCServers},
		{&raw.ModelArgs, params.ModelArgs},
	} {
		p, err := state.refs.add(m.s)
		if err != nil {
			return nil, err
		}
		*m.dst = p
	}

	state.embeddings = make([]cEmbedding, len(params.Embeddings))
	for i := range params.Embeddings {
		name, err := state.refs.add(params.Embeddings[i].Name)
		if err != nil {
			return nil, err
		}
		path, err := state.refs.add(params.Embeddings[i].Path)
		if err != nil {
			return nil, err
		}
		state.embeddings[i] = cEmbedding{Name: name, Path: path}
	}
	if len(state.embeddings) > 0 {
		raw.Embeddings = uintptr(unsafe.Pointer(&state.embeddings[0]))
		raw.EmbeddingCount = uint32(len(state.embeddings))
	}

	return state, nil
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
	p, err := r.addPointer(s)
	if err != nil || p == nil {
		return 0, err
	}
	return uintptr(unsafe.Pointer(p)), nil
}

func (r *cStringRefs) addPointer(s string) (*byte, error) {
	if s == "" {
		return nil, nil
	}
	p, err := utils.BytePtrFromString(s)
	if err != nil {
		return nil, err
	}
	r.keep = append(r.keep, p)
	return p, nil
}

func boolToU8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
