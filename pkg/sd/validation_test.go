package sd

import "testing"

func TestContextAPIsRejectNilContext(t *testing.T) {
	testSetup(t)

	tests := []struct {
		name string
		call func() error
	}{
		{"ContextSupportsImageGeneration", func() error { _, err := ContextSupportsImageGeneration(0); return err }},
		{"ContextSupportsVideoGeneration", func() error { _, err := ContextSupportsVideoGeneration(0); return err }},
		{"LoadControlNet", func() error { return LoadControlNet(0, "control") }},
		{"UnloadControlNet", func() error { return UnloadControlNet(0) }},
		{"HasControlNet", func() error { _, err := HasControlNet(0); return err }},
		{"CancelGeneration", func() error { return CancelGeneration(0, CancelAll) }},
		{"DefaultSampleMethod", func() error { _, err := DefaultSampleMethod(0); return err }},
		{"DefaultScheduler", func() error { _, err := DefaultScheduler(0, SampleEuler); return err }},
		{"GenerateVideo", func() error { _, _, err := GenerateVideo(0, VideoGenParams{}); return err }},
		{"GetUpscaleFactor", func() error { _, err := GetUpscaleFactor(0); return err }},
		{"Upscale", func() error { _, err := Upscale(0, testMarshalImage(1), 2); return err }},
		{"ADetailImage ADetailer", func() error {
			_, err := ADetailImage(0, 1, testMarshalImage(1), ADetailerParams{}, ImgGenParams{})
			return err
		}},
		{"ADetailImage generation", func() error {
			_, err := ADetailImage(1, 0, testMarshalImage(1), ADetailerParams{}, ImgGenParams{})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("error: got nil, want nil-context error")
			}
		})
	}
}

func TestImageAPIsRejectNilInput(t *testing.T) {
	testSetup(t)

	if _, err := Upscale(1, nil, 2); err == nil {
		t.Fatal("Upscale nil input: got nil error")
	}
	if _, err := ADetailImage(1, 1, nil, ADetailerParams{}, ImgGenParams{}); err == nil {
		t.Fatal("ADetailImage nil input: got nil error")
	}
	if err := PreprocessCanny(nil, CannyParams{}); err == nil {
		t.Fatal("PreprocessCanny nil input: got nil error")
	}
}

func TestImatrixCollectionTargetAPI(t *testing.T) {
	testSetup(t)

	if err := EnableImatrixCollection(); err != nil {
		t.Fatalf("EnableImatrixCollection: %v", err)
	}
	if err := DisableImatrixCollection(); err != nil {
		t.Fatalf("DisableImatrixCollection: %v", err)
	}
}
