package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

func GeneratePerformanceWorkloadsDocument() (*WorkloadsDocument, error) {
	workloads := expectedPerformanceWorkloads()
	for index := range workloads {
		hash, err := performancePayloadSHA256(workloads[index])
		if err != nil {
			return nil, err
		}
		workloads[index].PayloadSHA256 = hash
	}
	return &WorkloadsDocument{Schema: CorpusSchemaVersion, Workloads: workloads}, nil
}

func WritePerformanceWorkloads(path string) error {
	document, err := GeneratePerformanceWorkloadsDocument()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode performance workloads: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write performance workloads: %w", err)
	}
	return nil
}

type WorkloadsDocument struct {
	Schema    int                   `json:"schema"`
	Workloads []PerformanceWorkload `json:"workloads"`
}

type PerformanceWorkload struct {
	ID               string   `json:"id"`
	Description      string   `json:"description"`
	TargetBytes      int      `json:"target_bytes"`
	WarmupSamples    int      `json:"warmup_samples"`
	MeasuredSamples  int      `json:"measured_samples"`
	FreshProcesses   int      `json:"fresh_processes"`
	Concurrency      int      `json:"concurrency"`
	Requests         int      `json:"requests"`
	RequiredMetrics  []string `json:"required_metrics"`
	Generator        string   `json:"generator"`
	GeneratorVersion int      `json:"generator_version"`
	GeneratorSeed    string   `json:"generator_seed"`
	PayloadSHA256    string   `json:"payload_sha256"`
}

type BudgetsDocument struct {
	Schema               int                `json:"schema"`
	EvidenceAuthority    string             `json:"evidence_authority"`
	Sampling             SamplingPolicy     `json:"sampling"`
	HardwareProfiles     []HardwareProfile  `json:"hardware_profiles"`
	Limits               []PerformanceLimit `json:"limits"`
	HardwareIdentityFile string             `json:"hardware_identity_file"`
}

type SamplingPolicy struct {
	MinimumFreshProcesses int `json:"minimum_fresh_processes"`
	MinimumWarmSamples    int `json:"minimum_warm_samples"`
	DiagnosticReruns      int `json:"diagnostic_reruns"`
}

type HardwareProfile struct {
	ID               string   `json:"id"`
	RunnerLabels     []string `json:"runner_labels"`
	VCPUs            int      `json:"vcpus"`
	MemoryBytes      int64    `json:"memory_bytes"`
	SwapBytes        int64    `json:"swap_bytes"`
	CPUOnly          bool     `json:"cpu_only"`
	Storage          string   `json:"storage"`
	RequiredMetadata []string `json:"required_metadata"`
}

type PerformanceLimit struct {
	ID        string   `json:"id"`
	Workload  string   `json:"workload"`
	Statistic string   `json:"statistic"`
	Maximum   int64    `json:"maximum"`
	Unit      string   `json:"unit"`
	Profiles  []string `json:"profiles"`
}

const (
	PerformancePayloadGenerator = "hs-privacy-performance-payload"
	PerformancePayloadVersion   = 1
)

var standardPerformanceMetrics = []string{
	"load_time", "first_inference", "warm_p50", "warm_p95", "warm_p99", "cpu_time", "wall_time",
	"throughput", "rss", "page_faults", "disk_bytes", "detector_constructions",
}

