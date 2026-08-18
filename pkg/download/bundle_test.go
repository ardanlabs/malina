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
		{"sdxl-base-1.0", true},
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
	want := []string{"flux2-klein-4b", "flux2-klein-9b", "sd-1.5", "sdxl-base-1.0"}
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
// (role, filename) pair for each file. The example header in
// examples/hello/main.go (and the FLUX example in examples/flux2/main.go)
// hard-code these filenames; a typo in the catalog would silently break
// the examples and CI without this test catching it.
func TestBundleShapes(t *testing.T) {
	type wantFile struct {
		role     FileRole
		filename string
	}
	tests := []struct {
		name  string
		gated bool
		files []wantFile
	}{
		{
			name:  "sd-1.5",
			gated: false,
			files: []wantFile{
				{RoleModel, "v1-5-pruned-emaonly.safetensors"},
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
