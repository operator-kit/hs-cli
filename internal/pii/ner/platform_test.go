package ner

import (
	"strings"
	"testing"
)

func TestRuntimeCapabilityMatrix(t *testing.T) {
	tests := []struct {
		platform  Platform
		supported bool
	}{
		{Platform{OS: "linux", Arch: "amd64"}, true},
		{Platform{OS: "linux", Arch: "arm64"}, true},
		{Platform{OS: "darwin", Arch: "amd64"}, true},
		{Platform{OS: "darwin", Arch: "arm64"}, true},
		{Platform{OS: "windows", Arch: "amd64"}, false},
		{Platform{OS: "windows", Arch: "arm64"}, false},
		{Platform{OS: "freebsd", Arch: "amd64"}, false},
		{Platform{OS: "linux", Arch: "riscv64"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.platform.Key(), func(t *testing.T) {
			got := RuntimeCapabilityFor(tt.platform)
			if got.Supported != tt.supported {
				t.Fatalf("Supported = %v, want %v (%s)", got.Supported, tt.supported, got.Reason)
			}
			if !tt.supported && !strings.Contains(got.Reason, tt.platform.Key()) {
				t.Fatalf("unsupported reason %q does not name platform %q", got.Reason, tt.platform.Key())
			}
		})
	}
}

func TestSupportedPlatformsExcludeWindows(t *testing.T) {
	for _, platform := range SupportedPlatforms() {
		if platform.OS == "windows" {
			t.Fatalf("Windows must not be advertised before its runtime smoke test: %+v", platform)
		}
	}
}