func expectedPerformanceWorkloads() []PerformanceWorkload {
	workloads := []PerformanceWorkload{
		{ID: "subject", Description: "Synthetic support subject exactly 256 UTF-8 bytes.", TargetBytes: 256, WarmupSamples: 10, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 1, Requests: 1},
		{ID: "message", Description: "Synthetic 2 KiB support message with mixed typed privacy targets.", TargetBytes: 2048, WarmupSamples: 10, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 1, Requests: 1},
		{ID: "thread", Description: "Synthetic 20 KiB multi-message thread exercising window boundaries.", TargetBytes: 20480, WarmupSamples: 10, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 1, Requests: 1},
		{ID: "export", Description: "Synthetic 100 KiB conversation export.", TargetBytes: 102400, WarmupSamples: 5, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 1, Requests: 1},
		{ID: "adversarial-long", Description: "Dense 128 KiB synthetic Unicode input with awkward token and byte boundaries.", TargetBytes: 131072, WarmupSamples: 5, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 1, Requests: 1},
		{ID: "mcp-longevity", Description: "One hundred warm 2 KiB synthetic requests after explicit warm-up.", TargetBytes: 2048, WarmupSamples: 10, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 1, Requests: 100},
		{ID: "mcp-burst", Description: "Four concurrent 2 KiB synthetic requests measuring queueing and deadlock safety.", TargetBytes: 2048, WarmupSamples: 10, MeasuredSamples: 100, FreshProcesses: 20, Concurrency: 4, Requests: 4},
	}
	for index := range workloads {
		workloads[index].RequiredMetrics = append([]string(nil), standardPerformanceMetrics...)
		workloads[index].Generator = PerformancePayloadGenerator
		workloads[index].GeneratorVersion = PerformancePayloadVersion
		workloads[index].GeneratorSeed = "performance-" + workloads[index].ID + "-v1"
	}
	workloads[3].RequiredMetrics = append(workloads[3].RequiredMetrics, "truncations")
	workloads[4].RequiredMetrics = append(workloads[4].RequiredMetrics, "truncations", "fail_closed")
	workloads[5].RequiredMetrics = append(workloads[5].RequiredMetrics, "rss_growth", "goroutines")
	workloads[6].RequiredMetrics = append(workloads[6].RequiredMetrics, "deadlocks", "race_failures")
	return workloads
}

func expectedPerformanceLimits() []PerformanceLimit {
	allProfiles := []string{"H0", "H1", "H2"}
	return []PerformanceLimit{
		{ID: "detector-readiness-reference", Workload: "all", Statistic: "p95", Maximum: 5000, Unit: "milliseconds", Profiles: []string{"H1", "H2"}},
		{ID: "detector-readiness-slowest", Workload: "all", Statistic: "p95", Maximum: 8000, Unit: "milliseconds", Profiles: []string{"H0"}},
		{ID: "warm-subject", Workload: "subject", Statistic: "p95", Maximum: 250, Unit: "milliseconds", Profiles: append([]string(nil), allProfiles...)},
		{ID: "warm-message", Workload: "message", Statistic: "p95", Maximum: 750, Unit: "milliseconds", Profiles: append([]string(nil), allProfiles...)},
		{ID: "warm-thread", Workload: "thread", Statistic: "p95", Maximum: 4000, Unit: "milliseconds", Profiles: append([]string(nil), allProfiles...)},
		{ID: "warm-export", Workload: "export", Statistic: "p95", Maximum: 20000, Unit: "milliseconds", Profiles: append([]string(nil), allProfiles...)},
		{ID: "peak-rss", Workload: "all", Statistic: "maximum", Maximum: 2 * 1024 * 1024 * 1024, Unit: "bytes", Profiles: append([]string(nil), allProfiles...)},
		{ID: "steady-rss", Workload: "mcp-longevity", Statistic: "post-warm", Maximum: 1536 * 1024 * 1024, Unit: "bytes", Profiles: append([]string(nil), allProfiles...)},
		{ID: "rss-growth-percent", Workload: "mcp-longevity", Statistic: "maximum", Maximum: 5, Unit: "percent", Profiles: append([]string(nil), allProfiles...)},
		{ID: "rss-growth-bytes", Workload: "mcp-longevity", Statistic: "maximum", Maximum: 64 * 1024 * 1024, Unit: "bytes", Profiles: append([]string(nil), allProfiles...)},
		{ID: "installed-bundle", Workload: "all", Statistic: "maximum", Maximum: 1280 * 1024 * 1024, Unit: "bytes", Profiles: append([]string(nil), allProfiles...)},
		{ID: "compressed-download", Workload: "all", Statistic: "maximum", Maximum: 1181116006, Unit: "bytes", Profiles: append([]string(nil), allProfiles...)},
		{ID: "detector-construction", Workload: "all", Statistic: "maximum", Maximum: 1, Unit: "count", Profiles: append([]string(nil), allProfiles...)},
		{ID: "mode-off-construction", Workload: "all", Statistic: "maximum", Maximum: 0, Unit: "count", Profiles: append([]string(nil), allProfiles...)},
		{ID: "mcp-burst-factor", Workload: "mcp-burst", Statistic: "single-request-p95-multiple", Maximum: 5, Unit: "factor", Profiles: append([]string(nil), allProfiles...)},
	}
}

