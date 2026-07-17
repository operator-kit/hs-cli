package ner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/operator-kit/hs-cli/internal/pii"
	"github.com/operator-kit/hs-cli/internal/pii/evaluation"
)

type baselineFixedDetector struct {
	spans []pii.NameSpan
}

func (d baselineFixedDetector) DetectNames(string) ([]pii.NameSpan, error) {
	return append([]pii.NameSpan(nil), d.spans...), nil
}

func TestDistilBERTTypedCorpusBaseline(t *testing.T) {
	modelDir := os.Getenv("HS_PII_G0_MODEL_DIR")
	if modelDir == "" {
		t.Skip("set HS_PII_G0_MODEL_DIR to run the authoritative DistilBERT G0 baseline")
	}
	reportPath := os.Getenv("HS_PII_G0_REPORT")
	if reportPath == "" {
		t.Fatal("HS_PII_G0_REPORT is required when running the G0 baseline")
	}
	metadata := baselineMetadataFromEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	paths := pathsAt(modelDir, CurrentPlatform())
	if err := validateRuntimeBundle(ctx, paths); err != nil {
		t.Fatalf("validate DistilBERT baseline bundle: %v", err)
	}
	detector, err := newDetectorFromPaths(paths)
	if err != nil {
		t.Fatalf("load DistilBERT baseline detector: %v", err)
	}
	defer detector.Close()

	fixtureDir := filepath.Join("..", "testdata", "privacy-filter", "v1")
	corpus, err := evaluation.LoadCorpusDir(fixtureDir)
	if err != nil {
		t.Fatalf("load typed privacy corpus: %v", err)
	}
	metadata.CorpusSHA256 = corpus.Fingerprint
	if _, err := evaluation.LoadPolicy(filepath.Join(fixtureDir, "policy.json")); err != nil {
		t.Fatalf("load frozen typed policy: %v", err)
	}
	if _, _, err := evaluation.LoadPerformanceContract(
		filepath.Join(fixtureDir, "performance", "workloads.json"),
		filepath.Join(fixtureDir, "performance", "budgets.json"),
	); err != nil {
		t.Fatalf("load frozen performance contract: %v", err)
	}
	if _, err := evaluation.LoadIdentitySnapshot(filepath.Join(fixtureDir, "identity-compatibility.json")); err != nil {
		t.Fatalf("load deterministic identity snapshot: %v", err)
	}
	metadata.PolicySHA256 = mustHashFile(t, filepath.Join(fixtureDir, "policy.json"))
	metadata.BudgetSHA256 = mustHashFiles(t,
		filepath.Join(fixtureDir, "performance", "budgets.json"),
		filepath.Join(fixtureDir, "performance", "workloads.json"),
	)
	metadata.IdentitySHA256 = mustHashFile(t, filepath.Join(fixtureDir, "identity-compatibility.json"))

	secret, err := pii.NewSecretString("phase0-baseline-synthetic-pseudonym-key")
	if err != nil {
		t.Fatalf("construct baseline pseudonym key: %v", err)
	}
	pseudonym, err := pii.NewPseudonymContext(secret, "privacy-filter-g0")
	if err != nil {
		t.Fatalf("construct baseline pseudonym context: %v", err)
	}

	observations := make([]evaluation.CaseObservation, 0, len(corpus.Cases))
	for _, fixture := range corpus.Cases {
		if err := ctx.Err(); err != nil {
			t.Fatalf("DistilBERT baseline timed out after case %q: %v", fixture.ID, err)
		}
		nameSpans, err := detector.DetectNames(fixture.Text)
		if err != nil {
			t.Fatalf("case %q DistilBERT inference failed: %v", fixture.ID, err)
		}
		predictions := make([]evaluation.PredictedSpan, 0, len(nameSpans))
		for _, span := range nameSpans {
			predictions = append(predictions, evaluation.PredictedSpan{
				Kind: evaluation.SpanPerson, Start: span.Start, End: span.End,
			})
		}
		known := make([]pii.KnownIdentity, 0, len(fixture.KnownIdentities))
		for _, identity := range fixture.KnownIdentities {
			known = append(known, pii.KnownIdentity{
				Type: identity.Type, First: identity.First, Last: identity.Last, Email: identity.Email, Phone: identity.Phone,
			})
		}
		outputs := make(map[evaluation.Mode]string, len(evaluation.Modes))
		for _, fixtureMode := range evaluation.Modes {
			mode, err := pii.ParseMode(string(fixtureMode))
			if err != nil {
				t.Fatalf("case %q has invalid mode %q: %v", fixture.ID, fixtureMode, err)
			}
			modePseudonym := pseudonym
			if mode == pii.ModeOff {
				modePseudonym = pii.PseudonymContext{}
			}
			engine, err := pii.NewEngine(mode, modePseudonym, pii.WithNER(baselineFixedDetector{spans: nameSpans}))
			if err != nil {
				t.Fatalf("case %q construct %s engine: %v", fixture.ID, fixtureMode, err)
			}
			outputs[fixtureMode] = engine.RedactText(fixture.Text, known)
		}
		observations = append(observations, evaluation.CaseObservation{
			CaseID: fixture.ID, Predictions: predictions, Outputs: outputs,
		})
	}

	report, err := evaluation.Evaluate(corpus, observations, metadata)
	if err != nil {
		t.Fatalf("evaluate DistilBERT baseline: %v", err)
	}
	if err := evaluation.WriteReport(reportPath, report); err != nil {
		t.Fatalf("write DistilBERT baseline report: %v", err)
	}
	requireAuthoritativeG0Pass(t, report)
	t.Logf("wrote synthetic-only DistilBERT G0 baseline: cases=%d exact_f2=%.6f covering_f2=%.6f", report.CasesEvaluated, report.Detector.Exact.F2, report.Detector.Covering.F2)
}

