package download

import "testing"

func TestCatalogValid(t *testing.T) {
	for _, b := range Catalog() {
		if err := b.Validate(); err != nil {
			t.Errorf("bundle %q: %v", b.Name, err)
		}
	}
}

func TestBundleByName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"sd-1.5", true},
		{"controlnet-canny-sd1.5", true},
		{"controlnet-union-sdxl", true},
		{"realesrgan-x4-anime", true},
		{"adetailer-face-yolov8n", true},
		{"animatediff-sd1.5", true},
		{"wan2.2-s2v-14b", true},
		{"sdxl-base-1.0", true},
		{"llada-image-turbo", true},
		{"z-image-turbo", true},
		{"moss-image-sft", true},
		{"moss-image-edit-hq", true},
		{"z-image-turbo-controlnet-union", true},
		{"qwen-image-2.1", true},
		{"flux2-klein-base-4b", true},
		{"flux2-klein-4b", true},
		{"flux2-klein-9b", true},
		{"  Flux2-Klein-4B  ", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := BundleByName(tt.name)
			if ok != tt.want {
				t.Errorf("BundleByName(%q) = %v, want %v", tt.name, ok, tt.want)
			}
		})
	}
}

func TestBundleNames(t *testing.T) {
	got := BundleNames()
	want := []string{"adetailer-face-yolov8n", "animatediff-sd1.5", "controlnet-canny-sd1.5", "controlnet-union-sdxl", "flux2-klein-4b", "flux2-klein-9b", "flux2-klein-base-4b", "llada-image-turbo", "moss-image-edit-hq", "moss-image-sft", "qwen-image-2.1", "realesrgan-x4-anime", "sd-1.5", "sdxl-base-1.0", "wan2.2-s2v-14b", "z-image-turbo", "z-image-turbo-controlnet-union"}
	if len(got) != len(want) {
		t.Fatalf("BundleNames: got %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("BundleNames[%d]: got %q, want %q", i, got[i], name)
		}
	}
}

