package evaluation

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

const ReportSchemaVersion = 1

type EvidenceAuthority string

const (
	AuthorityDockerCI EvidenceAuthority = "docker-ci"
	AuthorityLocal    EvidenceAuthority = "local-sanity"
)

type EvidenceMetadata struct {
	GitCommit         string
	Backend           string
	ModelRevision     string
	Variant           string
	ArtifactSHA256    string
	RuntimeVersion    string
	ContainerImage    string
	Platform          string
	HardwareProfile   string
	RunnerName        string
	CorpusSHA256      string
	PolicySHA256      string
	BudgetSHA256      string
	IdentitySHA256    string
	EvidenceAuthority EvidenceAuthority
	Authoritative     bool
}

type PredictedSpan struct {
	Kind  SpanKind
	Start int
	End   int
}

type CaseObservation struct {
	CaseID      string
	Predictions []PredictedSpan
	Outputs     map[Mode]string
}

type GateState string

const (
	GateNotRun GateState = "not-run"
	GatePass   GateState = "pass"
	GateFail   GateState = "fail"
)

type Report struct {
	Schema            int               `json:"schema"`
	GitCommit         string            `json:"git_commit"`
	CorpusSHA256      string            `json:"corpus_sha256"`
	PolicySHA256      string            `json:"policy_sha256"`
	BudgetSHA256      string            `json:"budget_sha256"`
	IdentitySHA256    string            `json:"identity_sha256"`
	Backend           string            `json:"backend"`
	ModelRevision     string            `json:"model_revision"`
	Variant           string            `json:"variant"`
	ArtifactSHA256    string            `json:"artifact_sha256"`
	RuntimeVersion    string            `json:"runtime_version"`
	ContainerImage    string            `json:"container_image"`
	Platform          string            `json:"platform"`
	HardwareProfile   string            `json:"hardware_profile"`
	RunnerName        string            `json:"runner_name"`
	EvidenceAuthority EvidenceAuthority `json:"evidence_authority"`
	Authoritative     bool              `json:"authoritative"`
	CasesEvaluated    int               `json:"cases_evaluated"`
	Detector          DetectorMetrics   `json:"detector"`
	FinalOutput       []OutputMetrics   `json:"final_output"`
	CaseResults       []CaseResult      `json:"case_results"`
	Gates             []GateResult      `json:"gates"`
}

type DetectorMetrics struct {
	Exact               MetricSet      `json:"exact"`
	Covering            MetricSet      `json:"covering"`
	SensitivityWeighted WeightedMetric `json:"sensitivity_weighted"`
	ByKind              []MetricSlice  `json:"by_kind"`
	ByRisk              []MetricSlice  `json:"by_risk"`
	ByLanguage          []MetricSlice  `json:"by_language"`
	ByScript            []MetricSlice  `json:"by_script"`
}

type MetricSlice struct {
	Name     string    `json:"name"`
	Exact    MetricSet `json:"exact"`
	Covering MetricSet `json:"covering"`
}

type MetricSet struct {
	TruePositive  int     `json:"true_positive"`
	FalsePositive int     `json:"false_positive"`
	FalseNegative int     `json:"false_negative"`
	Precision     float64 `json:"precision"`
	Recall        float64 `json:"recall"`
	F1            float64 `json:"f1"`
	F2            float64 `json:"f2"`
}

type WeightedMetric struct {
	TruePositiveWeight  int     `json:"true_positive_weight"`
	FalsePositiveWeight int     `json:"false_positive_weight"`
	FalseNegativeWeight int     `json:"false_negative_weight"`
	Precision           float64 `json:"precision"`
	Recall              float64 `json:"recall"`
	F2                  float64 `json:"f2"`
}

type OutputMetrics struct {
	Mode                 Mode `json:"mode"`
	RawValueLeaks        int  `json:"raw_value_leaks"`
	PreservationFailures int  `json:"preservation_failures"`
	ExactPassThroughFail int  `json:"exact_pass_through_failures"`
}

type CaseResult struct {
	CaseID               string `json:"case_id"`
	ExactMisses          int    `json:"exact_misses"`
	CoveringMisses       int    `json:"covering_misses"`
	FalsePositives       int    `json:"false_positives"`
	RawValueLeaks        int    `json:"raw_value_leaks"`
	PreservationFailures int    `json:"preservation_failures"`
}

