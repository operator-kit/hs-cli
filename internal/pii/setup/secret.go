package setup

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/operator-kit/hs-cli/internal/pii"
)

const (
	EnvSecret            = "HS_INBOX_PII_SECRET"
	GeneratedSecretBytes = 32
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
// material into the opaque secret required by an enabled PII engine.
type SecretResolver struct {
	Store     SecretStore
	Lock      InitializationLock
	Random    io.Reader
	LookupEnv func(string) (string, bool)
}

func (r *SecretResolver) Resolve(ctx context.Context, mode pii.Mode) (pii.Secret, error) {
	if !mode.Valid() {
		return pii.Secret{}, fmt.Errorf("invalid PII mode")
	}
	if !pii.IsEnabled(mode) {
		return pii.Secret{}, nil
	}

	lookup := r.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if raw, ok := lookup(EnvSecret); ok && strings.TrimSpace(raw) != "" {
		secret, err := pii.NewSecretString(raw)
		if err != nil {
			return pii.Secret{}, unavailableSecretError()
		}
		return secret, nil
	}

	if r.Store == nil {
		return pii.Secret{}, unavailableSecretError()
	}
	raw, err := r.Store.Load(ctx)
	switch {
	case err == nil:
		return storedSecret(raw)
	case !errors.Is(err, ErrSecretNotFound):
		return pii.Secret{}, unavailableSecretError()
	}

	if r.Lock == nil {
		return pii.Secret{}, unavailableSecretError()
	}

	var resolved pii.Secret
	err = r.Lock.WithLock(ctx, func() error {
		// Another process may have initialized the key while this caller waited.
		raw, loadErr := r.Store.Load(ctx)
		switch {
		case loadErr == nil:
			var secretErr error
			resolved, secretErr = storedSecret(raw)
			return secretErr
		case !errors.Is(loadErr, ErrSecretNotFound):
			return unavailableSecretError()
		}

		random := r.Random
		if random == nil {
			random = rand.Reader
		}
		generated := make([]byte, GeneratedSecretBytes)
		if _, randomErr := io.ReadFull(random, generated); randomErr != nil {
			return unavailableSecretError()
		}
		if saveErr := r.Store.Save(ctx, generated); saveErr != nil {
			return unavailableSecretError()
		}

		// Use the persisted value, not the local candidate. This verifies that
		// the backing store accepted the write exactly.
		persisted, loadErr := r.Store.Load(ctx)
		if loadErr != nil {
			return unavailableSecretError()
		}
		var secretErr error
		resolved, secretErr = storedSecret(persisted)
		return secretErr
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return pii.Secret{}, ctxErr
		}
		return pii.Secret{}, unavailableSecretError()
	}
	return resolved, nil
}

func storedSecret(raw []byte) (pii.Secret, error) {
	if len(raw) != GeneratedSecretBytes {
		return pii.Secret{}, unavailableSecretError()
	}
	secret, err := pii.NewSecret(raw)
	if err != nil {
		return pii.Secret{}, unavailableSecretError()
	}
	return secret, nil
}

func unavailableSecretError() error {
	return fmt.Errorf("PII redaction secret unavailable; set %s to a non-blank private value", EnvSecret)
}
