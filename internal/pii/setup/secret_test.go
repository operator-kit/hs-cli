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

func environment(secret, keyID string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		switch name {
		case EnvSecret:
			return secret, secret != ""
		case EnvKeyID:
			return keyID, keyID != ""
		default:
			return "", false
		}
	}
}

func TestSecretResolverEnvironmentOverridesStoreAndPreservesBytes(t *testing.T) {
	store := &memorySecretStore{loadErr: errors.New("store must not be called")}
	resolver := &SecretResolver{
		Store:     store,
		LookupEnv: environment("  stable-secret  ", " Release-2026 "),
	}

	got, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	want, err := pii.NewSecretString("  stable-secret  ")
	if err != nil {
		t.Fatal(err)
	}
	if got.Secret() != want {
		t.Fatal("resolver changed explicit environment-secret bytes")
	}
	if got.KeyID() != "release-2026" || got.Schema() != pii.IdentitySchemaV2 {
		t.Fatalf("resolved metadata = %q/%q", got.KeyID(), got.Schema())
	}
	if store.loadCalls != 0 || store.saveCalls != 0 {
		t.Fatalf("environment override touched store: loads=%d saves=%d", store.loadCalls, store.saveCalls)
	}
}

func TestSecretResolverEnvironmentRequiresSecretAndPublicKeyIDTogether(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		secret string
		keyID  string
	}{
		{name: "missing key ID", secret: "private-secret"},
		{name: "missing secret", keyID: "release-2026"},
		{name: "invalid key ID", secret: "private-secret", keyID: "contains spaces"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			resolver := &SecretResolver{LookupEnv: environment(fixture.secret, fixture.keyID)}
			_, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
			if err == nil || !strings.Contains(err.Error(), EnvSecret) || !strings.Contains(err.Error(), EnvKeyID) {
				t.Fatalf("ResolveContext error = %v", err)
			}
		})
	}
}

func TestSecretResolverRejectsExplicitBlankEnvironmentValues(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		values map[string]string
	}{
		{name: "blank secret", values: map[string]string{EnvSecret: "", EnvKeyID: "release-2026"}},
		{name: "blank key ID", values: map[string]string{EnvSecret: "private-secret", EnvKeyID: "  "}},
		{name: "blank secret only", values: map[string]string{EnvSecret: ""}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			store := &memorySecretStore{loadErr: errors.New("store must not be called")}
			resolver := &SecretResolver{
				Store: store,
				LookupEnv: func(name string) (string, bool) {
					value, ok := fixture.values[name]
					return value, ok
				},
			}
			_, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
			if err == nil || !strings.Contains(err.Error(), EnvSecret) || !strings.Contains(err.Error(), EnvKeyID) {
				t.Fatalf("ResolveContext error = %v", err)
			}
			if store.loadCalls != 0 {
				t.Fatalf("invalid explicit environment fell back to store: loads=%d", store.loadCalls)
			}
		})
	}
}

func TestSecretResolverUsesVersionedStoredContext(t *testing.T) {
	storedSecret := fixedSecretBytes(0x21)
	record, err := encodeStoredPseudonym(storedSecret, "stored-a")
	if err != nil {
		t.Fatal(err)
	}
	store := &memorySecretStore{value: record}
	resolver := &SecretResolver{
		Store:     store,
		LookupEnv: environment("", ""),
	}

	got, err := resolver.ResolveContext(context.Background(), pii.ModeCustomers)
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	want, err := pii.NewSecret(storedSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got.Secret() != want || got.KeyID() != "stored-a" {
		t.Fatalf("resolver returned wrong stored context: key ID %q", got.KeyID())
	}
	if store.saveCalls != 0 {
		t.Fatalf("versioned record was unexpectedly rewritten %d times", store.saveCalls)
	}
}

func TestSecretResolverMigratesLegacyRawSecretUnderLock(t *testing.T) {
	legacySecret := fixedSecretBytes(0x31)
	keyRandom := bytes.Repeat([]byte{0x52}, GeneratedKeyIDBytes)
	wantKeyID, err := generateKeyID(bytes.NewReader(keyRandom))
	if err != nil {
		t.Fatal(err)
	}
	store := &memorySecretStore{value: append([]byte(nil), legacySecret...)}
	lock := &memoryInitializationLock{}
	resolver := &SecretResolver{
		Store:     store,
		Lock:      lock,
		Random:    bytes.NewReader(keyRandom),
		LookupEnv: environment("", ""),
	}

	first, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("first ResolveContext returned error: %v", err)
	}
	second, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("second ResolveContext returned error: %v", err)
	}
	wantSecret, err := pii.NewSecret(legacySecret)
	if err != nil {
		t.Fatal(err)
	}
	if first.Secret() != wantSecret || second.Secret() != wantSecret {
		t.Fatal("legacy migration changed private key material")
	}
	if first.KeyID() != wantKeyID || second.KeyID() != wantKeyID {
		t.Fatalf("migrated key IDs = %q/%q, want %q", first.KeyID(), second.KeyID(), wantKeyID)
	}
	if bytes.Equal(store.value, legacySecret) {
		t.Fatal("legacy record was not migrated")
	}
	if store.saveCalls != 1 || lock.calls != 1 {
		t.Fatalf("migration saves/locks = %d/%d, want 1/1", store.saveCalls, lock.calls)
	}
}