type GateResult struct {
	Gate  string    `json:"gate"`
	State GateState `json:"state"`
}

type metricCounts struct {
	tp int
	fp int
	fn int
}

type weightedCounts struct {
	tp int
	fp int
	fn int
}

type sliceCounts struct {
	exact    metricCounts
	covering metricCounts
}

func Evaluate(corpus *Corpus, observations []CaseObservation, metadata EvidenceMetadata) (*Report, error) {
	if corpus == nil || len(corpus.Cases) == 0 {
		return nil, fmt.Errorf("evaluate privacy corpus: corpus is empty")
	}
	if err := validateEvidenceMetadata(metadata); err != nil {
		return nil, err
	}
	byCase := make(map[string]CaseObservation, len(observations))
	for _, observation := range observations {
		if _, exists := byCase[observation.CaseID]; exists {
			return nil, fmt.Errorf("evaluate privacy corpus: duplicate observation for case %q", observation.CaseID)
		}
		byCase[observation.CaseID] = observation
	}
	if len(byCase) != len(corpus.Cases) {
		return nil, fmt.Errorf("evaluate privacy corpus: got %d observations for %d cases", len(byCase), len(corpus.Cases))
	}

	exactTotal, coveringTotal := metricCounts{}, metricCounts{}
	weighted := weightedCounts{}
	byKind := make(map[string]*sliceCounts)
	byRisk := make(map[string]*sliceCounts)
	byLanguage := make(map[string]*sliceCounts)
	byScript := make(map[string]*sliceCounts)
	outputTotals := map[Mode]*OutputMetrics{
		ModeOff:       {Mode: ModeOff},
		ModeCustomers: {Mode: ModeCustomers},
		ModeAll:       {Mode: ModeAll},
	}
	caseResults := make([]CaseResult, 0, len(corpus.Cases))

	for _, fixture := range corpus.Cases {
		observation, exists := byCase[fixture.ID]
		if !exists {
			return nil, fmt.Errorf("evaluate privacy corpus: missing observation for case %q", fixture.ID)
		}
		if err := validateObservation(fixture, observation); err != nil {
			return nil, err
		}

		positiveTargets := make([]Target, 0, len(fixture.Targets))
		for _, target := range fixture.Targets {
			if target.Actions.All == ActionRedact {
				positiveTargets = append(positiveTargets, target)
			}
		}
		exactCounts, _, _ := matchSpans(positiveTargets, observation.Predictions, false)
		coveringCounts, _, _ := matchSpans(positiveTargets, observation.Predictions, true)
		_, policyTargetMatched, policyPredictionMatched := matchPolicySpans(positiveTargets, observation.Predictions)
		exactTotal.add(exactCounts)
		coveringTotal.add(coveringCounts)

		weight := riskWeight(fixture.Risk)
		for targetIndex := range positiveTargets {
			if policyTargetMatched[targetIndex] {
				weighted.tp += weight
			} else {
				weighted.fn += weight
			}
		}
		for predictionIndex := range observation.Predictions {
			if !policyPredictionMatched[predictionIndex] {
				weighted.fp++
			}
		}

		addSliceCounts(byRisk, string(fixture.Risk), exactCounts, coveringCounts)
		addSliceCounts(byLanguage, fixture.Language, exactCounts, coveringCounts)
		addSliceCounts(byScript, fixture.Script, exactCounts, coveringCounts)
		for _, kind := range SpanKinds {
			kindTargets := filterTargets(positiveTargets, kind)
			kindPredictions := filterPredictions(observation.Predictions, kind)
			kindExact, _, _ := matchSpans(kindTargets, kindPredictions, false)
			kindCovering, _, _ := matchSpans(kindTargets, kindPredictions, true)
			addSliceCounts(byKind, string(kind), kindExact, kindCovering)
		}

		result := CaseResult{
			CaseID:         fixture.ID,
			ExactMisses:    exactCounts.fn,
			CoveringMisses: coveringCounts.fn,
			FalsePositives: exactCounts.fp,
		}
		for _, mode := range Modes {
			output := observation.Outputs[mode]
			expected := fixture.Outputs.For(mode)
			if mode == ModeOff && output != fixture.Text {
				outputTotals[mode].ExactPassThroughFail++
				result.PreservationFailures++
			}
			for _, absent := range expected.RequiredAbsent {
				if strings.Contains(output, absent) {
					outputTotals[mode].RawValueLeaks++
					result.RawValueLeaks++
				}
			}
			for _, present := range expected.RequiredPresent {
				if !strings.Contains(output, present) {
					outputTotals[mode].PreservationFailures++
					result.PreservationFailures++
				}
			}
		}
		caseResults = append(caseResults, result)
	}

	sort.Slice(caseResults, func(i, j int) bool { return caseResults[i].CaseID < caseResults[j].CaseID })
	finalOutput := []OutputMetrics{*outputTotals[ModeOff], *outputTotals[ModeCustomers], *outputTotals[ModeAll]}
	report := &Report{
		Schema:            ReportSchemaVersion,
		GitCommit:         metadata.GitCommit,
		CorpusSHA256:      metadata.CorpusSHA256,
		PolicySHA256:      metadata.PolicySHA256,
		BudgetSHA256:      metadata.BudgetSHA256,
		IdentitySHA256:    metadata.IdentitySHA256,
		Backend:           metadata.Backend,
		ModelRevision:     metadata.ModelRevision,
		Variant:           metadata.Variant,
		ArtifactSHA256:    metadata.ArtifactSHA256,
		RuntimeVersion:    metadata.RuntimeVersion,
		ContainerImage:    metadata.ContainerImage,
		Platform:          metadata.Platform,
		HardwareProfile:   metadata.HardwareProfile,
		RunnerName:        metadata.RunnerName,
		EvidenceAuthority: metadata.EvidenceAuthority,
		Authoritative:     metadata.Authoritative,
		CasesEvaluated:    len(corpus.Cases),
		Detector: DetectorMetrics{
			Exact:               exactTotal.metrics(),
			Covering:            coveringTotal.metrics(),
			SensitivityWeighted: weighted.metrics(),
			ByKind:              slicesToMetrics(byKind),
			ByRisk:              slicesToMetrics(byRisk),
			ByLanguage:          slicesToMetrics(byLanguage),
			ByScript:            slicesToMetrics(byScript),
		},
		FinalOutput: finalOutput,
		CaseResults: caseResults,
		Gates:       evaluateGates(metadata, finalOutput),
	}
	return report, nil
}

