package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/operator-kit/hs-cli/internal/pii/ner"
)

func TestPIIModelCommandStatusExplainsUnsupportedRuntime(t *testing.T) {
	var output bytes.Buffer
	writePIIModelStatus(&output, ner.ModelStatus{
		State:    ner.ModelUnsupported,
		Platform: ner.Platform{OS: "windows", Arch: "amd64"},
		Reason:   "local NER runtime is not supported on windows-amd64; free-form content remains hidden",
	})
	got := output.String()
	for _, want := range []string{"unsupported on windows-amd64", "free-form content remains hidden"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output %q does not contain %q", got, want)
		}
	}
	if strings.Contains(got, "installed and verified") {
		t.Fatalf("unsupported status claimed readiness: %q", got)
	}
}

func TestPIIModelCommandStatusRejectsLegacyTrust(t *testing.T) {
	var output bytes.Buffer
	writePIIModelStatus(&output, ner.ModelStatus{
		State: ner.ModelInstalledUnverified,
		Dir:   "legacy-cache",
	})
	got := output.String()
	for _, want := range []string{"unverified legacy bundle", "will not be loaded"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output %q does not contain %q", got, want)
		}
	}
}
