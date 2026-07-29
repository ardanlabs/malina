package generate

import (
	"strings"
	"testing"
)

func TestRunRequiredFlags(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		prompt  string
		wantErr string
	}{
		{name: "model", wantErr: "--model is required"},
		{name: "prompt", model: "model.safetensors", wantErr: "--prompt is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCmd()
			if err := cmd.Flags().Set("model", tt.model); err != nil {
				t.Fatal(err)
			}
			if err := cmd.Flags().Set("prompt", tt.prompt); err != nil {
				t.Fatal(err)
			}
			err := run(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("run: got %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}
