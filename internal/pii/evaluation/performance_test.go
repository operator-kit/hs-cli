package evaluation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func loadTestPerformanceContract(t *testing.T) (*WorkloadsDocument, *BudgetsDocument) {
	t.Helper()
	dir := filepath.Join(corpusDir(t), "performance")
	workloads, budgets, err := LoadPerformanceContract(filepath.Join(dir, "workloads.json"), filepath.Join(dir, "budgets.json"))
	if err != nil {
		t.Fatalf("load performance contract: %v", err)
	}
	return workloads, budgets
}

func TestPerformancePayloadsAreByteExactAndReproducible(t *testing.T) {
	workloads, _ := loadTestPerformanceContract(t)
	for _, workload := range workloads.Workloads {
		payload, err := GeneratePerformancePayload(workload)
		if err != nil {
			t.Fatalf("generate workload %q: %v", workload.ID, err)
		}
		if len(payload) != workload.TargetBytes {
			t.Fatalf("workload %q generated %d bytes, want %d", workload.ID, len(payload), workload.TargetBytes)
		}
		if !utf8.Valid(payload) {
			t.Fatalf("workload %q generated invalid UTF-8", workload.ID)
		}
		hash, err := performancePayloadSHA256(workload)
		if err != nil || hash != workload.PayloadSHA256 {
			t.Fatalf("workload %q hash mismatch: got %q want %q err=%v", workload.ID, hash, workload.PayloadSHA256, err)
		}
	}

	generated, err := GeneratePerformanceWorkloadsDocument()
	if err != nil {
		t.Fatal(err)
	}
	generatedRaw, err := json.MarshalIndent(generated, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	generatedRaw = append(generatedRaw, '\n')
	checkedRaw, err := os.ReadFile(filepath.Join(corpusDir(t), "performance", "workloads.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generatedRaw, checkedRaw) {
		t.Fatal("checked performance workloads are stale; run go run ./internal/pii/evaluation/cmd/performancegen -repo-root .")
	}
}

func TestPerformanceContractRejectsSemanticWeakening(t *testing.T) {
	workloads, budgets := loadTestPerformanceContract(t)

	weakenedWorkloads := *workloads
	weakenedWorkloads.Workloads = append([]PerformanceWorkload(nil), workloads.Workloads...)
	weakenedWorkloads.Workloads[1].TargetBytes++
	if err := validatePerformanceContract(&weakenedWorkloads, budgets); err == nil {
		t.Fatal("performance contract accepted a changed target size")
	}

	weakenedWorkloads = *workloads
	weakenedWorkloads.Workloads = append([]PerformanceWorkload(nil), workloads.Workloads...)
	weakenedWorkloads.Workloads[1].RequiredMetrics = append([]string(nil), workloads.Workloads[1].RequiredMetrics[1:]...)
	if err := validatePerformanceContract(&weakenedWorkloads, budgets); err == nil {
		t.Fatal("performance contract accepted a removed required metric")
	}

	weakenedBudgets := *budgets
	weakenedBudgets.Limits = append([]PerformanceLimit(nil), budgets.Limits...)
	weakenedBudgets.Limits[2].Statistic = "p99"
	if err := validatePerformanceContract(workloads, &weakenedBudgets); err == nil {
		t.Fatal("performance contract accepted a changed statistic")
	}

	weakenedBudgets = *budgets
	weakenedBudgets.Limits = append([]PerformanceLimit(nil), budgets.Limits...)
	weakenedBudgets.Limits[2].Profiles = []string{"H2"}
	if err := validatePerformanceContract(workloads, &weakenedBudgets); err == nil {
		t.Fatal("performance contract accepted weakened profile applicability")
	}
}

func TestHardwareIdentityContractExplicitlyBlocksAuthoritativeGatesUntilProvisioned(t *testing.T) {
	_, budgets := loadTestPerformanceContract(t)
	document, ready, err := LoadHardwareIdentityContract(
		filepath.Join(corpusDir(t), "performance", budgets.HardwareIdentityFile), budgets,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ready || document.Status != "unprovisioned" {
		t.Fatalf("repository must not invent H0/H1/H2 machine evidence: status=%q ready=%v", document.Status, ready)
	}
	metadata := testMetadata()
	metadata.EvidenceAuthority = AuthorityDockerCI
	metadata.Authoritative = true
	metadata.HardwareContractReady = false
	gates := evaluateGates(metadata, []OutputMetrics{{Mode: ModeOff}})
	if gates[0].State != GateNotRun || gates[0].Reason != "stable-hardware-identities-not-frozen" {
		t.Fatalf("unprovisioned stable hosts did not keep G0 not-run: %+v", gates[0])
	}
	if err := RequireGatePass(&Report{Gates: gates}, "G0"); err == nil {
		t.Fatal("unprovisioned hardware identities were allowed to pass authoritative G0")
	}
}

func TestReadyHardwareIdentitiesRequireConcreteFingerprintMatches(t *testing.T) {
	_, budgets := loadTestPerformanceContract(t)
	document := HardwareIdentityDocument{Schema: HardwareIdentitySchemaVersion, Status: "ready"}
	for _, profile := range budgets.HardwareProfiles {
		identity := ConcreteHardwareIdentity{
			ID: profile.ID, RunnerLabel: profile.RunnerLabels[len(profile.RunnerLabels)-1], RunnerName: "fixture-" + profile.ID,
			CPUModel: "Fixture CPU 1", Architecture: "x86_64", InstructionSets: []string{"avx2", "sse4.2"},
			PhysicalCores: profile.VCPUs, LogicalCores: profile.VCPUs, Virtualization: "fixture-hypervisor",
			RAMBytes: profile.MemoryBytes, SwapBytes: profile.SwapBytes, NUMA: "single-node", Storage: profile.Storage,
			BenchmarkWorkspace: "/fixture/benchmark", OS: "Fixture Linux", Kernel: "1.0.0",
			PowerGovernor: "performance", ThermalState: "controlled", ContainerVCPUs: profile.VCPUs,
			ContainerMemoryBytes: profile.MemoryBytes, ContainerSwapBytes: profile.SwapBytes,
		}
		fingerprint, err := hardwareFingerprint(identity)
		if err != nil {
			t.Fatal(err)
		}
		identity.FingerprintSHA256 = fingerprint
		document.Profiles = append(document.Profiles, identity)
	}
	ready, err := validateHardwareIdentityContract(&document, budgets)
	if err != nil || !ready {
		t.Fatalf("complete concrete hardware identities were rejected: ready=%v err=%v", ready, err)
	}
	document.Profiles[0].CPUModel = "silently changed CPU"
	if _, err := validateHardwareIdentityContract(&document, budgets); err == nil {
		t.Fatal("hardware identity accepted a changed CPU with a stale fingerprint")
	}
}

func TestApprovedBaselineAnchorRemainsExplicitlyNotRun(t *testing.T) {
	anchor, err := LoadBaselineAnchor(filepath.Join(corpusDir(t), "performance", "approved-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	if anchor.Status != "not-run" {
		t.Fatalf("baseline anchor must remain not-run until reviewed Docker CI evidence exists: %q", anchor.Status)
	}
}
