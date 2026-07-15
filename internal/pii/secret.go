package pii

import (
	"fmt"
	"strings"
)

// Secret is validated key material used for deterministic PII pseudonyms. Its
// value is deliberately opaque so callers cannot accidentally log it.
type Secret struct {
	key string
}

// NewSecret copies arbitrary non-empty key bytes into an opaque Secret.
func NewSecret(value []byte) (Secret, error) {
	if len(value) == 0 {
		return Secret{}, fmt.Errorf("PII secret must not be empty")
	}
	return Secret{key: string(append([]byte(nil), value...))}, nil
}

// NewSecretString validates an operator-supplied secret without trimming it.
// Leading and trailing bytes are part of the established identity contract.
func NewSecretString(value string) (Secret, error) {
	if strings.TrimSpace(value) == "" {
		return Secret{}, fmt.Errorf("PII secret must not be blank")
	}
	return Secret{key: value}, nil
}

func (s Secret) IsZero() bool {
	return s.key == ""
}

func (s Secret) bytes() []byte {
	return []byte(s.key)
}
