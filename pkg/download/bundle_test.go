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
		{"flux2-klein-9b", true},
		{"  Flux2-Klein-9B  ", true},
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
	want := []string{"flux2-klein-9b", "sd-1.5", "sdxl-base-1.0"}
	if len(got) != len(want) {
		t.Fatalf("BundleNames: got %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("BundleNames[%d]: got %q, want %q", i, got[i], name)
		}
	}
}

func TestFluxGated(t *testing.T) {
	b, ok := BundleByName("flux2-klein-9b")
	if !ok {
		t.Fatal("flux2-klein-9b not in catalog")
	}
	if !b.Gated {
		t.Error("flux2-klein-9b: expected Gated = true")
	}
	if len(b.Files) != 3 {
		t.Errorf("flux2-klein-9b: got %d files, want 3", len(b.Files))
	}
}
