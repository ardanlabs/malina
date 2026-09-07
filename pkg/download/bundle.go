package download

import (
	"fmt"
	"slices"
	"strings"
)

// FileRole identifies how a bundle file is consumed by pkg/sd. Most roles map
// to ContextParams fields; standalone tools use their matching constructors.
// Kronk and other downstream consumers read these roles from bundle manifests.
type FileRole string

// File roles identify ContextParams fields and standalone model constructors.
const (
	RoleModel          FileRole = "model"           // ModelPath  (all-in-one checkpoints: SD 1.x/2.x/SDXL)
	RoleDiffusion      FileRole = "diffusion"       // DiffusionModelPath
	RoleVAE            FileRole = "vae"             // VAEPath
	RoleClipL          FileRole = "clip_l"          // ClipLPath
	RoleClipG          FileRole = "clip_g"          // ClipGPath
	RoleT5XXL          FileRole = "t5xxl"           // T5XXLPath
	RoleLLM            FileRole = "llm"             // LLMPath
	RoleLLMVision      FileRole = "llm_vision"      // LLMVisionPath
	RoleControlNet     FileRole = "control_net"     // ControlNetPath
	RoleTAESD          FileRole = "taesd"           // TAESDPath
	RolePhotoMaker     FileRole = "photo_maker"     // PhotoMakerPath
	RoleClipVision     FileRole = "clip_vision"     // ClipVisionPath
	RoleHighNoise      FileRole = "high_noise"      // HighNoiseDiffusionModelPath
	RoleEmbeddingsConn FileRole = "embeddings_conn" // EmbeddingsConnectorsPath
	RoleMotionModule   FileRole = "motion_module"   // MotionModulePath
	RoleUpscaler       FileRole = "upscaler"        // NewUpscalerContext
	RoleADetailer      FileRole = "adetailer"       // NewADetailerContext
)

// BundleFile describes a single file inside a bundle.
type BundleFile struct {
	Role     FileRole // how pkg/sd consumes this file
	Filename string   // local filename to write under the bundle directory
	URL      string   // direct HTTPS download URL (resolve via Hugging Face)
	Size     string   // human-readable size for catalog listings
}

// Bundle is a curated set of files required for one stable-diffusion.cpp
// model workflow. Use GetBundle to download a bundle.
type Bundle struct {
	Name        string
	Description string
	License     string
	// Gated indicates the upstream Hugging Face repo gates downloads
	// behind an "Agree to license" click-through. Users must accept the
	// terms in their browser and set HF_TOKEN before Get will succeed.
	Gated bool
	Files []BundleFile
}

