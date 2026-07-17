package pii

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/operator-kit/hs-cli/internal/pii/evaluation"
)

const phase0IdentityKeyFixture = "phase0-synthetic-pseudonym-key"

type typedCorpusNameDetector struct {
	spans []NameSpan
}

func (d typedCorpusNameDetector) DetectNames(string) ([]NameSpan, error) {
	return append([]NameSpan(nil), d.spans...), nil
}

func TestExistingPIIContractAgainstTypedCorpus(t *testing.T) {
	corpus, err := evaluation.LoadCorpusDir(filepath.Join("testdata", "privacy-filter", "v1"))
	if err != nil {
		t.Fatalf("load Phase 0 typed corpus: %v", err)
	}
	exercised := 0
	for _, fixture := range corpus.Cases {
		if !evaluation.HasTag(fixture, "existing-contract") {
			continue
		}
		exercised++
		spans := make([]NameSpan, 0)
		for _, target := range fixture.Targets {
			if target.Kind == evaluation.SpanPerson && target.Actions.All == evaluation.ActionRedact {
				spans = append(spans, NameSpan{
					Text: fixture.Text[target.Start:target.End], Start: target.Start, End: target.End, Score: 1,
				})
			}
		}
		known := make([]KnownIdentity, 0, len(fixture.KnownIdentities))
		for _, identity := range fixture.KnownIdentities {
			known = append(known, KnownIdentity{
				Type: identity.Type, First: identity.First, Last: identity.Last, Email: identity.Email, Phone: identity.Phone,
			})
		}

		for _, fixtureMode := range evaluation.Modes {
			mode, err := ParseMode(string(fixtureMode))
			if err != nil {
				t.Fatalf("case %q mode %q: %v", fixture.ID, fixtureMode, err)
			}
			var pseudonym PseudonymContext
			if IsEnabled(mode) {
				secret, secretErr := NewSecretString("phase0-existing-contract-synthetic-key")
				if secretErr != nil {
					t.Fatalf("construct case %q key: %v", fixture.ID, secretErr)
				}
				pseudonym, err = NewPseudonymContext(secret, "privacy-filter-g0")
				if err != nil {
					t.Fatalf("construct case %q pseudonym: %v", fixture.ID, err)
				}
			}
			engine, err := NewEngine(mode, pseudonym, WithNER(typedCorpusNameDetector{spans: spans}))
			if err != nil {
				t.Fatalf("construct case %q engine: %v", fixture.ID, err)
			}
			output := engine.RedactText(fixture.Text, known)
			if fixtureMode == evaluation.ModeOff && output != fixture.Text {
				t.Errorf("case %q changed bytes in off mode", fixture.ID)
			}
			expected := fixture.Outputs.For(fixtureMode)
			for _, absent := range expected.RequiredAbsent {
				if strings.Contains(output, absent) {
					t.Errorf("case %q leaked required-absent target in %s mode", fixture.ID, fixtureMode)
				}
			}
			for _, present := range expected.RequiredPresent {
				if !strings.Contains(output, present) {
					t.Errorf("case %q failed required-presence target in %s mode", fixture.ID, fixtureMode)
				}
			}
		}
	}
	if exercised < 20 {
		t.Fatalf("typed corpus exercised too few existing-contract cases: %d", exercised)
	}
}

func TestPrivacyFilterG0IdentityCompatibilitySnapshot(t *testing.T) {
	snapshot, err := evaluation.LoadIdentitySnapshot(filepath.Join("testdata", "privacy-filter", "v1", "identity-compatibility.json"))
	if err != nil {
		t.Fatalf("load identity compatibility snapshot: %v", err)
	}
	if snapshot.KeyFixture != "phase0-synthetic-pseudonym-key-v1" {
		t.Fatalf("unknown identity key fixture %q", snapshot.KeyFixture)
	}
	secret, err := NewSecretString(phase0IdentityKeyFixture)
	if err != nil {
		t.Fatal(err)
	}
	pseudonym, err := NewPseudonymContext(secret, snapshot.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(ModeAll, pseudonym)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range snapshot.Cases {
		first, last, email := engine.RedactPerson(fixture.Input.First, fixture.Input.Last, fixture.Input.Email)
		if first != fixture.Expected.First || last != fixture.Expected.Last || email != fixture.Expected.Email {
			t.Errorf("identity compatibility case %q changed byte-for-byte output", fixture.ID)
		}
	}
}
