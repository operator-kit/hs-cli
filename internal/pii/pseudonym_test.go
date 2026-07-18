package pii

import "testing"

func TestNewPseudonymContextValidatesPublicKeyID(t *testing.T) {
	secret, err := NewSecretString("test-secret")
	if err != nil {
		t.Fatal(err)
	}

	context, err := NewPseudonymContext(secret, "  ROTATION-2  ")
	if err != nil {
		t.Fatalf("NewPseudonymContext returned error: %v", err)
	}
	if context.KeyID() != "rotation-2" {
		t.Fatalf("key ID = %q, want rotation-2", context.KeyID())
	}
	if context.Schema() != IdentitySchemaV2 {
		t.Fatalf("identity schema = %q, want %q", context.Schema(), IdentitySchemaV2)
	}

	for _, invalid := range []string{"", "contains spaces", "secret/value", "abcdefghijklmnopqrstuvwxyz1234567"} {
		if _, err := NewPseudonymContext(secret, invalid); err == nil {
			t.Fatalf("NewPseudonymContext accepted invalid key ID %q", invalid)
		}
	}
}