func requireAuthoritativeG0Pass(t *testing.T, report *evaluation.Report) {
	t.Helper()
	if err := evaluation.RequireGatePass(report, "G0"); err != nil {
		t.Fatalf("authoritative DistilBERT baseline did not pass G0: %v", err)
	}
}

func TestAuthoritativeG0GateFailureExitsNonZero(t *testing.T) {
	const helperEnv = "HS_PII_G0_FAILED_GATE_HELPER"
	if os.Getenv(helperEnv) == "1" {
		requireAuthoritativeG0Pass(t, &evaluation.Report{Gates: []evaluation.GateResult{{
			Gate: "G0", State: evaluation.GateFail,
		}}})
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestAuthoritativeG0GateFailureExitsNonZero$", "-test.count=1")
	command.Env = append(os.Environ(), helperEnv+"=1")
	err := command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() == 0 {
		t.Fatalf("failed authoritative G0 gate did not make the baseline command exit non-zero: %v", err)
	}
}

func baselineMetadataFromEnvironment(t *testing.T) evaluation.EvidenceMetadata {
	t.Helper()
	authoritative, err := strconv.ParseBool(requiredBaselineEnv(t, "HS_PII_G0_AUTHORITATIVE"))
	if err != nil {
		t.Fatalf("HS_PII_G0_AUTHORITATIVE: %v", err)
	}
	authority := evaluation.EvidenceAuthority(requiredBaselineEnv(t, "HS_PII_G0_EVIDENCE_AUTHORITY"))
	if authoritative && (authority != evaluation.AuthorityDockerCI || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_NAME") == "") {
		t.Fatal("authoritative G0 evidence requires Docker CI running in GitHub Actions on an identified runner")
	}
	return evaluation.EvidenceMetadata{
		GitCommit:         requiredBaselineEnv(t, "HS_PII_G0_GIT_COMMIT"),
		Backend:           "distilbert",
		ModelRevision:     requiredBaselineEnv(t, "HS_PII_G0_MODEL_REVISION"),
		Variant:           "int8",
		ArtifactSHA256:    requiredBaselineEnv(t, "HS_PII_G0_ARTIFACT_SHA256"),
		RuntimeVersion:    "onnxruntime-1.23.0",
		ContainerImage:    requiredBaselineEnv(t, "HS_PII_G0_CONTAINER_IMAGE"),
		Platform:          runtime.GOOS + "-" + runtime.GOARCH,
		HardwareProfile:   requiredBaselineEnv(t, "HS_PII_G0_HARDWARE_PROFILE"),
		RunnerName:        requiredBaselineEnv(t, "RUNNER_NAME"),
		EvidenceAuthority: authority,
		Authoritative:     authoritative,
	}
}

func requiredBaselineEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required for the G0 baseline", name)
	}
	return value
}

func mustHashFile(t *testing.T, path string) string {
	t.Helper()
	hash, err := evaluation.HashFile(path)
	if err != nil {
		t.Fatalf("hash %s: %v", filepath.Base(path), err)
	}
	return hash
}

func mustHashFiles(t *testing.T, paths ...string) string {
	t.Helper()
	hash, err := evaluation.HashFiles(paths...)
	if err != nil {
		t.Fatalf("hash performance contract: %v", err)
	}
	return hash
}
