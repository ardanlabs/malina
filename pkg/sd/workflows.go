package sd

import (
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
)

//go:embed workflows
var workflowFiles embed.FS

// FaceRefinementParams configures the bundled SD 1.5 ADetailer workflow.
// ModelPath is a complete SD 1.5 checkpoint; DetectorPath is a YOLO detector.
// Inpainting dimensions must be positive multiples of 64.
// Start with FaceRefinementParamsInit. A negative Seed selects a random seed.
type FaceRefinementParams struct {
	ModelPath      string
	DetectorPath   string
	Prompt         string
	NegativePrompt string
	InpaintWidth   int32
	InpaintHeight  int32
	Steps          int32
	Strength       float32
	CFGScale       float32
	Seed           int64
	Confidence     float32
	InputSize      int32
	TopK           int32
	Padding        int32
	MaskBlur       float32
}

// FaceRefinementParamsInit returns default face-refinement settings.
func FaceRefinementParamsInit() FaceRefinementParams {
	return FaceRefinementParams{InpaintWidth: 64, InpaintHeight: 64, Steps: 20, Strength: .75, CFGScale: 7, Seed: -1, Confidence: .3, InputSize: 640, TopK: 4, Padding: 32, MaskBlur: 4}
}

// RefineFaces detects and refines faces using Malina's embedded DGL workflow.
// The context supplies execution settings and caches; params supplies model paths.
// It returns a Go-owned final image with the executed source in GenerationInfo.
// No external DGL file is needed. Like RunDGL, this is a synchronous context call.
func RefineFaces(ctx Context, image SDImage, params FaceRefinementParams) (SDImage, error) {
	if ctx == 0 {
		return SDImage{}, errors.New("refine faces: nil context")
	}
	if err := validateImage(image); err != nil {
		return SDImage{}, err
	}
	if params.ModelPath == "" || params.DetectorPath == "" {
		return SDImage{}, errors.New("refine faces: checkpoint and detector paths are required")
	}
	if params.InpaintWidth <= 0 || params.InpaintHeight <= 0 || params.InpaintWidth%64 != 0 || params.InpaintHeight%64 != 0 || params.Steps <= 0 || !(params.Strength > 0 && params.Strength <= 1) || !finiteFloat(params.CFGScale) || !(params.Confidence >= 0 && params.Confidence <= 1) || params.InputSize <= 0 || params.TopK <= 0 || params.Padding < 0 || !(params.MaskBlur >= 0) || !finiteFloat(params.MaskBlur) {
		return SDImage{}, errors.New("refine faces: invalid sampling or region settings")
	}
	if params.Seed < 0 {
		params.Seed = rand.Int64()
	}
	workflow, err := workflowFiles.ReadFile("workflows/adetailer.dgl")
	if err != nil {
		return SDImage{}, fmt.Errorf("loading face refinement workflow: %w", err)
	}
	source := fmt.Sprintf(string(workflow), strconv.Quote(params.ModelPath), strconv.Quote(params.DetectorPath), strconv.Quote(params.Prompt), strconv.Quote(params.NegativePrompt), params.InpaintWidth, params.InpaintHeight, params.Steps, params.Strength, params.CFGScale, params.Seed, params.Confidence, params.InputSize, params.TopK, params.Padding, params.MaskBlur)
	outputs, err := RunDGL(ctx, source, map[string]SDImage{"image": image})
	if err != nil {
		return SDImage{}, err
	}
	images, err := outputs["image"].Images()
	if err != nil {
		return SDImage{}, err
	}
	result := images[0]
	result.GenerationInfo = source
	return result, nil
}

// AnimateDiffParams configures the bundled SD 1.5 text-to-video workflow.
// ModelPath is a complete SD 1.5 checkpoint; MotionModulePath is an AnimateDiff
// motion module. Start with AnimateDiffParamsInit. A negative Seed is random.
// Width and Height must be positive multiples of 64.
// FPS controls playback, not diffusion. Image/audio conditioning requires custom DGL.
type AnimateDiffParams struct {
	ModelPath        string
	MotionModulePath string
	Prompt           string
	NegativePrompt   string
	Width            int32
	Height           int32
	Frames           int32
	Steps            int32
	Seed             int64
	CFGScale         float32
	FPS              int32
}

// AnimateDiffParamsInit returns conservative text-to-video defaults.
func AnimateDiffParamsInit() AnimateDiffParams {
	return AnimateDiffParams{Width: 128, Height: 128, Frames: 4, Steps: 4, Seed: -1, CFGScale: 7, FPS: 1}
}