func validateEvidenceMetadata(metadata EvidenceMetadata) error {
	for name, value := range map[string]string{
		"git commit": metadata.GitCommit, "backend": metadata.Backend, "model revision": metadata.ModelRevision,
		"variant": metadata.Variant, "artifact sha256": metadata.ArtifactSHA256, "runtime version": metadata.RuntimeVersion,
		"container image": metadata.ContainerImage, "platform": metadata.Platform, "hardware profile": metadata.HardwareProfile,
		"runner name": metadata.RunnerName, "corpus sha256": metadata.CorpusSHA256,
		"policy sha256": metadata.PolicySHA256, "budget sha256": metadata.BudgetSHA256, "identity sha256": metadata.IdentitySHA256,
	} {
		if value == "" {
			return fmt.Errorf("evaluate privacy corpus: evidence metadata is missing %s", name)
		}
	}
	if metadata.EvidenceAuthority != AuthorityDockerCI && metadata.EvidenceAuthority != AuthorityLocal {
		return fmt.Errorf("evaluate privacy corpus: unsupported evidence authority %q", metadata.EvidenceAuthority)
	}
	if metadata.Authoritative && metadata.EvidenceAuthority != AuthorityDockerCI {
		return fmt.Errorf("evaluate privacy corpus: only docker-ci evidence may be authoritative")
	}
	if metadata.Authoritative {
		for name, value := range map[string]string{
			"artifact": metadata.ArtifactSHA256, "corpus": metadata.CorpusSHA256, "policy": metadata.PolicySHA256,
			"budget": metadata.BudgetSHA256, "identity": metadata.IdentitySHA256,
		} {
			if len(value) != sha256HexLength {
				return fmt.Errorf("evaluate privacy corpus: authoritative %s identity is not SHA-256", name)
			}
			if _, err := hex.DecodeString(value); err != nil {
				return fmt.Errorf("evaluate privacy corpus: authoritative %s identity is not SHA-256", name)
			}
		}
		imageHash := strings.TrimPrefix(metadata.ContainerImage, "sha256:")
		if !strings.HasPrefix(metadata.ContainerImage, "sha256:") || len(imageHash) != sha256HexLength {
			return fmt.Errorf("evaluate privacy corpus: authoritative container image lacks an immutable content identity")
		}
		if _, err := hex.DecodeString(imageHash); err != nil {
			return fmt.Errorf("evaluate privacy corpus: authoritative container image lacks an immutable content identity")
		}
		if metadata.HardwareProfile == "local" || metadata.RunnerName == "local-sanity" {
			return fmt.Errorf("evaluate privacy corpus: local hardware cannot produce authoritative evidence")
		}
	}
	return nil
}

