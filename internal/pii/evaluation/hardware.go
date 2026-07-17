package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

const HardwareIdentitySchemaVersion = 1

type HardwareIdentityDocument struct {
	Schema   int                        `json:"schema"`
	Status   string                     `json:"status"`
	Profiles []ConcreteHardwareIdentity `json:"profiles"`
}

type ConcreteHardwareIdentity struct {
	ID                   string   `json:"id"`
	RunnerLabel          string   `json:"runner_label"`
	RunnerName           string   `json:"runner_name"`
	CPUModel             string   `json:"cpu_model"`
	Architecture         string   `json:"architecture"`
	InstructionSets      []string `json:"instruction_sets"`
	PhysicalCores        int      `json:"physical_cores"`
	LogicalCores         int      `json:"logical_cores"`
	Virtualization       string   `json:"virtualization"`
	RAMBytes             int64    `json:"ram_bytes"`
	SwapBytes            int64    `json:"swap_bytes"`
	NUMA                 string   `json:"numa"`
	Storage              string   `json:"storage"`
	BenchmarkWorkspace   string   `json:"benchmark_workspace"`
	OS                   string   `json:"os"`
	Kernel               string   `json:"kernel"`
	PowerGovernor        string   `json:"power_governor"`
	ThermalState         string   `json:"thermal_state"`
	ContainerVCPUs       int      `json:"container_vcpus"`
	ContainerMemoryBytes int64    `json:"container_memory_bytes"`
	ContainerSwapBytes   int64    `json:"container_swap_bytes"`
	FingerprintSHA256    string   `json:"fingerprint_sha256"`
}

func LoadHardwareIdentityContract(path string, budgets *BudgetsDocument) (*HardwareIdentityDocument, bool, error) {
	if budgets == nil {
		return nil, false, fmt.Errorf("hardware identity contract requires performance budgets")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read hardware identity contract: %w", err)
	}
	var document HardwareIdentityDocument
	if err := decodeStrict(raw, &document); err != nil {
		return nil, false, fmt.Errorf("decode hardware identity contract: %w", err)
	}
	ready, err := validateHardwareIdentityContract(&document, budgets)
	if err != nil {
		return nil, false, err
	}
	return &document, ready, nil
}

func validateHardwareIdentityContract(document *HardwareIdentityDocument, budgets *BudgetsDocument) (bool, error) {
	if document.Schema != HardwareIdentitySchemaVersion {
		return false, fmt.Errorf("hardware identity contract requires schema %d", HardwareIdentitySchemaVersion)
	}
	if document.Status != "unprovisioned" && document.Status != "ready" {
		return false, fmt.Errorf("hardware identity contract has unsupported status %q", document.Status)
	}
	if len(document.Profiles) != len(budgets.HardwareProfiles) {
		return false, fmt.Errorf("hardware identity contract must describe every budget profile")
	}
	for index, identity := range document.Profiles {
		profile := budgets.HardwareProfiles[index]
		if identity.ID != profile.ID || identity.RunnerLabel != profile.RunnerLabels[len(profile.RunnerLabels)-1] {
			return false, fmt.Errorf("hardware identity %d does not match budget profile %q", index, profile.ID)
		}
		if document.Status == "unprovisioned" {
			if !hardwareIdentityIsEmpty(identity) {
				return false, fmt.Errorf("unprovisioned hardware identity %q contains invented machine evidence", identity.ID)
			}
			continue
		}
		if err := validateReadyHardwareIdentity(identity, profile); err != nil {
			return false, err
		}
	}
	return document.Status == "ready", nil
}

func hardwareIdentityIsEmpty(identity ConcreteHardwareIdentity) bool {
	return identity.RunnerName == "" && identity.CPUModel == "" && identity.Architecture == "" &&
		len(identity.InstructionSets) == 0 && identity.PhysicalCores == 0 && identity.LogicalCores == 0 &&
		identity.Virtualization == "" && identity.RAMBytes == 0 && identity.SwapBytes == 0 && identity.NUMA == "" &&
		identity.Storage == "" && identity.BenchmarkWorkspace == "" && identity.OS == "" && identity.Kernel == "" &&
		identity.PowerGovernor == "" && identity.ThermalState == "" && identity.ContainerVCPUs == 0 &&
		identity.ContainerMemoryBytes == 0 && identity.ContainerSwapBytes == 0 && identity.FingerprintSHA256 == ""
}

func validateReadyHardwareIdentity(identity ConcreteHardwareIdentity, profile HardwareProfile) error {
	for name, value := range map[string]string{
		"runner_name": identity.RunnerName, "cpu_model": identity.CPUModel, "architecture": identity.Architecture,
		"virtualization": identity.Virtualization, "numa": identity.NUMA, "storage": identity.Storage,
		"benchmark_workspace": identity.BenchmarkWorkspace, "os": identity.OS, "kernel": identity.Kernel,
		"power_governor": identity.PowerGovernor, "thermal_state": identity.ThermalState,
	} {
		if value == "" {
			return fmt.Errorf("ready hardware identity %q is missing %s", identity.ID, name)
		}
	}
	if len(identity.InstructionSets) == 0 || !slices.IsSorted(identity.InstructionSets) {
		return fmt.Errorf("ready hardware identity %q requires sorted instruction sets", identity.ID)
	}
	if identity.PhysicalCores <= 0 || identity.LogicalCores < identity.PhysicalCores || identity.RAMBytes <= 0 ||
		identity.ContainerVCPUs != profile.VCPUs || identity.ContainerMemoryBytes != profile.MemoryBytes ||
		identity.ContainerSwapBytes != profile.SwapBytes || identity.Storage != profile.Storage {
		return fmt.Errorf("ready hardware identity %q contradicts its frozen resource profile", identity.ID)
	}
	expected, err := hardwareFingerprint(identity)
	if err != nil {
		return err
	}
	if identity.FingerprintSHA256 != expected {
		return fmt.Errorf("ready hardware identity %q fingerprint does not match its concrete machine fields", identity.ID)
	}
	return nil
}

func hardwareFingerprint(identity ConcreteHardwareIdentity) (string, error) {
	identity.FingerprintSHA256 = ""
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("encode hardware identity fingerprint: %w", err)
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}
