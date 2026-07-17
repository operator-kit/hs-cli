package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustSHA256(t *testing.T, raw []byte) string {
	t.Helper()
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func TestDeterministicMetricAndGateEvaluator(t *testing.T) {
	redactText := "private-value"
	preserveText := "public-value"
	corpus := &Corpus{Cases: []Case{
		{
			ID: "redact-case", Language: "en", Script: "Latin", Shape: "prose", Risk: RiskCritical, Text: redactText,
			Targets: []Target{{ID: "value", Kind: SpanSecret, Start: 0, End: len(redactText), Value: redactText, Match: MatchCovering, Actions: ModeActions{ActionPreserve, ActionRedact, ActionRedact}}},
			Outputs: ModeOutputs{
				Off:       OutputExpectation{RequiredAbsent: []string{}, RequiredPresent: []string{redactText}},
				Customers: OutputExpectation{RequiredAbsent: []string{redactText}, RequiredPresent: []string{}},
				All:       OutputExpectation{RequiredAbsent: []string{redactText}, RequiredPresent: []string{}},
			},
		},
		{
			ID: "preserve-case", Language: "en", Script: "Latin", Shape: "prose", Risk: RiskPreservation, Text: preserveText,
			Targets: []Target{{ID: "value", Kind: SpanAccountNumber, Start: 0, End: len(preserveText), Value: preserveText, Match: MatchExact, Actions: ModeActions{ActionPreserve, ActionPreserve, ActionPreserve}}},
			Outputs: ModeOutputs{
				Off:       OutputExpectation{RequiredAbsent: []string{}, RequiredPresent: []string{preserveText}},
				Customers: OutputExpectation{RequiredAbsent: []string{}, RequiredPresent: []string{preserveText}},
				All:       OutputExpectation{RequiredAbsent: []string{}, RequiredPresent: []string{preserveText}},
			},
		},
	}}
	observations := []CaseObservation{
		{CaseID: "redact-case", Predictions: []PredictedSpan{{Kind: SpanSecret, Start: 0, End: len(redactText)}}, Outputs: map[Mode]string{ModeOff: redactText, ModeCustomers: SecretMarker, ModeAll: SecretMarker}},
		{CaseID: "preserve-case", Outputs: map[Mode]string{ModeOff: preserveText, ModeCustomers: preserveText, ModeAll: preserveText}},
	}
	metadata := testMetadata()
	report, err := Evaluate(corpus, observations, metadata)
	if err != nil {
		t.Fatalf("evaluate metrics: %v", err)
	}
	if report.Detector.Exact.TruePositive != 1 || report.Detector.Exact.FalsePositive != 0 || report.Detector.Exact.FalseNegative != 0 {
		t.Fatalf("unexpected exact metrics: %+v", report.Detector.Exact)
	}
	if report.Detector.Exact.F2 != 1 || report.Detector.SensitivityWeighted.F2 != 1 {
		t.Fatalf("unexpected F2 metrics: exact=%v weighted=%v", report.Detector.Exact.F2, report.Detector.SensitivityWeighted.F2)
	}
	for _, output := range report.FinalOutput {
		if output.RawValueLeaks != 0 || output.PreservationFailures != 0 || output.ExactPassThroughFail != 0 {
			t.Fatalf("unexpected final-output failures for %s: %+v", output.Mode, output)
		}
	}
	if report.Gates[0].State != GateNotRun {
		t.Fatalf("local evidence must not pass G0: %+v", report.Gates[0])
	}

	second, err := Evaluate(corpus, observations, metadata)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(report)
	secondJSON, _ := json.Marshal(second)
	if !reflect.DeepEqual(firstJSON, secondJSON) {
		t.Fatalf("metric report is not deterministic")
	}
	if strings.Contains(string(firstJSON), redactText) {
		t.Fatalf("metric report serialized a raw protected value")
	}
	schemaRaw, err := os.ReadFile(filepath.Join(corpusDir(t), "report-schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	report.ReportSchemaSHA256 = mustSHA256(t, schemaRaw)
	if err := ValidateReportAgainstSchema(schemaRaw, report); err != nil {
		t.Fatalf("metric report does not satisfy its frozen schema: %v", err)
	}
}

func TestGateEvidenceRejectsLocalSanityAsAuthoritative(t *testing.T) {
	metadata := testMetadata()
	metadata.Authoritative = true
	if err := validateEvidenceMetadata(metadata); err == nil {
		t.Fatalf("local-sanity evidence was allowed to become authoritative")
	}
	metadata.EvidenceAuthority = AuthorityDockerCI
	hash := strings.Repeat("a", 64)
	metadata.ArtifactSHA256 = hash
	metadata.CorpusSHA256 = hash
	metadata.PolicySHA256 = hash
	metadata.BudgetSHA256 = hash
	metadata.IdentitySHA256 = hash
	metadata.ContainerImage = "sha256:" + hash
	metadata.HardwareProfile = "docker-functional"
	metadata.RunnerName = "privacy-filter-ci-01"
	metadata.Artifacts = []ArtifactIdentity{
		{Name: "archive", SHA256: hash, SizeBytes: 10},
		{Name: "config.json", SHA256: hash, SizeBytes: 10},
		{Name: "model_quantized.onnx", SHA256: hash, SizeBytes: 10},
		{Name: "tokenizer.json", SHA256: hash, SizeBytes: 10},
		{Name: "libonnxruntime.so", SHA256: hash, SizeBytes: 10},
	}
	if err := validateEvidenceMetadata(metadata); err != nil {
		t.Fatalf("valid Docker CI evidence was rejected: %v", err)
	}
	finalOutput := []OutputMetrics{{Mode: ModeOff}}
	if gates := evaluateGates(metadata, finalOutput); gates[0].State != GatePass {
		t.Fatalf("authoritative Docker CI evidence did not advance G0: %+v", gates[0])
	}
	for _, gate := range evaluateGates(metadata, finalOutput)[1:] {
		if gate.State != GateNotRun {
			t.Fatalf("Phase 0 evidence advanced a later gate: %+v", gate)
		}
	}
	failingOff := []OutputMetrics{{Mode: ModeOff, ExactPassThroughFail: 1}}
	if gates := evaluateGates(metadata, failingOff); gates[0].State != GateFail {
		t.Fatalf("authoritative off-mode regression did not fail G0: %+v", gates[0])
	}
}

func TestRequireGatePassRejectsFailedIncompleteAndDuplicateEvidence(t *testing.T) {
	for _, test := range []struct {
		name   string
		report *Report
	}{
		{name: "nil report"},
		{name: "missing gate", report: &Report{}},
		{name: "not run", report: &Report{Gates: []GateResult{{Gate: "G0", State: GateNotRun}}}},
		{name: "failed", report: &Report{Gates: []GateResult{{Gate: "G0", State: GateFail}}}},
		{name: "duplicate", report: &Report{Gates: []GateResult{
			{Gate: "G0", State: GatePass}, {Gate: "G0", State: GatePass},
		}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := RequireGatePass(test.report, "G0"); err == nil {
				t.Fatal("non-passing gate evidence was accepted")
			}
		})
	}
	passing := &Report{Gates: []GateResult{{Gate: "G0", State: GatePass}}}
	if err := RequireGatePass(passing, "G0"); err != nil {
		t.Fatalf("passing gate evidence was rejected: %v", err)
	}
}

func TestMetricMatchingUsesMaximumOneToOneAssignment(t *testing.T) {
	targets := []Target{
		{Kind: SpanSecret, Start: 10, End: 20},
		{Kind: SpanSecret, Start: 0, End: 5},
	}
	predictions := []PredictedSpan{
		{Kind: SpanSecret, Start: 0, End: 20},
		{Kind: SpanSecret, Start: 10, End: 20},
	}
	counts, targetMatched, predictionMatched := matchSpans(targets, predictions, true)
	if counts.tp != 2 || counts.fp != 0 || counts.fn != 0 {
		t.Fatalf("covering matcher did not find maximum assignment: %+v", counts)
	}
	for i, matched := range targetMatched {
		if !matched {
			t.Fatalf("target %d was left unmatched", i)
		}
	}
	for i, matched := range predictionMatched {
		if !matched {
			t.Fatalf("prediction %d was left unmatched", i)
		}
	}
}

func TestMetricReportFreezesRequiredMatchAndEndToEndSlices(t *testing.T) {
	text := "Engineer Rowan Vale joined the thread."
	value := "Rowan Vale"
	start := strings.Index(text, value)
	fixture := Case{
		ID: "unknown-third-party", Language: "en", Script: "Latin", Shape: "quoted-reply", Risk: RiskHigh, Text: text,
		Targets: []Target{{
			ID: "developer", Kind: SpanPerson, Start: start, End: start + len(value), Value: value, Match: MatchCovering,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact},
		}},
		Outputs: ModeOutputs{
			Off:       OutputExpectation{RequiredAbsent: []string{}, RequiredPresent: []string{value}},
			Customers: OutputExpectation{RequiredAbsent: []string{value}, RequiredPresent: []string{}},
			All:       OutputExpectation{RequiredAbsent: []string{value}, RequiredPresent: []string{}},
		},
	}
	observation := CaseObservation{
		CaseID:      fixture.ID,
		Predictions: []PredictedSpan{{Kind: SpanPerson, Start: start - len("Engineer "), End: start + len(value)}},
		Outputs:     map[Mode]string{ModeOff: text, ModeCustomers: "Engineer [PERSON] joined the thread.", ModeAll: "Engineer [PERSON] joined the thread."},
	}
	report, err := Evaluate(&Corpus{Cases: []Case{fixture}}, []CaseObservation{observation}, testMetadata())
	if err != nil {
		t.Fatal(err)
	}
	if report.Detector.Exact.FalseNegative != 1 || report.Detector.Covering.TruePositive != 1 || report.Detector.RequiredMatch.TruePositive != 1 {
		t.Fatalf("target match policies were not reported independently: %+v", report.Detector)
	}
	identity := findMetricSlice(t, report.Detector.ByIdentityPolicy, "unknown-third-party")
	if identity.RequiredMatch.TruePositive != 1 {
		t.Fatalf("unknown-third-party detector slice was not preserved: %+v", identity)
	}
	format := findMetricSlice(t, report.Detector.ByFormat, "quoted-reply")
	if format.Covering.TruePositive != 1 {
		t.Fatalf("format detector slice was not preserved: %+v", format)
	}
	output := findOutputSlice(t, report.FinalOutputSlices, ModeCustomers, "identity_policy", "unknown-third-party")
	if output.RequiredAbsent != 1 || output.RawValueLeaks != 0 || output.LeakRate != 0 {
		t.Fatalf("unknown-third-party final-output slice is wrong: %+v", output)
	}
}

func findMetricSlice(t *testing.T, slices []MetricSlice, name string) MetricSlice {
	t.Helper()
	for _, slice := range slices {
		if slice.Name == name {
			return slice
		}
	}
	t.Fatalf("metric slice %q not found", name)
	return MetricSlice{}
}

func findOutputSlice(t *testing.T, slices []OutputMetricSlice, mode Mode, dimension, name string) OutputMetricSlice {
	t.Helper()
	for _, slice := range slices {
		if slice.Mode == mode && slice.Dimension == dimension && slice.Name == name {
			return slice
		}
	}
	t.Fatalf("output slice %s/%s/%s not found", mode, dimension, name)
	return OutputMetricSlice{}
}

func testMetadata() EvidenceMetadata {
	hash := strings.Repeat("a", 64)
	return EvidenceMetadata{
		GitCommit: "fixture-commit", Backend: "distilbert", ModelRevision: "fixture-model",
		Variant: "int8", ArtifactSHA256: hash, RuntimeVersion: "fixture-runtime",
		ContainerImage: "fixture-image", Platform: "linux-amd64", HardwareProfile: "local", RunnerName: "fixture-runner", CorpusSHA256: hash, BroadCorpusSHA256: hash,
		PolicySHA256: hash, BudgetSHA256: hash, IdentitySHA256: hash, ReportSchemaSHA256: hash,
		Artifacts:         []ArtifactIdentity{{Name: "fixture-component", SHA256: hash, SizeBytes: 1}},
		EvidenceAuthority: AuthorityLocal, Authoritative: false,
	}
}
