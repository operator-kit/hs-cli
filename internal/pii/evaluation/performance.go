package evaluation

import (
	"fmt"
	"os"
	"strings"
)

type WorkloadsDocument struct {
	Schema    int                   `json:"schema"`
	Workloads []PerformanceWorkload `json:"workloads"`
}

type PerformanceWorkload struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	TargetBytes     int      `json:"target_bytes"`
	WarmupSamples   int      `json:"warmup_samples"`
	MeasuredSamples int      `json:"measured_samples"`
	FreshProcesses  int      `json:"fresh_processes"`
	Concurrency     int      `json:"concurrency"`
	Requests        int      `json:"requests"`
	RequiredMetrics []string `json:"required_metrics"`
}

type BudgetsDocument struct {
	Schema            int                `json:"schema"`
	EvidenceAuthority string             `json:"evidence_authority"`
	Sampling          SamplingPolicy     `json:"sampling"`
	HardwareProfiles  []HardwareProfile  `json:"hardware_profiles"`
	Limits            []PerformanceLimit `json:"limits"`
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
	if budgets.Sampling.MinimumFreshProcesses < 20 || budgets.Sampling.MinimumWarmSamples < 100 || budgets.Sampling.DiagnosticReruns != 1 {
		return fmt.Errorf("performance sampling is below the frozen minimum")
	}
	if len(workloads.Workloads) != 7 {
		return fmt.Errorf("performance contract requires exactly seven workloads")
	}
	workloadIDs := make(map[string]struct{}, len(workloads.Workloads))
	for _, workload := range workloads.Workloads {
		if !idPattern.MatchString(workload.ID) || workload.Description == "" || workload.TargetBytes <= 0 {
			return fmt.Errorf("performance workload %q has invalid metadata", workload.ID)
		}
		if _, exists := workloadIDs[workload.ID]; exists {
			return fmt.Errorf("duplicate performance workload %q", workload.ID)
		}
		workloadIDs[workload.ID] = struct{}{}
		if workload.WarmupSamples < 1 || workload.MeasuredSamples < budgets.Sampling.MinimumWarmSamples ||
			workload.FreshProcesses < budgets.Sampling.MinimumFreshProcesses || workload.Concurrency < 1 || workload.Requests < 1 {
			return fmt.Errorf("performance workload %q is below the frozen sampling minimum", workload.ID)
		}
		if len(workload.RequiredMetrics) == 0 {
			return fmt.Errorf("performance workload %q has no required metrics", workload.ID)
		}
		if err := validateUniqueStrings("performance workload "+workload.ID, "required_metrics", workload.RequiredMetrics); err != nil {
			return err
		}
	}
	for _, required := range []string{"subject", "message", "thread", "export", "adversarial-long", "mcp-longevity", "mcp-burst"} {
		if _, exists := workloadIDs[required]; !exists {
			return fmt.Errorf("performance contract is missing workload %q", required)
		}
	}

	if len(budgets.HardwareProfiles) != 3 {
		return fmt.Errorf("performance contract requires H0, H1, and H2 hardware profiles")
	}
	profiles := make(map[string]HardwareProfile, len(budgets.HardwareProfiles))
	for _, profile := range budgets.HardwareProfiles {
		if profile.ID != "H0" && profile.ID != "H1" && profile.ID != "H2" {
			return fmt.Errorf("unsupported hardware profile %q", profile.ID)
		}
		if _, exists := profiles[profile.ID]; exists {
			return fmt.Errorf("duplicate hardware profile %q", profile.ID)
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
			"virtualization", "ram_limit", "swap_policy", "numa", "storage", "os", "kernel", "power_governor",
			"onnx_runtime", "go_version", "python_version", "thread_settings", "git_commit", "corpus_sha256",
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

	requiredLimits := map[string]struct {
		maximum int64
		unit    string
	}{
		"detector-readiness-reference": {5000, "milliseconds"},
		"detector-readiness-slowest":   {8000, "milliseconds"},
		"warm-subject":                 {250, "milliseconds"},
		"warm-message":                 {750, "milliseconds"},
		"warm-thread":                  {4000, "milliseconds"},
		"warm-export":                  {20000, "milliseconds"},
		"peak-rss":                     {2 * 1024 * 1024 * 1024, "bytes"},
		"steady-rss":                   {1536 * 1024 * 1024, "bytes"},
		"rss-growth-percent":           {5, "percent"},
		"rss-growth-bytes":             {64 * 1024 * 1024, "bytes"},
		"installed-bundle":             {1280 * 1024 * 1024, "bytes"},
		"compressed-download":          {1181116006, "bytes"},
		"detector-construction":        {1, "count"},
		"mode-off-construction":        {0, "count"},
		"mcp-burst-factor":             {5, "factor"},
	}
	if len(budgets.Limits) != len(requiredLimits) {
		return fmt.Errorf("performance contract expected %d frozen limits", len(requiredLimits))
	}
	seenLimits := make(map[string]struct{}, len(budgets.Limits))
	for _, limit := range budgets.Limits {
		expected, exists := requiredLimits[limit.ID]
		if !exists {
			return fmt.Errorf("unknown performance limit %q", limit.ID)
		}
		if _, exists := seenLimits[limit.ID]; exists {
			return fmt.Errorf("duplicate performance limit %q", limit.ID)
		}
		seenLimits[limit.ID] = struct{}{}
		if limit.Maximum != expected.maximum || limit.Unit != expected.unit || limit.Statistic == "" || len(limit.Profiles) == 0 {
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
