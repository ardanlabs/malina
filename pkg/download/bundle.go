package download

import (
	"fmt"
	"slices"
	"strings"
)

// FileRole identifies which slot in sd.ContextParams a bundle file populates.
// kronk (and any downstream consumer) reads the bundle manifest and uses
// these roles to wire each downloaded file into the matching ContextParams
// field.
type FileRole string

// File roles map 1:1 onto sd.ContextParams path fields.
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
)

// BundleFile describes a single file inside a bundle.
type BundleFile struct {
	Role     FileRole // which ContextParams slot this file populates
	Filename string   // local filename to write under the bundle directory
	URL      string   // direct HTTPS download URL (resolve via Hugging Face)
	Size     string   // human-readable size for catalog listings
}

// Bundle is a curated set of files required to construct one
// stable-diffusion.cpp Context. Use Get to download a bundle.
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
// These three were chosen to span the practical complexity range:
//   - sd-1.5: smallest, single-file, fully open
//   - sdxl-base-1.0: mainstream quality baseline, single-file
//   - flux2-klein-9b: multi-file (diffusion + VAE + LLM), license-gated
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
					URL:      "https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5/resolve/main/v1-5-pruned-emaonly.safetensors",
					Size:     "4.3 GB",
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