func expectedHardwareProfiles() []HardwareProfile {
	metadata := []string{
		"runner_name", "cpu_model", "architecture", "instruction_sets", "physical_cores", "logical_cores",
		"virtualization", "ram_limit", "swap_policy", "numa", "storage", "benchmark_workspace", "os", "kernel", "power_governor", "thermal_state",
		"onnx_runtime", "go_version", "python_version", "thread_settings", "container_limits", "git_commit", "corpus_sha256",
		"budget_sha256", "model_sha256",
	}
	return []HardwareProfile{
		{ID: "H0", RunnerLabels: []string{"self-hosted", "linux", "x64", "privacy-filter-h0"}, VCPUs: 2, MemoryBytes: 4 * 1024 * 1024 * 1024, SwapBytes: 0, CPUOnly: true, Storage: "ssd", RequiredMetadata: append([]string(nil), metadata...)},
		{ID: "H1", RunnerLabels: []string{"self-hosted", "linux", "x64", "privacy-filter-h1"}, VCPUs: 4, MemoryBytes: 8 * 1024 * 1024 * 1024, SwapBytes: 0, CPUOnly: true, Storage: "ssd", RequiredMetadata: append([]string(nil), metadata...)},
		{ID: "H2", RunnerLabels: []string{"self-hosted", "linux", "x64", "privacy-filter-h2"}, VCPUs: 8, MemoryBytes: 16 * 1024 * 1024 * 1024, SwapBytes: 0, CPUOnly: true, Storage: "ssd", RequiredMetadata: append([]string(nil), metadata...)},
	}
}

// GeneratePerformancePayload constructs the exact bytes identified by a
// workload. It is deliberately independent of detector implementation.
func GeneratePerformancePayload(workload PerformanceWorkload) ([]byte, error) {
	if workload.Generator != PerformancePayloadGenerator || workload.GeneratorVersion != PerformancePayloadVersion ||
		!idPattern.MatchString(workload.GeneratorSeed) || workload.TargetBytes <= 0 {
		return nil, fmt.Errorf("performance workload %q has an unsupported payload generator contract", workload.ID)
	}
	provenance := SyntheticProvenance{
		Generator: SyntheticSecretGenerator, Version: SyntheticSecretVersion,
		Seed: workload.GeneratorSeed, Recipe: "command-secret", Purpose: "must-detect",
	}
	credential, err := GenerateSyntheticValue(provenance)
	if err != nil {
		return nil, err
	}
	patterns := map[string]string{
		"subject":          "Support request from Rowan Vale about account CUST-2048 and credential " + credential + ". ",
		"message":          "Customer Rowan Vale wrote from rowan.vale@example.invalid about 742 Evergreen Road. Generated credential=" + credential + ". Public release 2026-07-17. ",
		"thread":           "[message] Customer Rowan Vale; email=rowan.vale@example.invalid; private date=1984-03-12; account=CUST-2048; credential=" + credential + "; [/message]\n",
		"export":           `{"customer":"Rowan Vale","email":"rowan.vale@example.invalid","account":"CUST-2048","credential":"` + credential + `","public_release":"2026-07-17"}` + "\n",
		"adversarial-long": "مستخدم Rowan Vale 顧客 さくら 고객 आरव café e\u0301 👩🏽‍💻 credential=" + credential + " boundary\u200dtext\n",
		"mcp-longevity":    `{"tool":"inbox_search","query":"Rowan Vale ` + credential + `","email":"rowan.vale@example.invalid"}` + "\n",
		"mcp-burst":        `{"request":"concurrent","customer":"Rowan Vale","credential":"` + credential + `"}` + "\n",
	}
	pattern, exists := patterns[workload.ID]
	if !exists {
		return nil, fmt.Errorf("performance workload %q has no deterministic payload recipe", workload.ID)
	}
	header := fmt.Sprintf("HS-PRIVACY-WORKLOAD/%s/v%d\n", workload.ID, workload.GeneratorVersion)
	payload := make([]byte, 0, workload.TargetBytes)
	payload = append(payload, header...)
	for len(payload)+len(pattern) <= workload.TargetBytes {
		payload = append(payload, pattern...)
	}
	for _, character := range pattern {
		encoded := []byte(string(character))
		if len(payload)+len(encoded) > workload.TargetBytes {
			break
		}
		payload = append(payload, encoded...)
	}
	for len(payload) < workload.TargetBytes {
		payload = append(payload, 'x')
	}
	return payload, nil
}