// GenerateAnimateDiff generates video using Malina's embedded DGL workflow.
// The context supplies execution settings and caches; params supplies model paths.
// Frames and metadata are Go-owned. No external DGL file is needed.
// Like RunDGL, this is a synchronous context call; audio is not generated.
func GenerateAnimateDiff(ctx Context, params AnimateDiffParams) (VideoGenerationResult, error) {
	if ctx == 0 {
		return VideoGenerationResult{}, errors.New("generate AnimateDiff: nil context")
	}
	if params.ModelPath == "" || params.MotionModulePath == "" {
		return VideoGenerationResult{}, errors.New("generate AnimateDiff: checkpoint and motion-module paths are required")
	}
	if params.Width <= 0 || params.Height <= 0 || params.Width%64 != 0 || params.Height%64 != 0 || params.Frames <= 0 || params.Steps <= 0 || params.FPS <= 0 || !finiteFloat(params.CFGScale) {
		return VideoGenerationResult{}, errors.New("generate AnimateDiff: invalid dimensions or sampling settings")
	}
	if params.Seed < 0 {
		params.Seed = rand.Int64()
	}
	workflow, err := workflowFiles.ReadFile("workflows/animatediff.dgl")
	if err != nil {
		return VideoGenerationResult{}, fmt.Errorf("loading AnimateDiff workflow: %w", err)
	}
	source := fmt.Sprintf(string(workflow), strconv.Quote(params.ModelPath), strconv.Quote(params.MotionModulePath), strconv.Quote(params.Prompt), params.Width, params.Height, params.Frames, params.Steps, strconv.Quote(params.NegativePrompt), params.Seed, params.CFGScale, params.FPS)
	outputs, err := RunDGL(ctx, source, nil)
	if err != nil {
		return VideoGenerationResult{}, err
	}
	frames, err := outputs["video"].Images()
	if err != nil {
		return VideoGenerationResult{}, err
	}
	fps := outputs["fps"]
	if fps.Type != 3 || len(fps.Data) != 8 {
		return VideoGenerationResult{}, errors.New("generate AnimateDiff: expected native I64 FPS")
	}
	for i := range frames {
		frames[i].GenerationInfo = source
	}
	result := VideoGenerationResult{Frames: frames, FPS: int32(binary.LittleEndian.Uint64(fps.Data)), GenerationInfo: source}
	return result, nil
}

// MOSSImageEditParams configures the bundled MOSS-Image Edit HQ workflow.
// Supply MOSS SFT diffusion weights, a Qwen3.5 text encoder and vision projector,
// its tokenizer, the FLUX.2 VAE and the armV5 editing LoRA. Output dimensions
// must be positive multiples of 16. A negative Seed selects a random seed.
type MOSSImageEditParams struct {
	DiffusionModelPath string
	LLMPath            string
	LLMVisionPath      string
	TokenizerPath      string
	VAEPath            string
	LoraPath           string
	LoraWeight         float32
	Prompt             string
	NegativePrompt     string
	Width              int32
	Height             int32
	Steps              int32
	CFGScale           float32
	Seed               int64
}

// MOSSImageEditParamsInit returns the upstream HQ editing defaults.
func MOSSImageEditParamsInit() MOSSImageEditParams {
	return MOSSImageEditParams{LoraWeight: 1, Width: 512, Height: 512, Steps: 20, CFGScale: 3, Seed: -1}
}

// EditMOSSImage edits one RGB reference using Malina's embedded HQ workflow.
// The reference latent is aligned to multiples of 16 and capped at a 1024-pixel
// long edge. The input is not mutated. The result owns its pixels and retains
// replayable DGL source in GenerationInfo. Like RunDGL, this is synchronous;
// the context supplies execution settings and caches, not model paths.
func EditMOSSImage(ctx Context, image SDImage, params MOSSImageEditParams) (SDImage, error) {
	if ctx == 0 {
		return SDImage{}, errors.New("edit MOSS image: nil context")
	}
	if err := validateImage(image); err != nil {
		return SDImage{}, err
	}
	if params.DiffusionModelPath == "" || params.LLMPath == "" || params.LLMVisionPath == "" || params.TokenizerPath == "" || params.VAEPath == "" || params.LoraPath == "" {
		return SDImage{}, errors.New("edit MOSS image: diffusion, text encoder, vision projector, tokenizer, VAE and LoRA paths are required")
	}
	if params.Width <= 0 || params.Height <= 0 || params.Width%16 != 0 || params.Height%16 != 0 || params.Steps <= 0 || !finiteFloat(params.CFGScale) || !finiteFloat(params.LoraWeight) {
		return SDImage{}, errors.New("edit MOSS image: invalid dimensions or sampling settings")
	}
	if params.Seed < 0 {
		params.Seed = rand.Int64()
	}
	workflow, err := workflowFiles.ReadFile("workflows/moss-image-edit-hq.dgl")
	if err != nil {
		return SDImage{}, fmt.Errorf("loading MOSS image editing workflow: %w", err)
	}
	source := fmt.Sprintf(string(workflow), strconv.Quote(params.DiffusionModelPath), strconv.Quote(params.LLMPath), strconv.Quote(params.LLMVisionPath), strconv.Quote(params.TokenizerPath), strconv.Quote(params.VAEPath), strconv.Quote(params.LoraPath), params.LoraWeight, strconv.Quote(params.Prompt), strconv.Quote(params.NegativePrompt), params.Width, params.Height, params.Steps, params.CFGScale, params.Seed)
	outputs, err := RunDGL(ctx, source, map[string]SDImage{"image": image})
	if err != nil {
		return SDImage{}, err
	}
	images, err := outputs["result"].Images()
	if err != nil {
		return SDImage{}, err
	}
	result := images[0]
	result.GenerationInfo = source
	return result, nil
}

func finiteFloat(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}
