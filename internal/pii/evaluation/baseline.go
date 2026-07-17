package evaluation

import (
	"encoding/hex"
	"fmt"
	"os"
)

const BaselineAnchorSchemaVersion = 1

type BaselineAnchor struct {
	Schema            int    `json:"schema"`
	Status            string `json:"status"`
	Reason            string `json:"reason"`
	GitCommit         string `json:"git_commit"`
	WorkflowRunURL    string `json:"workflow_run_url"`
	ReportSHA256      string `json:"report_sha256"`
	ContainerImage    string `json:"container_image"`
	SmokeCorpusSHA256 string `json:"smoke_corpus_sha256"`
	BroadCorpusSHA256 string `json:"broad_corpus_sha256"`
	BudgetSHA256      string `json:"budget_sha256"`
	HardwareSHA256    string `json:"hardware_sha256"`
}

func LoadBaselineAnchor(path string) (*BaselineAnchor, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline anchor: %w", err)
	}
	var anchor BaselineAnchor
	if err := decodeStrict(raw, &anchor); err != nil {
		return nil, fmt.Errorf("decode baseline anchor: %w", err)
	}
	if err := anchor.Validate(); err != nil {
		return nil, err
	}
	return &anchor, nil
}

func (anchor BaselineAnchor) Validate() error {
	if anchor.Schema != BaselineAnchorSchemaVersion {
		return fmt.Errorf("baseline anchor requires schema %d", BaselineAnchorSchemaVersion)
	}
	if anchor.Status == "not-run" {
		if anchor.Reason != "awaiting-authoritative-docker-ci-and-stable-host-provisioning" {
			return fmt.Errorf("not-run baseline anchor has an unexpected reason")
		}
		if anchor.GitCommit != "" || anchor.WorkflowRunURL != "" || anchor.ReportSHA256 != "" || anchor.ContainerImage != "" ||
			anchor.SmokeCorpusSHA256 != "" || anchor.BroadCorpusSHA256 != "" || anchor.BudgetSHA256 != "" || anchor.HardwareSHA256 != "" {
			return fmt.Errorf("not-run baseline anchor must not contain invented evidence identities")
		}
		return nil
	}
	if anchor.Status != "approved" || anchor.Reason != "reviewed-authoritative-g0-baseline" {
		return fmt.Errorf("baseline anchor has unsupported status or reason")
	}
	for name, value := range map[string]string{
		"git_commit": anchor.GitCommit, "workflow_run_url": anchor.WorkflowRunURL, "container_image": anchor.ContainerImage,
	} {
		if value == "" {
			return fmt.Errorf("approved baseline anchor is missing %s", name)
		}
	}
	for name, value := range map[string]string{
		"report": anchor.ReportSHA256, "smoke corpus": anchor.SmokeCorpusSHA256, "broad corpus": anchor.BroadCorpusSHA256,
		"budget": anchor.BudgetSHA256, "hardware": anchor.HardwareSHA256,
	} {
		if len(value) != sha256HexLength {
			return fmt.Errorf("approved baseline anchor %s identity is not SHA-256", name)
		}
		if _, err := hex.DecodeString(value); err != nil {
			return fmt.Errorf("approved baseline anchor %s identity is not SHA-256", name)
		}
	}
	return nil
}
