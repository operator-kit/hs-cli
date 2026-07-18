package evaluation

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func corpusDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "testdata", "privacy-filter", "v1")
}

func TestPrivacyCorpusSchemaIsStrictAndSelfConsistent(t *testing.T) {
	corpus, err := LoadCorpusDir(corpusDir(t))
	if err != nil {
		t.Fatalf("load strict privacy corpus: %v", err)
	}
	if len(corpus.Cases) < 70 {
		t.Fatalf("typed corpus unexpectedly small: %d cases", len(corpus.Cases))
	}
	if len(corpus.Fingerprint) != 64 {
		t.Fatalf("corpus fingerprint is not SHA-256: %q", corpus.Fingerprint)
	}

	schemaRaw, err := os.ReadFile(filepath.Join(corpusDir(t), "schema.json"))
	if err != nil {
		t.Fatalf("read corpus JSON schema: %v", err)
	}
	var schema map[string]any
	if err := decodeStrict(schemaRaw, &schema); err != nil {
		t.Fatalf("decode corpus JSON schema: %v", err)
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("corpus JSON schema root must reject unknown fields")
	}
	definitions, ok := schema["$defs"].(map[string]any)
	if !ok || len(definitions) == 0 {
		t.Fatalf("corpus JSON schema has no typed definitions")
	}
	for name, rawDefinition := range definitions {
		definition, ok := rawDefinition.(map[string]any)
		if !ok {
			t.Fatalf("schema definition %q is not an object", name)
		}
		if definition["type"] == "object" && definition["additionalProperties"] != false {
			t.Fatalf("schema definition %q must reject unknown fields", name)
		}
	}
	for _, partition := range RequiredPartitions {
		documentRaw, readErr := os.ReadFile(filepath.Join(corpusDir(t), partition+".json"))
		if readErr != nil {
			t.Fatalf("read corpus partition %q for schema validation: %v", partition, readErr)
		}
		if err := ValidateJSONDocument(schemaRaw, documentRaw); err != nil {
			t.Fatalf("execute corpus JSON schema for partition %q: %v", partition, err)
		}
	}

	t.Run("unknown fields", func(t *testing.T) {
		var document CorpusDocument
		err := decodeStrict([]byte(`{"schema":1,"partition":"secrets","cases":[],"unknown":true}`), &document)
		if err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("unknown field was not rejected: %v", err)
		}
	})
	t.Run("invalid UTF-8", func(t *testing.T) {
		var document CorpusDocument
		err := decodeStrict([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}, &document)
		if err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
			t.Fatalf("invalid UTF-8 was not rejected: %v", err)
		}
	})
	t.Run("invalid byte boundary", func(t *testing.T) {
		fixture := findCase(t, corpus, "multilingual-ar-person-redact")
		fixture.Targets[0].Start++
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "UTF-8 byte range") {
			t.Fatalf("invalid byte boundary was not rejected: %v", err)
		}
	})
	t.Run("contradictory overlap", func(t *testing.T) {
		fixture := findCase(t, corpus, "person-unknown-developer")
		contradiction := fixture.Targets[0]
		contradiction.ID = "contradictory-target"
		contradiction.Actions.Customers = ActionPreserve
		fixture.Targets = append(fixture.Targets, contradiction)
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "contradictory") {
			t.Fatalf("contradictory overlap was not rejected: %v", err)
		}
	})
	t.Run("known customer must redact in both protected modes", func(t *testing.T) {
		fixture := findCase(t, corpus, "person-known-customer")
		fixture.Targets[0].Actions.Customers = ActionPreserve
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "known customer") {
			t.Fatalf("known customer policy weakening was not rejected: %v", err)
		}
	})
	t.Run("known staff must redact in all mode", func(t *testing.T) {
		fixture := findCase(t, corpus, "person-known-staff")
		fixture.Targets[0].Actions.All = ActionPreserve
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "known staff") {
			t.Fatalf("known staff all-mode weakening was not rejected: %v", err)
		}
	})
	t.Run("one independently exercised output boundary per case", func(t *testing.T) {
		fixture := findCase(t, corpus, "command-boundary-table-must-detect")
		fixture.Tags = append(fixture.Tags, "boundary:json")
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "at most one output boundary") {
			t.Fatalf("multi-boundary tag shortcut was not rejected: %v", err)
		}
	})
	t.Run("must-detect purpose binds actions without secret fixture metadata", func(t *testing.T) {
		fixture := findCase(t, corpus, "command-boundary-table-must-detect")
		fixture.Targets[1].Actions.Customers = ActionPreserve
		fixture.Targets[1].Actions.All = ActionPreserve
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "must-detect") {
			t.Fatalf("must-detect purpose weakening was not rejected: %v", err)
		}
	})
	t.Run("preservation purpose binds actions without secret fixture metadata", func(t *testing.T) {
		fixture := findCase(t, corpus, "command-boundary-table-preserve")
		fixture.Targets[1].Actions.Customers = ActionRedact
		fixture.Targets[1].Actions.All = ActionRedact
		if err := validateCase(&fixture); err == nil || !strings.Contains(err.Error(), "preservation secret") {
			t.Fatalf("preservation purpose weakening was not rejected: %v", err)
		}
	})
	t.Run("duplicate case IDs", func(t *testing.T) {
		tempDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tempDir, "schema.json"), schemaRaw, 0o600); err != nil {
			t.Fatalf("write schema copy: %v", err)
		}
		for _, partition := range RequiredPartitions {
			raw, readErr := os.ReadFile(filepath.Join(corpusDir(t), partition+".json"))
			if readErr != nil {
				t.Fatalf("read partition copy: %v", readErr)
			}
			if partition == RequiredPartitions[1] {
				var document CorpusDocument
				if err := json.Unmarshal(raw, &document); err != nil {
					t.Fatalf("decode partition copy: %v", err)
				}
				document.Cases[0].ID = corpus.Documents[0].Cases[0].ID
				raw, readErr = json.Marshal(document)
				if readErr != nil {
					t.Fatalf("encode partition copy: %v", readErr)
				}
			}
			if err := os.WriteFile(filepath.Join(tempDir, partition+".json"), raw, 0o600); err != nil {
				t.Fatalf("write partition copy: %v", err)
			}
		}
		if _, err := LoadCorpusDir(tempDir); err == nil || !strings.Contains(err.Error(), "duplicate case ID") {
			t.Fatalf("duplicate case ID was not rejected: %v", err)
		}
	})
}

