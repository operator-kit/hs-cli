package evaluation

import (
	"fmt"
	"os"
)

const SecretMarker = "[REDACTED_SECRET]"

type Replacement string

const (
	ReplacementPassThrough         Replacement = "pass-through"
	ReplacementDeterministicPerson Replacement = "deterministic-person"
	ReplacementDeterministicToken  Replacement = "deterministic-token"
	ReplacementConstantSecret      Replacement = "constant-secret-marker"
)

type PolicyDocument struct {
	Schema       int              `json:"schema"`
	SecretMarker string           `json:"secret_marker"`
	Kinds        []KindPolicy     `json:"kinds"`
	Identity     []IdentityPolicy `json:"identity_policy"`
}

type KindPolicy struct {
	Kind        SpanKind    `json:"kind"`
	Replacement Replacement `json:"replacement"`
	Actions     ModeActions `json:"actions"`
}

type IdentityPolicy struct {
	ID           string      `json:"id"`
	Kind         SpanKind    `json:"kind"`
	IdentityType string      `json:"identity_type"`
	Replacement  Replacement `json:"replacement"`
	Actions      ModeActions `json:"actions"`
}

func LoadPolicy(path string) (*PolicyDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read typed privacy policy: %w", err)
	}
	var policy PolicyDocument
	if err := decodeStrict(raw, &policy); err != nil {
		return nil, fmt.Errorf("decode typed privacy policy: %w", err)
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &policy, nil
}

func (p *PolicyDocument) Validate() error {
	if p.Schema != CorpusSchemaVersion {
		return fmt.Errorf("typed privacy policy: unsupported schema %d", p.Schema)
	}
	if p.SecretMarker != SecretMarker {
		return fmt.Errorf("typed privacy policy: secret marker must be the frozen constant %q", SecretMarker)
	}
	if len(p.Kinds) != len(SpanKinds) {
		return fmt.Errorf("typed privacy policy: expected exactly %d kind policies", len(SpanKinds))
	}
	seenKinds := make(map[SpanKind]struct{}, len(p.Kinds))
	for _, policy := range p.Kinds {
		if !allowedKinds[policy.Kind] {
			return fmt.Errorf("typed privacy policy: unsupported kind %q", policy.Kind)
		}
		if _, exists := seenKinds[policy.Kind]; exists {
			return fmt.Errorf("typed privacy policy: duplicate kind %q", policy.Kind)
		}
		seenKinds[policy.Kind] = struct{}{}
		if policy.Actions.Off != ActionPreserve || policy.Actions.Customers != ActionRedact || policy.Actions.All != ActionRedact {
			return fmt.Errorf("typed privacy policy: default kind %q must preserve off and redact customers/all", policy.Kind)
		}
		expectedReplacement := ReplacementDeterministicToken
		switch policy.Kind {
		case SpanPerson:
			expectedReplacement = ReplacementDeterministicPerson
		case SpanSecret:
			expectedReplacement = ReplacementConstantSecret
		}
		if policy.Replacement != expectedReplacement {
			return fmt.Errorf("typed privacy policy: kind %q has replacement %q, want %q", policy.Kind, policy.Replacement, expectedReplacement)
		}
	}

	requiredIdentityPolicies := map[string]IdentityPolicy{
		"known-customer-person": {
			Kind: SpanPerson, IdentityType: "customer", Replacement: ReplacementDeterministicPerson,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact},
		},
		"known-staff-person": {
			Kind: SpanPerson, IdentityType: "user", Replacement: ReplacementDeterministicPerson,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionRedact},
		},
		"unknown-third-party-person": {
			Kind: SpanPerson, IdentityType: "unknown", Replacement: ReplacementDeterministicPerson,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact},
		},
		"customer-secret": {
			Kind: SpanSecret, IdentityType: "customer", Replacement: ReplacementConstantSecret,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact},
		},
		"staff-secret": {
			Kind: SpanSecret, IdentityType: "user", Replacement: ReplacementConstantSecret,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact},
		},
		"third-party-secret": {
			Kind: SpanSecret, IdentityType: "unknown", Replacement: ReplacementConstantSecret,
			Actions: ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact},
		},
	}
	if len(p.Identity) != len(requiredIdentityPolicies) {
		return fmt.Errorf("typed privacy policy: expected exactly %d identity policies", len(requiredIdentityPolicies))
	}
	seenIdentity := make(map[string]struct{}, len(p.Identity))
	for _, policy := range p.Identity {
		if _, exists := seenIdentity[policy.ID]; exists {
			return fmt.Errorf("typed privacy policy: duplicate identity policy %q", policy.ID)
		}
		seenIdentity[policy.ID] = struct{}{}
		expected, exists := requiredIdentityPolicies[policy.ID]
		if !exists {
			return fmt.Errorf("typed privacy policy: unknown identity policy %q", policy.ID)
		}
		if policy.Kind != expected.Kind || policy.IdentityType != expected.IdentityType ||
			policy.Replacement != expected.Replacement || policy.Actions != expected.Actions {
			return fmt.Errorf("typed privacy policy: identity policy %q contradicts the frozen matrix", policy.ID)
		}
	}
	return nil
}

type IdentitySnapshot struct {
	Schema         int                    `json:"schema"`
	IdentitySchema string                 `json:"identity_schema"`
	KeyID          string                 `json:"key_id"`
	KeyFixture     string                 `json:"key_fixture"`
	Cases          []IdentitySnapshotCase `json:"cases"`
}

type IdentitySnapshotCase struct {
	ID       string         `json:"id"`
	Input    IdentityValues `json:"input"`
	Expected IdentityValues `json:"expected"`
}

type IdentityValues struct {
	First string `json:"first"`
	Last  string `json:"last"`
	Email string `json:"email"`
}

func LoadIdentitySnapshot(path string) (*IdentitySnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read identity compatibility snapshot: %w", err)
	}
	var snapshot IdentitySnapshot
	if err := decodeStrict(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("decode identity compatibility snapshot: %w", err)
	}
	if snapshot.Schema != CorpusSchemaVersion || snapshot.IdentitySchema != "v2" || snapshot.KeyID == "" || snapshot.KeyFixture == "" {
		return nil, fmt.Errorf("identity compatibility snapshot has invalid metadata")
	}
	if len(snapshot.Cases) == 0 {
		return nil, fmt.Errorf("identity compatibility snapshot is empty")
	}
	seen := make(map[string]struct{}, len(snapshot.Cases))
	for _, fixture := range snapshot.Cases {
		if !idPattern.MatchString(fixture.ID) {
			return nil, fmt.Errorf("identity compatibility snapshot contains an invalid case ID")
		}
		if _, exists := seen[fixture.ID]; exists {
			return nil, fmt.Errorf("identity compatibility snapshot contains duplicate case ID %q", fixture.ID)
		}
		seen[fixture.ID] = struct{}{}
		if fixture.Input.First == "" && fixture.Input.Last == "" && fixture.Input.Email == "" {
			return nil, fmt.Errorf("identity compatibility case %q has empty input", fixture.ID)
		}
		if fixture.Expected.First == "" && fixture.Expected.Last == "" && fixture.Expected.Email == "" {
			return nil, fmt.Errorf("identity compatibility case %q has empty expected output", fixture.ID)
		}
	}
	return &snapshot, nil
}