// TestBundleShapes pins the structural contract of every bundle in the
// catalog: gated status, license non-empty, expected file count, and the
// (role, filename) pair for each file. A typo in these roles would silently
// break the examples and model-backed CI jobs without this test catching it.
func TestBundleShapes(t *testing.T) {
	type wantFile struct {
		role     FileRole
		filename string
	}
	tests := []struct {
		name    string
		gated   bool
		license string
		files   []wantFile
	}{
		{
			name:  "sd-1.5",
			gated: false,
			files: []wantFile{
				{RoleModel, "v1-5-pruned-emaonly.safetensors"},
			},
		},
		{
			name:  "controlnet-canny-sd1.5",
			gated: false,
			files: []wantFile{
				{RoleModel, "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf"},
				{RoleControlNet, "control_canny-fp16.safetensors"},
			},
		},
		{
			name: "controlnet-union-sdxl",
			files: []wantFile{
				{RoleModel, "sd_xl_base_1.0.safetensors"},
				{RoleControlNet, "diffusion_pytorch_model.safetensors"},
			},
		},
		{
			name:  "realesrgan-x4-anime",
			gated: false,
			files: []wantFile{
				{RoleUpscaler, "RealESRGAN_x4plus_anime_6B.pth"},
			},
		},
		{
			name:  "adetailer-face-yolov8n",
			gated: false,
			files: []wantFile{
				{RoleModel, "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf"},
				{RoleADetailer, "face_yolov8n.safetensors"},
			},
		},
		{
			name:  "animatediff-sd1.5",
			gated: false,
			files: []wantFile{
				{RoleModel, "stable-diffusion-v1-5-pruned-emaonly-Q4_0.gguf"},
				{RoleMotionModule, "mm_sd15_v3.safetensors"},
			},
		},
		{
			name:  "wan2.2-s2v-14b",
			gated: false,
			files: []wantFile{
				{RoleDiffusion, "Wan2.2-S2V-14B-Q4_K_M.gguf"},
				{RoleVAE, "wan_2.1_vae.safetensors"},
				{RoleT5XXL, "umt5-xxl-encoder-Q4_K_M.gguf"},
				{RoleAudioEncoder, "wav2vec2_large_english_fp16.safetensors"},
			},
		},
		{
			name:  "sdxl-base-1.0",
			gated: false,
			files: []wantFile{
				{RoleModel, "sd_xl_base_1.0.safetensors"},
			},
		},
		{
			name:  "llada-image-turbo",
			gated: false,
			files: []wantFile{
				{RoleDiffusion, "llada-image-turbo-q8_0.gguf"},
				{RoleLLM, "llada-image-turbo-text_encoder-q4_K.gguf"},
				{RoleEmbeddingsConn, "llada-image-turbo-connectors-edit.safetensors"},
				{RoleVAE, "llada-image-vae.safetensors"},
				{RoleTokenizer, "tokenizer.json"},
			},
		},
		{
			name:  "z-image-turbo",
			gated: false,
			files: []wantFile{
				{RoleDiffusion, "z_image_turbo-Q4_K.gguf"},
				{RoleLLM, "Qwen3-4B-Instruct-2507-Q4_K_M.gguf"},
				{RoleVAE, "ae.safetensors"},
			},
		},
		{
			name:  "moss-image-sft",
			gated: false,
			files: []wantFile{
				{RoleDiffusion, "MOSS-Image-SFT-Q4_0.gguf"},
				{RoleLLM, "Qwen3.5-9B-Q4_K_M.gguf"},
				{RoleVAE, "flux2-vae.safetensors"},
				{RoleTokenizer, "tokenizer.json"},
			},
		},
		{
			name: "moss-image-edit-hq",
			files: []wantFile{
				{RoleDiffusion, "MOSS-Image-SFT-Q4_0.gguf"},
				{RoleLLM, "Qwen3.5-9B-Q4_K_M.gguf"},
				{RoleVAE, "flux2-vae.safetensors"},
				{RoleTokenizer, "tokenizer.json"},
				{RoleLLMVision, "mmproj-BF16.gguf"},
				{RoleLora, "armV5-moss-edit-lora.safetensors"},
			},
		},
		{
			name:  "z-image-turbo-controlnet-union",
			gated: false,
			files: []wantFile{
				{RoleDiffusion, "z_image_turbo-Q4_K.gguf"},
				{RoleLLM, "Qwen3-4B-Instruct-2507-Q4_K_M.gguf"},
				{RoleVAE, "ae.safetensors"},
				{RoleControlNet, "Z-Image-Turbo-Fun-Controlnet-Union-2.1-2601-8steps.safetensors"},
			},
		},
		{
			name:    "qwen-image-2.1",
			license: "Qwen Research (research/evaluation only; commercial license required for commercial use)",
			files: []wantFile{
				{RoleDiffusion, "qwen-image-2.1-Q4_K_M.gguf"},
				{RoleLLM, "Qwen3-VL-8B-Instruct-UD-Q4_K_XL.gguf"},
				{RoleLLMVision, "mmproj-BF16.gguf"},
				{RoleVAE, "qwen_image_2.1_vae_bf16.safetensors"},
			},
		},
		{
			name:    "flux2-klein-base-4b",
			license: "Apache-2.0",
			files: []wantFile{
				{RoleDiffusion, "flux-2-klein-base-4b-Q4_0.gguf"},
				{RoleVAE, "flux2-vae.safetensors"},
				{RoleLLM, "Qwen3-4B-Q4_K_M.gguf"},
			},
		},
		{
			name:  "flux2-klein-4b",
			gated: true,
			files: []wantFile{
				{RoleDiffusion, "flux-2-klein-4b-Q4_0.gguf"},
				{RoleVAE, "ae.safetensors"},
				{RoleLLM, "Qwen3-4B-Q4_K_M.gguf"},
			},
		},
		{
			name:  "flux2-klein-9b",
			gated: true,
			files: []wantFile{
				{RoleDiffusion, "flux-2-klein-9b-Q4_0.gguf"},
				{RoleVAE, "ae.safetensors"},
				{RoleLLM, "Qwen3-8B-Q4_K_M.gguf"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, ok := BundleByName(tt.name)
			if !ok {
				t.Fatalf("%s: not in catalog", tt.name)
			}
			if b.License == "" {
				t.Errorf("%s: License is empty", tt.name)
			}
			if tt.license != "" && b.License != tt.license {
				t.Errorf("%s: License = %q, want %q", tt.name, b.License, tt.license)
			}
			if b.Gated != tt.gated {
				t.Errorf("%s: Gated = %v, want %v", tt.name, b.Gated, tt.gated)
			}
			if len(b.Files) != len(tt.files) {
				t.Fatalf("%s: got %d files, want %d", tt.name, len(b.Files), len(tt.files))
			}
			for i, want := range tt.files {
				got := b.Files[i]
				if got.Role != want.role {
					t.Errorf("%s: file[%d].Role = %q, want %q", tt.name, i, got.Role, want.role)
				}
				if got.Filename != want.filename {
					t.Errorf("%s: file[%d].Filename = %q, want %q", tt.name, i, got.Filename, want.filename)
				}
			}
		})
	}
}