const sha256HexLength = 64

func validateObservation(fixture Case, observation CaseObservation) error {
	if observation.CaseID != fixture.ID {
		return fmt.Errorf("privacy corpus observation ID mismatch for case %q", fixture.ID)
	}
	if len(observation.Outputs) != len(Modes) {
		return fmt.Errorf("privacy corpus observation %q must contain all mode outputs", fixture.ID)
	}
	for _, mode := range Modes {
		if _, exists := observation.Outputs[mode]; !exists {
			return fmt.Errorf("privacy corpus observation %q is missing %s output", fixture.ID, mode)
		}
	}
	seen := make(map[PredictedSpan]struct{}, len(observation.Predictions))
	for _, prediction := range observation.Predictions {
		if !allowedKinds[prediction.Kind] {
			return fmt.Errorf("privacy corpus observation %q contains unsupported kind %q", fixture.ID, prediction.Kind)
		}
		if !validByteRange(fixture.Text, prediction.Start, prediction.End) {
			return fmt.Errorf("privacy corpus observation %q contains invalid byte range [%d:%d]", fixture.ID, prediction.Start, prediction.End)
		}
		if _, exists := seen[prediction]; exists {
			return fmt.Errorf("privacy corpus observation %q contains a duplicate prediction", fixture.ID)
		}
		seen[prediction] = struct{}{}
	}
	return nil
}

func matchSpans(targets []Target, predictions []PredictedSpan, covering bool) (metricCounts, []bool, []bool) {
	return maximumSpanMatching(targets, predictions, func(target Target, prediction PredictedSpan) bool {
		if prediction.Kind != target.Kind {
			return false
		}
		if covering {
			return prediction.Start <= target.Start && prediction.End >= target.End
		}
		return prediction.Start == target.Start && prediction.End == target.End
	})
}

func matchPolicySpans(targets []Target, predictions []PredictedSpan) (metricCounts, []bool, []bool) {
	return maximumSpanMatching(targets, predictions, func(target Target, prediction PredictedSpan) bool {
		if prediction.Kind != target.Kind {
			return false
		}
		if target.Match == MatchCovering {
			return prediction.Start <= target.Start && prediction.End >= target.End
		}
		return prediction.Start == target.Start && prediction.End == target.End
	})
}

func maximumSpanMatching(targets []Target, predictions []PredictedSpan, matches func(Target, PredictedSpan) bool) (metricCounts, []bool, []bool) {
	targetMatched := make([]bool, len(targets))
	predictionMatched := make([]bool, len(predictions))
	predictionOwner := make([]int, len(predictions))
	for i := range predictionOwner {
		predictionOwner[i] = -1
	}
	var assign func(int, []bool) bool
	assign = func(targetIndex int, visited []bool) bool {
		for predictionIndex, prediction := range predictions {
			if visited[predictionIndex] || !matches(targets[targetIndex], prediction) {
				continue
			}
			visited[predictionIndex] = true
			if predictionOwner[predictionIndex] == -1 || assign(predictionOwner[predictionIndex], visited) {
				predictionOwner[predictionIndex] = targetIndex
				return true
			}
		}
		return false
	}
	for targetIndex := range targets {
		assign(targetIndex, make([]bool, len(predictions)))
	}
	for predictionIndex, targetIndex := range predictionOwner {
		if targetIndex >= 0 {
			targetMatched[targetIndex] = true
			predictionMatched[predictionIndex] = true
		}
	}
	counts := metricCounts{}
	for _, matched := range targetMatched {
		if matched {
			counts.tp++
		} else {
			counts.fn++
		}
	}
	for _, matched := range predictionMatched {
		if !matched {
			counts.fp++
		}
	}
	return counts, targetMatched, predictionMatched
}

func filterTargets(targets []Target, kind SpanKind) []Target {
	out := make([]Target, 0)
	for _, target := range targets {
		if target.Kind == kind {
			out = append(out, target)
		}
	}
	return out
}

