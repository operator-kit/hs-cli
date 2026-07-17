package evaluation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestG0DockerBaselineKeepsWeightsReadOnlyAndInferenceOffline(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	dockerfile := readContractFile(t, filepath.Join(repoRoot, "build", "privacy-filter", "Dockerfile.g0"))
	if strings.Contains(dockerfile, "model_quantized.onnx") || strings.Contains(dockerfile, "trusted_manifest.json") {
		t.Fatalf("G0 Docker image must not bake model artifacts")
	}
	if !strings.Contains(dockerfile, "@sha256:") || !strings.Contains(dockerfile, "go mod verify") {
		t.Fatalf("G0 Docker image must pin its base and verify locked dependencies")
	}

	script := readContractFile(t, filepath.Join(repoRoot, "scripts", "privacy-filter-g0-baseline.sh"))
	for _, required := range []string{
		"--network none", "dst=/models/distilbert,readonly", "HS_PII_G0_EVIDENCE_AUTHORITY",
		"HS_PII_G0_AUTHORITATIVE", "HS_PII_G0_ARTIFACTS_JSON", "sha256sum", "GITHUB_ACTIONS", "RUNNER_NAME",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("G0 baseline script is missing required control %q", required)
		}
	}

	workflow := readContractFile(t, filepath.Join(repoRoot, ".github", "workflows", "privacy-filter-evaluation.yml"))
	for _, required := range []string{
		"needs: hermetic", "HS_PII_G0_EVIDENCE_AUTHORITY: docker-ci", "HS_PII_G0_AUTHORITATIVE: \"true\"",
		"distilbert-baseline.json", "any(.gates[]; .gate == \"G0\" and .state == \"pass\")",
		"if: always()", "go.mod", "go.sum", "patches/onnxruntime-purego/go.mod",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("G0 evaluation workflow is missing required ordering/evidence control %q", required)
		}
	}
}

func readContractFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	return string(raw)
}
