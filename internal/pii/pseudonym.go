package pii

import (
	"fmt"
	"strings"
)

// IdentitySchema identifies the canonicalization contract used to derive
// deterministic identity keys. It is metadata only: already-canonical legacy
// inputs intentionally retain their existing HMAC input and pseudonym.
type IdentitySchema string

const IdentitySchemaV2 IdentitySchema = "v2"

// PseudonymContext owns the private key material and public rotation metadata
// used by an Engine. The secret remains opaque and the key ID is explicitly
// supplied rather than derived from secret material.
type PseudonymContext struct {
	secret Secret
	keyID  string
	schema IdentitySchema
}

func NewPseudonymContext(secret Secret, keyID string) (PseudonymContext, error) {
	if secret.IsZero() {
		return PseudonymContext{}, fmt.Errorf("pseudonym context requires a secret")
	}
	keyID = strings.ToLower(strings.TrimSpace(keyID))
	if err := validatePseudonymKeyID(keyID); err != nil {
		return PseudonymContext{}, err
	}
	return PseudonymContext{secret: secret, keyID: keyID, schema: IdentitySchemaV2}, nil
}

func validatePseudonymKeyID(value string) error {
	if len(value) < 1 || len(value) > 32 {
		return fmt.Errorf("PII key ID must contain 1 to 32 characters")
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("PII key ID may contain only lowercase letters, numbers, '.', '_' and '-'")
	}
	return nil
}

func (c PseudonymContext) IsZero() bool {
	return c.secret.IsZero() && c.keyID == "" && c.schema == ""
}

func (c PseudonymContext) Secret() Secret {
	return c.secret
}

func (c PseudonymContext) KeyID() string {
	return c.keyID
}

func (c PseudonymContext) Schema() IdentitySchema {
	return c.schema
}
