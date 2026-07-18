package secretstore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/operator-kit/hs-cli/internal/pii/setup"
	"github.com/zalando/go-keyring"
)

const (
	keyringService = "hs-cli"
	keyringRecord  = "pii_identity_secret_v1"
)

type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
}

type packageKeyringBackend struct{}

func (packageKeyringBackend) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (packageKeyringBackend) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

type KeyringStore struct {
	backend keyringBackend
}

func NewKeyringStore() *KeyringStore {
	return newKeyringStore(packageKeyringBackend{})
}

func newKeyringStore(backend keyringBackend) *KeyringStore {
	return &KeyringStore{backend: backend}
}

func (s *KeyringStore) Load(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	encoded, err := s.backend.Get(keyringService, keyringRecord)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, setup.ErrSecretNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading PII secret from OS keyring")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding PII secret from OS keyring")
	}
	return raw, nil
}

func (s *KeyringStore) Save(ctx context.Context, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.backend.Set(keyringService, keyringRecord, encoded); err != nil {
		return fmt.Errorf("saving PII secret to OS keyring")
	}
	return nil
}
