package setup

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/operator-kit/hs-cli/internal/pii"
)

type memorySecretStore struct {
	mu        sync.Mutex
	value     []byte
	loadErr   error
	saveErr   error
	loadCalls int
	saveCalls int
}

func (s *memorySecretStore) Load(context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadCalls++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	if s.value == nil {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), s.value...), nil
}

func (s *memorySecretStore) Save(_ context.Context, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveCalls++
	if s.saveErr != nil {
		return s.saveErr
	}
	s.value = append([]byte(nil), value...)
	return nil
}

type memoryInitializationLock struct {
	mu    sync.Mutex
	calls int
}

func (l *memoryInitializationLock) WithLock(_ context.Context, fn func() error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	return fn()
}

func fixedSecretBytes(value byte) []byte {
	return bytes.Repeat([]byte{value}, GeneratedSecretBytes)
}

func TestSecretResolverEnvironmentOverridesStoreAndPreservesBytes(t *testing.T) {
	store := &memorySecretStore{loadErr: errors.New("store must not be called")}
	resolver := &SecretResolver{
		Store: store,
		LookupEnv: func(string) (string, bool) {
			return "  stable-secret  ", true
		},
	}

	got, err := resolver.Resolve(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	want, err := pii.NewSecretString("  stable-secret  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("resolver changed explicit environment-secret bytes")
	}
	if store.loadCalls != 0 || store.saveCalls != 0 {
		t.Fatalf("environment override touched store: loads=%d saves=%d", store.loadCalls, store.saveCalls)
	}
}

func TestSecretResolverBlankEnvironmentUsesStoredSecret(t *testing.T) {
	stored := fixedSecretBytes(0x21)
	store := &memorySecretStore{value: stored}
	resolver := &SecretResolver{
		Store: store,
		LookupEnv: func(string) (string, bool) {
			return "  \t ", true
		},
	}

	got, err := resolver.Resolve(context.Background(), pii.ModeCustomers)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	want, err := pii.NewSecret(stored)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("resolver did not use stored secret")
	}
}

func TestSecretResolverGeneratesStoresAndReusesSecret(t *testing.T) {
	store := &memorySecretStore{}
	lock := &memoryInitializationLock{}
	random := fixedSecretBytes(0x42)
	resolver := &SecretResolver{
		Store:     store,
		Lock:      lock,
		Random:    bytes.NewReader(random),
		LookupEnv: func(string) (string, bool) { return "", false },
	}

	first, err := resolver.Resolve(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("first Resolve returned error: %v", err)
	}
	second, err := resolver.Resolve(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("second Resolve returned error: %v", err)
	}
	if first != second {
		t.Fatal("stored secret was not reused")
	}
	if !bytes.Equal(store.value, random) {
		t.Fatal("generated secret was not persisted exactly")
	}
	if store.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", store.saveCalls)
	}
}

func TestSecretResolverOffDoesNotTouchDependencies(t *testing.T) {
	resolver := &SecretResolver{
		Store:  &memorySecretStore{loadErr: errors.New("store must not be called")},
		Random: errorReader{err: errors.New("random must not be called")},
		LookupEnv: func(string) (string, bool) {
			panic("environment must not be read")
		},
	}

	got, err := resolver.Resolve(context.Background(), pii.ModeOff)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if !got.IsZero() {
		t.Fatal("off mode returned a secret")
	}
}

func TestSecretResolverStoreFailureIsSafeAndActionable(t *testing.T) {
	store := &memorySecretStore{loadErr: errors.New("dbus path /private/user leaked")}
	resolver := &SecretResolver{
		Store:     store,
		LookupEnv: func(string) (string, bool) { return "", false },
	}

	_, err := resolver.Resolve(context.Background(), pii.ModeAll)
	if err == nil {
		t.Fatal("Resolve returned no error")
	}
	if !strings.Contains(err.Error(), EnvSecret) {
		t.Fatalf("error does not explain environment recovery: %v", err)
	}
	if strings.Contains(err.Error(), "dbus") || strings.Contains(err.Error(), "/private/user") {
		t.Fatalf("error leaked backend details: %v", err)
	}
}

func TestSecretResolverRejectsInvalidStoredSecret(t *testing.T) {
	store := &memorySecretStore{value: []byte("short")}
	resolver := &SecretResolver{
		Store:     store,
		LookupEnv: func(string) (string, bool) { return "", false },
	}

	if _, err := resolver.Resolve(context.Background(), pii.ModeAll); err == nil {
		t.Fatal("Resolve accepted an invalid stored secret")
	}
}

func TestSecretResolverConcurrentInitializersConverge(t *testing.T) {
	const workers = 12
	store := &memorySecretStore{}
	lock := &memoryInitializationLock{}
	resolver := &SecretResolver{
		Store:     store,
		Lock:      lock,
		Random:    bytes.NewReader(bytes.Repeat([]byte{0x7c}, GeneratedSecretBytes*workers)),
		LookupEnv: func(string) (string, bool) { return "", false },
	}

	results := make(chan pii.Secret, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			secret, err := resolver.Resolve(context.Background(), pii.ModeAll)
			results <- secret
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Resolve returned error: %v", err)
		}
	}
	var first pii.Secret
	for secret := range results {
		if first.IsZero() {
			first = secret
			continue
		}
		if secret != first {
			t.Fatal("concurrent initializers returned different secrets")
		}
	}
	if store.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", store.saveCalls)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
