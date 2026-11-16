package sd

import (
	"runtime"
	"strings"
	"unsafe"

	"github.com/ardanlabs/malina/pkg/utils"
	"github.com/jupiterrider/ffi"
)

// ContextParams configures model loading. Start with ContextParamsInit.
// Fields equal to its defaults are left unset in native options, allowing model
// recipes to choose defaults. Options explicitly overrides fields, including
// default-looking values; a nil option value requests native unset.
type ContextParams struct {
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
	AudioEncoderPath            string
	TAESDPath                   string
	ControlNetPath              string
	IPAdapterPath               string
	MotionModulePath            string
	Embeddings                  []Embedding
	PhotoMakerPath              string
	PulidWeightsPath            string
	TensorTypeRules             string
	NThreads                    int32
	Wtype                       SDType
	RngType                     RngType
	SamplerRngType              RngType
	Prediction                  Prediction
	LoraApplyMode               LoraApplyMode
	EnableMmap                  bool
	FlashAttn                   bool
	DiffusionFlashAttn          bool
	TaePreviewOnly              bool
	DiffusionConvDirect         bool
	VAEConvDirect               bool
	ForceSDXLVAEConvScale       bool
	VaeFormat                   SDVaeFormat
	MaxVram                     string
	DisablePrefetch             bool
	EagerLoad                   bool
	DisableSegmentedCompute     bool
	Backend                     string
	ParamsBackend               string
	SplitMode                   string
	AutoFit                     bool
	RPCServers                  string
	ModelArgs                   string
	LinearScale                 float32
	AttnScale                   float32
	Tokenizer                   string
	SageAttn                    bool
	ConditioningCacheSize       int32
	// BatchedCFG is forced off for Metal until upstream's batched group
	// normalization is correct. This also overrides Options["batched_cfg"].
	BatchedCFG     bool
	FlashAttnMode  string
	ConvDirectMode string
	Options        Options
}

// cEmbedding mirrors the live sd_embedding_t descriptor.
type cEmbedding struct{ Name, Path uintptr }

var newSDCtxFunc, freeSDCtxFunc ffi.Fun

func loadContextFuncs(lib ffi.Lib) error {
	var err error
	if err = loadOptionFuncs(lib); err != nil {
		return err
	}
	if newSDCtxFunc, err = lib.Prep("sd_ctx_create", &ffi.TypeSint32, &ffi.TypePointer, &ffi.TypePointer); err != nil {
		return loadError("sd_ctx_create", err)
	}
	if freeSDCtxFunc, err = lib.Prep("sd_ctx_free", &ffi.TypeVoid, &ffi.TypePointer); err != nil {
		return loadError("sd_ctx_free", err)
	}
	return nil
}

// ContextParamsInit returns Go compatibility defaults. Native options, rather
// than removed public C parameter layouts, own inference defaults.
func ContextParamsInit() ContextParams {
	return ContextParams{NThreads: NumPhysicalCores(), Wtype: SDTypeCount, RngType: RngCuda, SamplerRngType: RngTypeCount, Prediction: PredictionCount, VaeFormat: SDVaeFormatAuto, AutoFit: true, ConditioningCacheSize: 4, BatchedCFG: true}
}

// NewContext loads configured models. Release the returned handle with FreeContext.
func NewContext(params ContextParams) (Context, error) {
	clearLastLog()
	o, err := newOptions("ctx")
	if err != nil {
		return 0, err
	}
	defer o.free()
	if err := o.fields(params, ContextParamsInit(), ""); err != nil {
		return 0, err
	}
	backend := params.Backend
	if value, ok := params.Options["backend"].(string); ok {
		backend = value
	}
	if metalDiffusionBackend(backend) {
		if err := o.set("batched_cfg", false); err != nil {
			return 0, err
		}
	}
	var handle Context
	out := &handle
	if err := statusCall(newSDCtxFunc, unsafe.Pointer(&o.ptr), unsafe.Pointer(&out)); err != nil {
		return 0, err
	}
	return handle, nil
}

func metalDiffusionBackend(assignment string) bool {
	var fallback, diffusion string
	for part := range strings.SplitSeq(strings.ToLower(assignment), ",") {
		key, value, found := strings.Cut(part, "=")
		if !found {
			if name := strings.TrimSpace(part); name != "" {
				fallback = name
			}
			continue
		}
		key = strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(key), "-", ""), "_", "")
		switch key {
		case "all", "default", "*":
			fallback = strings.TrimSpace(value)
		case "diffusion", "model", "unet", "dit":
			diffusion = strings.TrimSpace(value)
		}
	}
	if diffusion == "" {
		diffusion = fallback
	}
	switch diffusion {
	case "", "auto", "default", "gpu":
		return runtime.GOOS == "darwin"
	default:
		return strings.HasPrefix(diffusion, "metal") || strings.HasPrefix(diffusion, "mtl")
	}
}

// FreeContext releases a context. Zero is ignored.
func FreeContext(ctx Context) {
	if ctx == 0 {
		return
	}
	freeSDCtxFunc.Call(nil, unsafe.Pointer(&ctx))
	clearPromptImage(ctx)
	progressMu.Lock()
	delete(generationProgressHandlers, ctx)
	progressMu.Unlock()
}

type cStringRefs struct{ keep []*byte }

func (r *cStringRefs) add(s string) (uintptr, error) {
	p, err := r.addPointer(s)
	return uintptr(unsafe.Pointer(p)), err
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
