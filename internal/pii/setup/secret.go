package setup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/operator-kit/hs-cli/internal/pii"
)

const (
	EnvSecret             = "HS_INBOX_PII_SECRET"
	EnvKeyID              = "HS_INBOX_PII_KEY_ID"
	GeneratedSecretBytes  = 32
	GeneratedKeyIDBytes   = 5
	StoredPseudonymSchema = 1
	generatedKeyIDPrefix  = "k-"
)

var ErrSecretNotFound = errors.New("PII secret not found")

type SecretStore interface {
	Load(context.Context) ([]byte, error)
	Save(context.Context, []byte) error
}

type InitializationLock interface {
	WithLock(context.Context, func() error) error
}

// SecretResolver turns operator configuration or installation-local key
// material into the versioned pseudonym context required by an enabled engine.
type SecretResolver struct {
	Store     SecretStore
	Lock      InitializationLock
	Random    io.Reader
	LookupEnv func(string) (string, bool)
}

type storedPseudonymRecord struct {
	Schema         int                `json:"schema"`
	IdentitySchema pii.IdentitySchema `json:"identity_schema"`
	KeyID          string             `json:"key_id"`
	Secret         string             `json:"secret"`
}

type decodedStoredPseudonym struct {
	context      pii.PseudonymContext
	legacySecret []byte
}

func (r *SecretResolver) ResolveContext(ctx context.Context, mode pii.Mode) (pii.PseudonymContext, error) {
	if !mode.Valid() {
		return pii.PseudonymContext{}, fmt.Errorf("invalid PII mode")
	}
	if !pii.IsEnabled(mode) {
		return pii.PseudonymContext{}, nil
	}

	lookup := r.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	rawSecret, secretSet := lookup(EnvSecret)
	rawKeyID, keyIDSet := lookup(EnvKeyID)
	if secretSet || keyIDSet {
		if !secretSet || !keyIDSet || strings.TrimSpace(rawSecret) == "" || strings.TrimSpace(rawKeyID) == "" {
			return pii.PseudonymContext{}, unavailableSecretError()
		}
		secret, err := pii.NewSecretString(rawSecret)
		if err != nil {
			return pii.PseudonymContext{}, unavailableSecretError()
		}
		pseudonym, err := pii.NewPseudonymContext(secret, rawKeyID)
		if err != nil {
			return pii.PseudonymContext{}, unavailableSecretError()
		}
		return pseudonym, nil
	}

	if r.Store == nil {
		return pii.PseudonymContext{}, unavailableSecretError()
	}
	raw, err := r.Store.Load(ctx)
	switch {
	case err == nil:
		stored, decodeErr := decodeStoredPseudonym(raw)
		if decodeErr != nil {
			return pii.PseudonymContext{}, unavailableSecretError()
		}
		if stored.legacySecret == nil {
			return stored.context, nil
		}
	case !errors.Is(err, ErrSecretNotFound):
		return pii.PseudonymContext{}, unavailableSecretError()
	}

	if r.Lock == nil {
		return pii.PseudonymContext{}, unavailableSecretError()
	}

	var resolved pii.PseudonymContext
	err = r.Lock.WithLock(ctx, func() error {
		// Another process may have initialized or migrated the record while this
		// caller waited. Always make the decision from a fresh load under lock.
		raw, loadErr := r.Store.Load(ctx)
		var secretBytes []byte
		switch {
		case loadErr == nil:
			stored, decodeErr := decodeStoredPseudonym(raw)
			if decodeErr != nil {
				return unavailableSecretError()
			}
			if stored.legacySecret == nil {
				resolved = stored.context
				return nil
			}
			secretBytes = stored.legacySecret
		case errors.Is(loadErr, ErrSecretNotFound):
			secretBytes = make([]byte, GeneratedSecretBytes)
			if _, randomErr := io.ReadFull(r.random(), secretBytes); randomErr != nil {
				return unavailableSecretError()
			}
		default:
			return unavailableSecretError()
		}

		keyID, keyErr := generateKeyID(r.random())
		if keyErr != nil {
			return unavailableSecretError()
		}
		record, encodeErr := encodeStoredPseudonym(secretBytes, keyID)
		if encodeErr != nil {
			return unavailableSecretError()
		}
		if saveErr := r.Store.Save(ctx, record); saveErr != nil {
			return unavailableSecretError()
		}

		// Use the persisted value, not the local candidate. This verifies that
		// the backing store accepted the complete versioned record.
		persisted, loadErr := r.Store.Load(ctx)
		if loadErr != nil {
			return unavailableSecretError()
		}
		stored, decodeErr := decodeStoredPseudonym(persisted)
		if decodeErr != nil || stored.legacySecret != nil {
			return unavailableSecretError()
		}
		resolved = stored.context
		return nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return pii.PseudonymContext{}, ctxErr
		}
		return pii.PseudonymContext{}, unavailableSecretError()
	}
	return resolved, nil
}

func (r *SecretResolver) random() io.Reader {
	if r.Random != nil {
		return r.Random
	}
	return rand.Reader
}

func generateKeyID(random io.Reader) (string, error) {
	raw := make([]byte, GeneratedKeyIDBytes)
	if _, err := io.ReadFull(random, raw); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return generatedKeyIDPrefix + strings.ToLower(encoded), nil
}

func encodeStoredPseudonym(secretBytes []byte, keyID string) ([]byte, error) {
	if len(secretBytes) != GeneratedSecretBytes {
		return nil, unavailableSecretError()
	}
	secret, err := pii.NewSecret(secretBytes)
	if err != nil {
		return nil, err
	}
	pseudonym, err := pii.NewPseudonymContext(secret, keyID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(storedPseudonymRecord{
		Schema:         StoredPseudonymSchema,
		IdentitySchema: pseudonym.Schema(),
		KeyID:          pseudonym.KeyID(),
		Secret:         base64.RawStdEncoding.EncodeToString(secretBytes),
	})
}

func decodeStoredPseudonym(raw []byte) (decodedStoredPseudonym, error) {
	// Releases before the versioned record stored exactly 32 raw secret bytes.
	// Preserve those bytes and assign independent rotation metadata under lock.
	if len(raw) == GeneratedSecretBytes {
		return decodedStoredPseudonym{legacySecret: append([]byte(nil), raw...)}, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record storedPseudonymRecord
	if err := decoder.Decode(&record); err != nil {
		return decodedStoredPseudonym{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return decodedStoredPseudonym{}, fmt.Errorf("multiple PII key records")
		}
		return decodedStoredPseudonym{}, err
	}
	if record.Schema != StoredPseudonymSchema || record.IdentitySchema != pii.IdentitySchemaV2 {
		return decodedStoredPseudonym{}, fmt.Errorf("unsupported PII key record")
	}
	secretBytes, err := base64.RawStdEncoding.DecodeString(record.Secret)
	if err != nil || len(secretBytes) != GeneratedSecretBytes {
		return decodedStoredPseudonym{}, fmt.Errorf("invalid PII key record secret")
	}
	secret, err := pii.NewSecret(secretBytes)
	if err != nil {
		return decodedStoredPseudonym{}, err
	}
	pseudonym, err := pii.NewPseudonymContext(secret, record.KeyID)
	if err != nil {
		return decodedStoredPseudonym{}, err
	}
	return decodedStoredPseudonym{context: pseudonym}, nil
}

func unavailableSecretError() error {
	return fmt.Errorf("PII redaction key unavailable; set %s and %s to non-blank private-secret and public-ID values", EnvSecret, EnvKeyID)
}
