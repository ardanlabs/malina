package runner

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultConfigEnvironment(t *testing.T) {
	t.Setenv("MALINA_API_HOST", "localhost:9000")
	t.Setenv("MALINA_LIB", "/native")
	t.Setenv("MALINA_MODEL", "/model.gguf")
	t.Setenv("MALINA_QUEUE_DEPTH", "4")
	t.Setenv("MALINA_READ_TIMEOUT", "5s")
	t.Setenv("MALINA_WRITE_TIMEOUT", "6s")
	t.Setenv("MALINA_IDLE_TIMEOUT", "7s")
	t.Setenv("MALINA_INFERENCE_TIMEOUT", "8s")
	t.Setenv("MALINA_SHUTDOWN_TIMEOUT", "9s")
	t.Setenv("MALINA_BUI", "false")

	cfg := DefaultConfig()
	if cfg.Host != "localhost:9000" || cfg.LibPath != "/native" || cfg.ModelPath != "/model.gguf" || cfg.QueueDepth != 4 {
		t.Fatalf("paths and queue: got %+v", cfg)
	}
	if cfg.ReadTimeout != 5*time.Second || cfg.WriteTimeout != 6*time.Second || cfg.IdleTimeout != 7*time.Second || cfg.InferenceTimeout != 8*time.Second || cfg.ShutdownTimeout != 9*time.Second || cfg.BUI {
		t.Fatalf("timeouts and BUI: got %+v", cfg)
	}
}

func TestConfigValidate(t *testing.T) {
	valid := Config{Host: "localhost:8080", LibPath: "/native", QueueDepth: 2, ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second, InferenceTimeout: time.Second, ShutdownTimeout: time.Second}
	tests := []struct {
		name, want string
		change     func(*Config)
	}{
		{name: "host", want: "host", change: func(cfg *Config) { cfg.Host = "" }},
		{name: "library", want: "library", change: func(cfg *Config) { cfg.LibPath = "" }},
		{name: "queue", want: "queue", change: func(cfg *Config) { cfg.QueueDepth = 0 }},
		{name: "timeout", want: "timeouts", change: func(cfg *Config) { cfg.InferenceTimeout = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.change(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate: got %v, want error containing %q", err, tt.want)
			}
		})
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate valid config: %v", err)
	}
}