func TestSecretResolverGeneratesStoresAndReusesContext(t *testing.T) {
	store := &memorySecretStore{}
	lock := &memoryInitializationLock{}
	secretRandom := fixedSecretBytes(0x42)
	keyRandom := bytes.Repeat([]byte{0x24}, GeneratedKeyIDBytes)
	random := append(append([]byte(nil), secretRandom...), keyRandom...)
	resolver := &SecretResolver{
		Store:     store,
		Lock:      lock,
		Random:    bytes.NewReader(random),
		LookupEnv: environment("", ""),
	}

	first, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("first ResolveContext returned error: %v", err)
	}
	second, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
	if err != nil {
		t.Fatalf("second ResolveContext returned error: %v", err)
	}
	if first != second {
		t.Fatal("stored pseudonym context was not reused")
	}
	wantSecret, err := pii.NewSecret(secretRandom)
	if err != nil {
		t.Fatal(err)
	}
	wantKeyID, err := generateKeyID(bytes.NewReader(keyRandom))
	if err != nil {
		t.Fatal(err)
	}
	if first.Secret() != wantSecret || first.KeyID() != wantKeyID {
		t.Fatalf("generated context key ID = %q, want %q", first.KeyID(), wantKeyID)
	}
	stored, err := decodeStoredPseudonym(store.value)
	if err != nil || stored.legacySecret != nil || stored.context != first {
		t.Fatalf("stored versioned record did not round-trip: %v", err)
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

	got, err := resolver.ResolveContext(context.Background(), pii.ModeOff)
	if err != nil {
		t.Fatalf("ResolveContext returned error: %v", err)
	}
	if !got.IsZero() {
		t.Fatal("off mode returned a pseudonym context")
	}
}

func TestSecretResolverStoreFailureIsSafeAndActionable(t *testing.T) {
	store := &memorySecretStore{loadErr: errors.New("dbus path /private/user leaked")}
	resolver := &SecretResolver{
		Store:     store,
		LookupEnv: environment("", ""),
	}

	_, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
	if err == nil {
		t.Fatal("ResolveContext returned no error")
	}
	if !strings.Contains(err.Error(), EnvSecret) || !strings.Contains(err.Error(), EnvKeyID) {
		t.Fatalf("error does not explain environment recovery: %v", err)
	}
	if strings.Contains(err.Error(), "dbus") || strings.Contains(err.Error(), "/private/user") {
		t.Fatalf("error leaked backend details: %v", err)
	}
}

func TestSecretResolverRejectsInvalidStoredRecord(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte("short"),
		[]byte(`{"schema":1,"identity_schema":"v2","key_id":"bad id","secret":"AA"}`),
		[]byte(`{"schema":2,"identity_schema":"v2","key_id":"key-a","secret":"AA"}`),
	} {
		store := &memorySecretStore{value: raw}
		resolver := &SecretResolver{Store: store, LookupEnv: environment("", "")}
		if _, err := resolver.ResolveContext(context.Background(), pii.ModeAll); err == nil {
			t.Fatalf("ResolveContext accepted invalid stored record %q", raw)
		}
		if store.saveCalls != 0 {
			t.Fatal("invalid record was overwritten")
		}
	}
}

func TestSecretResolverConcurrentInitializersConverge(t *testing.T) {
	const workers = 12
	store := &memorySecretStore{}
	lock := &memoryInitializationLock{}
	resolver := &SecretResolver{
		Store:     store,
		Lock:      lock,
		Random:    bytes.NewReader(bytes.Repeat([]byte{0x7c}, (GeneratedSecretBytes+GeneratedKeyIDBytes)*workers)),
		LookupEnv: environment("", ""),
	}

	results := make(chan pii.PseudonymContext, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pseudonym, err := resolver.ResolveContext(context.Background(), pii.ModeAll)
			results <- pseudonym
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("ResolveContext returned error: %v", err)
		}
	}
	var first pii.PseudonymContext
	for pseudonym := range results {
		if first.IsZero() {
			first = pseudonym
			continue
		}
		if pseudonym != first {
			t.Fatal("concurrent initializers returned different contexts")
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
