package pii

import (
	"strings"
	"testing"
)

func TestNewSecretStringPreservesExplicitBytes(t *testing.T) {
	secret, err := NewSecretString("  stable-secret  ")
	if err != nil {
		t.Fatalf("NewSecretString returned error: %v", err)
	}
	pseudonym, err := NewPseudonymContext(secret, "test-v2")
	if err != nil {
		t.Fatalf("NewPseudonymContext returned error: %v", err)
	}
	engine, err := NewEngine(ModeAll, pseudonym)
	if err != nil {
		t.Fatalf("NewEngine returned error: %v", err)
	}

	first, last, email := engine.RedactPerson("Alice", "Smith", "ALICE@example.com ")
	if first != "Noel" || last != "Barnes [test-v2-DHMUKKXS]" || email != "noel.barnes-8362@anon.local" {
		t.Fatalf("explicit secret bytes changed: got %q %q <%s>", first, last, email)
	}
}

func TestNewSecretStringRejectsBlankValues(t *testing.T) {
	for _, raw := range []string{"", " ", "\t\r\n"} {
		if _, err := NewSecretString(raw); err == nil {
			t.Fatalf("NewSecretString(%q) returned no error", raw)
		}
	}
}

func TestNewEngineRequiresSecretOnlyWhenEnabled(t *testing.T) {
	if _, err := NewEngine(ModeAll, PseudonymContext{}); err == nil {
		t.Fatal("enabled engine accepted an empty pseudonym context")
	}
	if _, err := NewEngine(ModeCustomers, PseudonymContext{}); err == nil {
		t.Fatal("customers engine accepted an empty pseudonym context")
	}
	if _, err := NewEngine(ModeOff, PseudonymContext{}); err != nil {
		t.Fatalf("disabled engine rejected an empty pseudonym context: %v", err)
	}
}

func TestNewEngineRejectsInvalidMode(t *testing.T) {
	secret, err := NewSecretString("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	pseudonym, err := NewPseudonymContext(secret, "test-v2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewEngine(Mode(255), pseudonym)
	if err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("invalid mode error = %v", err)
	}
}
