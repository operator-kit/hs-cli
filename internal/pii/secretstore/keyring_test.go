package secretstore

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/operator-kit/hs-cli/internal/pii/setup"
	"github.com/zalando/go-keyring"
)

type fakeKeyringBackend struct {
	value  string
	getErr error
	setErr error
}

func (b *fakeKeyringBackend) Get(string, string) (string, error) {
	return b.value, b.getErr
}

func (b *fakeKeyringBackend) Set(_, _, value string) error {
	b.value = value
	return b.setErr
}

func TestKeyringStoreRoundTrip(t *testing.T) {
	backend := &fakeKeyringBackend{}
	store := newKeyringStore(backend)
	raw := []byte("01234567890123456789012345678901")

	if err := store.Save(context.Background(), raw); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if backend.value != base64.RawURLEncoding.EncodeToString(raw) {
		t.Fatal("keyring value did not use the fixed storage encoding")
	}
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatal("keyring round trip changed secret bytes")
	}
}

func TestKeyringStoreMapsNotFound(t *testing.T) {
	store := newKeyringStore(&fakeKeyringBackend{getErr: keyring.ErrNotFound})
	_, err := store.Load(context.Background())
	if !errors.Is(err, setup.ErrSecretNotFound) {
		t.Fatalf("Load error = %v, want ErrSecretNotFound", err)
	}
}

func TestKeyringStoreRejectsCorruptValueWithoutEchoingIt(t *testing.T) {
	const corrupt = "private-corrupt-value!"
	store := newKeyringStore(&fakeKeyringBackend{value: corrupt})
	_, err := store.Load(context.Background())
	if err == nil {
		t.Fatal("Load accepted corrupt keyring data")
	}
	if contains := errorContains(err, corrupt); contains {
		t.Fatalf("Load error exposed keyring value: %v", err)
	}
}

func errorContains(err error, value string) bool {
	return err != nil && len(value) > 0 && strings.Contains(err.Error(), value)
}