func TestAdversarialSecretFixturesFreezeRepeatedOverlapAndOwnershipSemantics(t *testing.T) {
	corpus, err := LoadCorpusDir(corpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	repeated := findCase(t, corpus, "command-secret-repeated")
	if len(repeated.Targets) != 2 || repeated.Targets[0].Value != repeated.Targets[1].Value || repeated.Targets[0].Start == repeated.Targets[1].Start {
		t.Fatalf("repeated-secret fixture must contain two spans for one value: %+v", repeated.Targets)
	}
	for _, mode := range []Mode{ModeCustomers, ModeAll} {
		if got := repeated.Outputs.For(mode).RequiredAbsent; len(got) != 1 || got[0] != repeated.Targets[0].Value {
			t.Fatalf("repeated-secret %s leak sentinel is not deduplicated: %v", mode, got)
		}
	}
	overlap := findCase(t, corpus, "command-secret-overlapping-account")
	if len(overlap.Targets) != 2 || overlap.Targets[0].Start > overlap.Targets[1].Start || overlap.Targets[0].End < overlap.Targets[1].End {
		t.Fatalf("overlapping secret/account fixture is not genuinely overlapping: %+v", overlap.Targets)
	}
	for _, mode := range []Mode{ModeCustomers, ModeAll} {
		if !contains(overlap.Outputs.For(mode).RequiredAbsent, overlap.Targets[1].Value) {
			t.Fatalf("overlapping fixture lacks its fragment sentinel in %s", mode)
		}
	}
	owned := findCase(t, corpus, "person-developer-owned-secret")
	if len(owned.KnownIdentities) != 1 || len(owned.Targets) != 3 || owned.Targets[2].Kind != SpanSecret {
		t.Fatalf("developer-owned-secret scenario is incomplete: %+v", owned)
	}
}

func TestPrivacyCorpusContainsRequiredRiskLanguageAndFormatSlices(t *testing.T) {
	corpus, err := LoadCorpusDir(corpusDir(t))
	if err != nil {
		t.Fatalf("load typed privacy corpus: %v", err)
	}
	if err := corpus.ValidateCoverage(); err != nil {
		t.Fatalf("typed privacy corpus coverage: %v", err)
	}

	risks := map[RiskTier]bool{}
	existingContractCases := 0
	for _, fixture := range corpus.Cases {
		risks[fixture.Risk] = true
		if HasTag(fixture, "existing-contract") {
			existingContractCases++
		}
	}
	for _, risk := range []RiskTier{RiskCritical, RiskHigh, RiskPreservation} {
		if !risks[risk] {
			t.Fatalf("typed corpus is missing risk tier %q", risk)
		}
	}
	if existingContractCases < 20 {
		t.Fatalf("too few behavior-preserving baseline cases: %d", existingContractCases)
	}
}

func TestSecretFixturesContainOnlyDeclaredSyntheticMaterial(t *testing.T) {
	corpus, err := LoadCorpusDir(corpusDir(t))
	if err != nil {
		t.Fatalf("load typed privacy corpus: %v", err)
	}
	families := make(map[string]map[string]bool)
	secretTargets := 0
	for _, fixture := range corpus.Cases {
		if fixture.SecretFixture != nil {
			if families[fixture.SecretFixture.Family] == nil {
				families[fixture.SecretFixture.Family] = map[string]bool{}
			}
			families[fixture.SecretFixture.Family][fixture.SecretFixture.Role] = true
		}
		for _, target := range fixture.Targets {
			if target.Kind != SpanSecret {
				continue
			}
			secretTargets++
			if target.Synthetic == nil {
				t.Fatalf("case %q secret target %q lacks synthetic provenance", fixture.ID, target.ID)
			}
			generated, generationErr := GenerateSyntheticValue(*target.Synthetic)
			if generationErr != nil || generated != target.Value {
				t.Fatalf("case %q secret target %q does not match its deterministic provenance: %v", fixture.ID, target.ID, generationErr)
			}
		}
	}
	if secretTargets < len(RequiredSecretFamilies)*2 {
		t.Fatalf("typed corpus has too few secret fixtures: %d", secretTargets)
	}
	for _, family := range RequiredSecretFamilies {
		if !families[family]["must-detect"] || !families[family]["preserve"] {
			t.Fatalf("secret family %q lacks must-detect and preservation fixtures", family)
		}
	}
}

func TestTypedPolicyMatrixCoversEveryModeAndSpanKind(t *testing.T) {
	policy, err := LoadPolicy(filepath.Join(corpusDir(t), "policy.json"))
	if err != nil {
		t.Fatalf("load typed privacy policy: %v", err)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("validate typed privacy policy: %v", err)
	}
	if policy.SecretMarker != SecretMarker {
		t.Fatalf("secret marker changed: %q", policy.SecretMarker)
	}
}

func TestPerformanceBudgetsSchemaAndHardwareProfile(t *testing.T) {
	performanceDir := filepath.Join(corpusDir(t), "performance")
	_, _, err := LoadPerformanceContract(
		filepath.Join(performanceDir, "workloads.json"),
		filepath.Join(performanceDir, "budgets.json"),
	)
	if err != nil {
		t.Fatalf("load frozen performance contract: %v", err)
	}
}

func TestIdentityCompatibilitySnapshotIsStrict(t *testing.T) {
	snapshot, err := LoadIdentitySnapshot(filepath.Join(corpusDir(t), "identity-compatibility.json"))
	if err != nil {
		t.Fatalf("load deterministic identity snapshot: %v", err)
	}
	if len(snapshot.Cases) < 8 {
		t.Fatalf("identity compatibility snapshot is too small: %d", len(snapshot.Cases))
	}
	raw, err := os.ReadFile(filepath.Join(corpusDir(t), "identity-compatibility.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("stable-secret")) {
		t.Fatalf("identity snapshot must identify, not embed, its synthetic key fixture")
	}
}

func findCase(t *testing.T, corpus *Corpus, id string) Case {
	t.Helper()
	for _, fixture := range corpus.Cases {
		if fixture.ID == id {
			return fixture
		}
	}
	t.Fatalf("corpus case %q not found", id)
	return Case{}
}