func performancePayloadSHA256(workload PerformanceWorkload) (string, error) {
	payload, err := GeneratePerformancePayload(workload)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:]), nil
}

func LoadPerformanceContract(workloadsPath, budgetsPath string) (*WorkloadsDocument, *BudgetsDocument, error) {
	workloadsRaw, err := os.ReadFile(workloadsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read performance workloads: %w", err)
	}
	var workloads WorkloadsDocument
	if err := decodeStrict(workloadsRaw, &workloads); err != nil {
		return nil, nil, fmt.Errorf("decode performance workloads: %w", err)
	}
	budgetsRaw, err := os.ReadFile(budgetsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read performance budgets: %w", err)
	}
	var budgets BudgetsDocument
	if err := decodeStrict(budgetsRaw, &budgets); err != nil {
		return nil, nil, fmt.Errorf("decode performance budgets: %w", err)
	}
	if err := validatePerformanceContract(&workloads, &budgets); err != nil {
		return nil, nil, err
	}
	return &workloads, &budgets, nil
}

func validatePerformanceContract(workloads *WorkloadsDocument, budgets *BudgetsDocument) error {
	if workloads.Schema != CorpusSchemaVersion || budgets.Schema != CorpusSchemaVersion {
		return fmt.Errorf("performance contract requires schema %d", CorpusSchemaVersion)
	}
	if budgets.EvidenceAuthority != "docker-ci" {
		return fmt.Errorf("performance budgets must name docker-ci as evidence authority")
	}
	if budgets.HardwareIdentityFile != "hardware-identities.json" {
		return fmt.Errorf("performance budgets must pin hardware-identities.json")
	}
	if budgets.Sampling.MinimumFreshProcesses < 20 || budgets.Sampling.MinimumWarmSamples < 100 || budgets.Sampling.DiagnosticReruns != 1 {
		return fmt.Errorf("performance sampling is below the frozen minimum")
	}
	expectedWorkloads := expectedPerformanceWorkloads()
	if len(workloads.Workloads) != len(expectedWorkloads) {
		return fmt.Errorf("performance contract requires exactly %d workloads", len(expectedWorkloads))
	}
	workloadIDs := make(map[string]struct{}, len(workloads.Workloads))
	for index, workload := range workloads.Workloads {
		if !idPattern.MatchString(workload.ID) || workload.Description == "" || workload.TargetBytes <= 0 {
			return fmt.Errorf("performance workload %q has invalid metadata", workload.ID)
		}
		if _, exists := workloadIDs[workload.ID]; exists {
			return fmt.Errorf("duplicate performance workload %q", workload.ID)
		}
		workloadIDs[workload.ID] = struct{}{}
		if err := validateUniqueStrings("performance workload "+workload.ID, "required_metrics", workload.RequiredMetrics); err != nil {
			return err
		}
		expected := expectedWorkloads[index]
		if workload.ID != expected.ID || workload.Description != expected.Description || workload.TargetBytes != expected.TargetBytes ||
			workload.WarmupSamples != expected.WarmupSamples || workload.MeasuredSamples != expected.MeasuredSamples ||
			workload.FreshProcesses != expected.FreshProcesses || workload.Concurrency != expected.Concurrency ||
			workload.Requests != expected.Requests || !slices.Equal(workload.RequiredMetrics, expected.RequiredMetrics) ||
			workload.Generator != expected.Generator || workload.GeneratorVersion != expected.GeneratorVersion ||
			workload.GeneratorSeed != expected.GeneratorSeed {
			return fmt.Errorf("performance workload %q differs from the exact frozen workload contract", workload.ID)
		}
		payloadHash, err := performancePayloadSHA256(workload)
		if err != nil {
			return fmt.Errorf("generate performance workload %q: %w", workload.ID, err)
		}
		if workload.PayloadSHA256 != payloadHash {
			return fmt.Errorf("performance workload %q payload hash differs from generated bytes", workload.ID)
		}
	}

	expectedProfiles := expectedHardwareProfiles()
	if len(budgets.HardwareProfiles) != len(expectedProfiles) {
		return fmt.Errorf("performance contract requires H0, H1, and H2 hardware profiles")
	}
	profiles := make(map[string]HardwareProfile, len(budgets.HardwareProfiles))
	for index, profile := range budgets.HardwareProfiles {
		if profile.ID != "H0" && profile.ID != "H1" && profile.ID != "H2" {
			return fmt.Errorf("unsupported hardware profile %q", profile.ID)
		}
		if _, exists := profiles[profile.ID]; exists {
			return fmt.Errorf("duplicate hardware profile %q", profile.ID)
		}
		expected := expectedProfiles[index]
		if profile.ID != expected.ID || !slices.Equal(profile.RunnerLabels, expected.RunnerLabels) ||
			profile.VCPUs != expected.VCPUs || profile.MemoryBytes != expected.MemoryBytes || profile.SwapBytes != expected.SwapBytes ||
			profile.CPUOnly != expected.CPUOnly || profile.Storage != expected.Storage ||
			!slices.Equal(profile.RequiredMetadata, expected.RequiredMetadata) {
			return fmt.Errorf("hardware profile %q differs from the exact frozen profile", profile.ID)
		}
		if profile.VCPUs <= 0 || profile.MemoryBytes <= 0 || !profile.CPUOnly || profile.Storage != "ssd" {
			return fmt.Errorf("hardware profile %q has invalid resources", profile.ID)
		}
		if len(profile.RunnerLabels) == 0 || contains(profile.RunnerLabels, "ubuntu-latest") {
			return fmt.Errorf("hardware profile %q must use a named stable CI runner", profile.ID)
		}
		hasNamedLabel := false
		for _, label := range profile.RunnerLabels {
			if strings.Contains(label, "privacy-filter-") {
				hasNamedLabel = true
			}
		}
		if !hasNamedLabel {
			return fmt.Errorf("hardware profile %q lacks its stable privacy-filter runner label", profile.ID)
		}
		for _, required := range []string{
			"runner_name", "cpu_model", "architecture", "instruction_sets", "physical_cores", "logical_cores",
			"virtualization", "ram_limit", "swap_policy", "numa", "storage", "benchmark_workspace", "os", "kernel", "power_governor", "thermal_state",
			"onnx_runtime", "go_version", "python_version", "thread_settings", "container_limits", "git_commit", "corpus_sha256",
			"budget_sha256", "model_sha256",
		} {
			if !contains(profile.RequiredMetadata, required) {
				return fmt.Errorf("hardware profile %q is missing reproduction metadata %q", profile.ID, required)
			}
		}
		profiles[profile.ID] = profile
	}
	if profiles["H0"].VCPUs != 2 || profiles["H0"].MemoryBytes != 4*1024*1024*1024 || profiles["H0"].SwapBytes != 0 ||
		profiles["H1"].VCPUs != 4 || profiles["H1"].MemoryBytes != 8*1024*1024*1024 ||
		profiles["H2"].VCPUs != 8 || profiles["H2"].MemoryBytes != 16*1024*1024*1024 {
		return fmt.Errorf("hardware profiles do not match the frozen H0/H1/H2 resources")
	}

	expectedLimits := expectedPerformanceLimits()
	if len(budgets.Limits) != len(expectedLimits) {
		return fmt.Errorf("performance contract expected %d frozen limits", len(expectedLimits))
	}
	seenLimits := make(map[string]struct{}, len(budgets.Limits))
	for index, limit := range budgets.Limits {
		if _, exists := seenLimits[limit.ID]; exists {
			return fmt.Errorf("duplicate performance limit %q", limit.ID)
		}
		seenLimits[limit.ID] = struct{}{}
		expected := expectedLimits[index]
		if limit.ID != expected.ID || limit.Workload != expected.Workload || limit.Statistic != expected.Statistic ||
			limit.Maximum != expected.Maximum || limit.Unit != expected.Unit || !slices.Equal(limit.Profiles, expected.Profiles) {
			return fmt.Errorf("performance limit %q differs from the frozen budget", limit.ID)
		}
		for _, profile := range limit.Profiles {
			if _, exists := profiles[profile]; !exists {
				return fmt.Errorf("performance limit %q references unknown profile %q", limit.ID, profile)
			}
		}
		if limit.Workload != "all" {
			if _, exists := workloadIDs[limit.Workload]; !exists {
				return fmt.Errorf("performance limit %q references unknown workload %q", limit.ID, limit.Workload)
			}
		}
	}
	return nil
}
