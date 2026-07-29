package model

import (
	"context"
	"os"
	"testing"

	"github.com/ardanlabs/malina/sdk/malina/sd"
)

func TestNativeReusableContext(t *testing.T) {
	libPath, modelPath := os.Getenv("MALINA_LIB"), os.Getenv("MALINA_TEST_MODEL")
	if libPath == "" || modelPath == "" {
		t.Skip("set MALINA_LIB and MALINA_TEST_MODEL to run the native integration test")
	}
	if err := sd.Load(libPath); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sd.GGMLBackendDeviceCount() == 0 {
		if err := sd.Init(libPath); err != nil {
			t.Fatalf("Init: %v", err)
		}
	}
	cfg, err := NewConfig(WithModelPath(modelPath))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	m, err := NewModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	t.Cleanup(func() {
		if err := m.Unload(); err != nil {
			t.Errorf("Unload: %v", err)
		}
	})
	p := NewGenerateParams()
	p.Prompt, p.Width, p.Height, p.Steps, p.Seed = "a red square", 64, 64, 1, 7
	for range 2 {
		if _, err := m.Generate(p); err != nil {
			t.Fatalf("Generate: %v", err)
		}
	}
}