func filterPredictions(predictions []PredictedSpan, kind SpanKind) []PredictedSpan {
	out := make([]PredictedSpan, 0)
	for _, prediction := range predictions {
		if prediction.Kind == kind {
			out = append(out, prediction)
		}
	}
	return out
}

func (c *metricCounts) add(other metricCounts) {
	c.tp += other.tp
	c.fp += other.fp
	c.fn += other.fn
}

func (c metricCounts) metrics() MetricSet {
	precision := ratio(c.tp, c.tp+c.fp)
	recall := ratio(c.tp, c.tp+c.fn)
	return MetricSet{
		TruePositive: c.tp, FalsePositive: c.fp, FalseNegative: c.fn,
		Precision: precision, Recall: recall,
		F1: fScore(precision, recall, 1), F2: fScore(precision, recall, 2),
	}
}

func (c weightedCounts) metrics() WeightedMetric {
	precision := ratio(c.tp, c.tp+c.fp)
	recall := ratio(c.tp, c.tp+c.fn)
	return WeightedMetric{
		TruePositiveWeight: c.tp, FalsePositiveWeight: c.fp, FalseNegativeWeight: c.fn,
		Precision: precision, Recall: recall, F2: fScore(precision, recall, 2),
	}
}

func addSliceCounts(slices map[string]*sliceCounts, name string, exact, covering metricCounts) {
	counts := slices[name]
	if counts == nil {
		counts = &sliceCounts{}
		slices[name] = counts
	}
	counts.exact.add(exact)
	counts.covering.add(covering)
}

func slicesToMetrics(values map[string]*sliceCounts) []MetricSlice {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]MetricSlice, 0, len(names))
	for _, name := range names {
		out = append(out, MetricSlice{Name: name, Exact: values[name].exact.metrics(), Covering: values[name].covering.metrics()})
	}
	return out
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 1
	}
	return roundMetric(float64(numerator) / float64(denominator))
}

func fScore(precision, recall, beta float64) float64 {
	if precision == 0 && recall == 0 {
		return 0
	}
	betaSquared := beta * beta
	return roundMetric((1 + betaSquared) * precision * recall / (betaSquared*precision + recall))
}

func roundMetric(value float64) float64 {
	return math.Round(value*1_000_000) / 1_000_000
}

func riskWeight(risk RiskTier) int {
	switch risk {
	case RiskCritical:
		return 5
	case RiskHigh:
		return 3
	default:
		return 1
	}
}

func evaluateGates(metadata EvidenceMetadata, finalOutput []OutputMetrics) []GateResult {
	gates := make([]GateResult, 0, 11)
	for gate := 0; gate <= 10; gate++ {
		state := GateNotRun
		if gate == 0 && metadata.Authoritative && metadata.EvidenceAuthority == AuthorityDockerCI {
			state = GatePass
			seenOff := false
			for _, output := range finalOutput {
				if output.Mode == ModeOff {
					seenOff = true
					if output.RawValueLeaks != 0 || output.PreservationFailures != 0 || output.ExactPassThroughFail != 0 {
						state = GateFail
					}
				}
			}
			if !seenOff {
				state = GateFail
			}
		}
		gates = append(gates, GateResult{Gate: fmt.Sprintf("G%d", gate), State: state})
	}
	return gates
}

func WriteReport(path string, report *Report) error {
	if report == nil {
		return fmt.Errorf("write evaluation report: report is nil")
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode evaluation report: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write evaluation report: %w", err)
	}
	return nil
}

// RequireGatePass rejects missing, duplicate, not-run, and failed gate results.
// Authoritative commands call this after writing their report so failed evidence
// remains inspectable without allowing the command to exit successfully.
func RequireGatePass(report *Report, gate string) error {
	if report == nil {
		return fmt.Errorf("require evaluation gate %s: report is nil", gate)
	}
	var matches []GateResult
	for _, result := range report.Gates {
		if result.Gate == gate {
			matches = append(matches, result)
		}
	}
	if len(matches) != 1 {
		return fmt.Errorf("require evaluation gate %s: found %d results, want exactly one", gate, len(matches))
	}
	if matches[0].State != GatePass {
		return fmt.Errorf("require evaluation gate %s: state is %s, want %s", gate, matches[0].State, GatePass)
	}
	return nil
}