// Catalog returns the curated set of bundles malina ships with.
//
// The catalog covers image generation, ControlNet, upscaling, ADetailer,
// AnimateDiff video generation, SDXL, and multi-file FLUX.2 pipelines.
func Catalog() []Bundle {
	return []Bundle{
		{
			Name:        "sd-1.5",
			Description: "Stable Diffusion v1.5 — classic baseline model, single safetensors file (~4.3 GB).",
			License:     "CreativeML Open RAIL-M",
			Files: []BundleFile{
				{
					Role:     RoleModel,
					Filename: "v1-5-pruned-emaonly.safetensors",
					URL:      "https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5/resolve/451f4fe16113bff5a5d2269ed5ad43b0592e9a14/v1-5-pruned-emaonly.safetensors",
					Size:     "4.3 GB",
				},
			},
		},
		{
			Name:        "controlnet-canny-sd1.5",
			Description: "Quantized SD 1.5 with fp16 Canny ControlNet conditioning. Two files (~2.3 GB total).",
			License:     "CreativeML Open RAIL-M / OpenRAIL",
			Files: []BundleFile{
				{
					Role:     RoleModel,
					Filename: "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf",
					URL:      "https://huggingface.co/second-state/stable-diffusion-v1-5-GGUF/resolve/031b5f5df991f511b3f5fa8fed6d99048ababb69/stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf",
					Size:     "1.6 GB",
				},
				{
					Role:     RoleControlNet,
					Filename: "control_canny-fp16.safetensors",
					URL:      "https://huggingface.co/webui/ControlNet-modules-safetensors/resolve/5194dff6fe5e3d26310a12c527eae8bc02d3a482/control_canny-fp16.safetensors",
					Size:     "723 MB",
				},
			},
		},
		{
			Name:        "realesrgan-x4-anime",
			Description: "Real-ESRGAN 4x anime image upscaler (~18 MB).",
			License:     "BSD-3-Clause",
			Files: []BundleFile{
				{
					Role:     RoleUpscaler,
					Filename: "RealESRGAN_x4plus_anime_6B.pth",
					URL:      "https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.2.4/RealESRGAN_x4plus_anime_6B.pth",
					Size:     "18 MB",
				},
			},
		},
		{
			Name:        "adetailer-face-yolov8n",
			Description: "Quantized SD 1.5 with a converted YOLOv8n ADetailer face detector. Two files (~1.6 GB total).",
			License:     "CreativeML Open RAIL-M / AGPL-3.0",
			Files: []BundleFile{
				{
					Role:     RoleModel,
					Filename: "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf",
					URL:      "https://huggingface.co/second-state/stable-diffusion-v1-5-GGUF/resolve/031b5f5df991f511b3f5fa8fed6d99048ababb69/stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf",
					Size:     "1.6 GB",
				},
				{
					Role:     RoleADetailer,
					Filename: "face_yolov8n.safetensors",
					URL:      "https://huggingface.co/exeterminal/adetailer-yolov8-safetensors/resolve/07ca0bd47f67955bf49e26c24d5aff8d161d81f2/face_yolov8n.safetensors",
					Size:     "6 MB",
				},
			},
		},
		{
			Name:        "animatediff-sd1.5",
			Description: "Quantized SD 1.5 with the fp16 AnimateDiff v3 motion module. Two files (~2.4 GB total).",
			License:     "CreativeML Open RAIL-M / Apache-2.0",
			Files: []BundleFile{
				{
					Role:     RoleModel,
					Filename: "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf",
					URL:      "https://huggingface.co/second-state/stable-diffusion-v1-5-GGUF/resolve/031b5f5df991f511b3f5fa8fed6d99048ababb69/stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf",
					Size:     "1.6 GB",
				},
				{
					Role:     RoleMotionModule,
					Filename: "mm_sd15_v3.safetensors",
					URL:      "https://huggingface.co/conrevo/AnimateDiff-A1111/resolve/aa4a0ef5bd366a0ec898e7a64b6fc0f612e37444/motion_module/mm_sd15_v3.safetensors",
					Size:     "837 MB",
				},
			},
		},
		{
			Name:        "sdxl-base-1.0",
			Description: "Stable Diffusion XL base 1.0 — mainstream high-quality baseline, single safetensors file (~6.9 GB).",
			License:     "CreativeML Open RAIL++-M",
			Files: []BundleFile{
				{
					Role:     RoleModel,
					Filename: "sd_xl_base_1.0.safetensors",
					URL:      "https://huggingface.co/stabilityai/stable-diffusion-xl-base-1.0/resolve/main/sd_xl_base_1.0.safetensors",
					Size:     "6.9 GB",
				},
			},
		},
		{
			Name:        "flux2-klein-4b",
			Description: "FLUX.2 [klein] 4B — compact 4-step distilled model with Qwen3-4B text encoder. Three files (~5.3 GB total).",
			License:     "FLUX Non-Commercial",
			Gated:       true,
			Files: []BundleFile{
				{
					Role:     RoleDiffusion,
					Filename: "flux-2-klein-4b-Q4_0.gguf",
					URL:      "https://huggingface.co/leejet/FLUX.2-klein-4B-GGUF/resolve/main/flux-2-klein-4b-Q4_0.gguf",
					Size:     "2.5 GB",
				},
				{
					Role:     RoleVAE,
					Filename: "ae.safetensors",
					URL:      "https://huggingface.co/black-forest-labs/FLUX.2-dev/resolve/main/ae.safetensors",
					Size:     "335 MB",
				},
				{
					Role:     RoleLLM,
					Filename: "Qwen3-4B-Q4_K_M.gguf",
					URL:      "https://huggingface.co/unsloth/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf",
					Size:     "2.5 GB",
				},
			},
		},
		{
			Name:        "flux2-klein-9b",
			Description: "FLUX.2 [klein] 9B — flagship 4-step distilled model with Qwen3-8B text encoder. Three files (~16 GB total).",
			License:     "FLUX Non-Commercial",
			Gated:       true,
			Files: []BundleFile{
				{
					Role:     RoleDiffusion,
					Filename: "flux-2-klein-9b-Q4_0.gguf",
					URL:      "https://huggingface.co/leejet/FLUX.2-klein-9B-GGUF/resolve/main/flux-2-klein-9b-Q4_0.gguf",
					Size:     "5.6 GB",
				},
				{
					Role:     RoleVAE,
					Filename: "ae.safetensors",
					URL:      "https://huggingface.co/black-forest-labs/FLUX.2-dev/resolve/main/ae.safetensors",
					Size:     "335 MB",
				},
				{
					Role:     RoleLLM,
					Filename: "Qwen3-8B-Q4_K_M.gguf",
					URL:      "https://huggingface.co/unsloth/Qwen3-8B-GGUF/resolve/main/Qwen3-8B-Q4_K_M.gguf",
					Size:     "5.0 GB",
				},
			},
		},
	}
}

// BundleByName returns the catalog entry for a short name, or false if no
// such bundle exists.
func BundleByName(name string) (Bundle, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, b := range Catalog() {
		if b.Name == name {
			return b, true
		}
	}
	return Bundle{}, false
}

// BundleNames returns the catalog bundle names in stable sorted order.
func BundleNames() []string {
	out := make([]string, 0, len(Catalog()))
	for _, b := range Catalog() {
		out = append(out, b.Name)
	}
	slices.Sort(out)
	return out
}

// Validate returns an error if any catalog entry has missing fields or
// duplicate roles. Intended for use from a unit test.
func (b Bundle) Validate() error {
	if b.Name == "" {
		return fmt.Errorf("bundle: missing Name")
	}
	if b.License == "" {
		return fmt.Errorf("bundle %q: missing License", b.Name)
	}
	if len(b.Files) == 0 {
		return fmt.Errorf("bundle %q: no files", b.Name)
	}
	seen := make(map[FileRole]struct{}, len(b.Files))
	for _, f := range b.Files {
		if f.Role == "" {
			return fmt.Errorf("bundle %q: file %q missing Role", b.Name, f.Filename)
		}
		if f.Filename == "" {
			return fmt.Errorf("bundle %q: file with role %q missing Filename", b.Name, f.Role)
		}
		if f.URL == "" {
			return fmt.Errorf("bundle %q: file %q missing URL", b.Name, f.Filename)
		}
		if _, dup := seen[f.Role]; dup {
			return fmt.Errorf("bundle %q: duplicate role %q", b.Name, f.Role)
		}
		seen[f.Role] = struct{}{}
	}
	return nil
}
