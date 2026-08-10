package sd

import "testing"

func TestMapGGMLLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		rawLevel int32
		previous LogLevel
		want     LogLevel
	}{
		{name: "none", rawLevel: 0, previous: LogError, want: LogDebug},
		{name: "debug", rawLevel: 1, previous: LogError, want: LogDebug},
		{name: "info", rawLevel: 2, previous: LogError, want: LogInfo},
		{name: "warning", rawLevel: 3, previous: LogInfo, want: LogWarn},
		{name: "error", rawLevel: 4, previous: LogInfo, want: LogError},
		{name: "continuation", rawLevel: 5, previous: LogWarn, want: LogWarn},
		{name: "unknown", rawLevel: 99, previous: LogError, want: LogDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapGGMLLogLevel(tt.rawLevel, tt.previous)
			if got != tt.want {
				t.Errorf("mapGGMLLogLevel(%d, %d) = %d, want %d", tt.rawLevel, tt.previous, got, tt.want)
			}
		})
	}
}
