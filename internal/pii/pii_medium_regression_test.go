package pii

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPIIRegression_Medium11_DisplayNamesDisambiguateDistinctPeople(t *testing.T) {
	engine := mustTestEngine(ModeAll, "medium-regression-secret")

	firstA, lastA, emailA := engine.RedactPerson(
		"Alice", "Fixture", "medium-collision-0008@example.test",
	)
	firstB, lastB, emailB := engine.RedactPerson(
		"Bob", "Fixture", "medium-collision-0102@example.test",
	)

	require.NotEqual(t, emailA, emailB,
		"fixture must represent distinct pseudonymous identities")
	assert.NotEqual(t, firstA+" "+lastA, firstB+" "+lastB,
		"table-safe display names must include deterministic disambiguation")
}

func TestPIIRegression_Medium12_IdentityKeysNormalizeUnicodeComposition(t *testing.T) {
	composed := personKey("Jos\u00e9", "Alvarez", "")
	decomposed := personKey("Jose\u0301", "Alvarez", "")

	require.NotEqual(t, "", composed)
	require.NotEqual(t, "", decomposed)
	assert.Equal(t, composed, decomposed,
		"visually identical NFC and NFD names must resolve to one identity key")
}

// pseudonymKeyIdentifier is the minimum rotation boundary: callers that emit
// pseudonyms need an opaque, non-secret identifier for the active key. The
// eventual representation and migration policy remain design decisions.
type pseudonymKeyIdentifier interface {
	PseudonymKeyID() string
}

func TestPIIRegression_Medium13_SecretRotationHasAnExplicitKeyIdentifier(t *testing.T) {
	firstEngine := mustTestEngine(ModeAll, "medium-rotation-secret-a")
	secondEngine := mustTestEngine(ModeAll, "medium-rotation-secret-b")

	_, _, firstEmail := firstEngine.RedactPerson("Alice", "Fixture", "alice@example.test")
	_, _, secondEmail := secondEngine.RedactPerson("Alice", "Fixture", "alice@example.test")
	require.NotEqual(t, firstEmail, secondEmail,
		"fixture must demonstrate that rotating the secret changes display identity")

	firstVersioned, firstOK := any(firstEngine).(pseudonymKeyIdentifier)
	secondVersioned, secondOK := any(secondEngine).(pseudonymKeyIdentifier)
	require.True(t, firstOK && secondOK,
		"the pseudonym boundary must expose an opaque key identifier for rotation")

	firstKeyID := firstVersioned.PseudonymKeyID()
	secondKeyID := secondVersioned.PseudonymKeyID()
	assert.NotEmpty(t, firstKeyID)
	assert.NotEmpty(t, secondKeyID)
	assert.NotEqual(t, firstKeyID, secondKeyID,
		"different rotation keys must have distinguishable identifiers")
	assert.NotContains(t, firstKeyID, "medium-rotation-secret-a",
		"a key identifier must never disclose key material")
}

type multilingualPrivacyCorpus struct {
	Schema int                      `json:"schema"`
	Cases  []multilingualCorpusCase `json:"cases"`
}

type multilingualCorpusCase struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Script   string `json:"script"`
	Category string `json:"category"`
	Expected string `json:"expected"`
	Text     string `json:"text"`
}

func TestPIIRegression_Medium14_MultilingualPrivacyCorpusIsMaintained(t *testing.T) {
	corpusPath := filepath.Join("testdata", "multilingual_privacy_corpus.json")
	raw, err := os.ReadFile(corpusPath)
	require.NoError(t, err,
		"a synthetic multilingual corpus is required to measure privacy recall and false positives")

	var corpus multilingualPrivacyCorpus
	require.NoError(t, json.Unmarshal(raw, &corpus))
	assert.Equal(t, 1, corpus.Schema)
	assert.NotEmpty(t, corpus.Cases)

	requiredScripts := map[string]bool{"Latin": false, "Arabic": false, "Han": false}
	requiredOutcomes := map[string]bool{"redact": false, "preserve": false}
	requiredCategories := map[string]bool{
		"person":         false,
		"address":        false,
		"identifier":     false,
		"customer_staff": false,
		"long_content":   false,
	}

	seenIDs := map[string]struct{}{}
	for _, fixture := range corpus.Cases {
		assert.NotEmpty(t, fixture.ID)
		assert.NotEmpty(t, fixture.Language)
		assert.NotEmpty(t, fixture.Text)
		if _, exists := seenIDs[fixture.ID]; fixture.ID != "" {
			assert.False(t, exists, "duplicate corpus case %q", fixture.ID)
			seenIDs[fixture.ID] = struct{}{}
		}
		if _, ok := requiredScripts[fixture.Script]; ok {
			requiredScripts[fixture.Script] = true
		}
		if _, ok := requiredOutcomes[fixture.Expected]; ok {
			requiredOutcomes[fixture.Expected] = true
		}
		if _, ok := requiredCategories[fixture.Category]; ok {
			requiredCategories[fixture.Category] = true
		}
	}

	for script, covered := range requiredScripts {
		assert.True(t, covered, "corpus must cover the %s script", script)
	}
	for outcome, covered := range requiredOutcomes {
		assert.True(t, covered, "corpus must include %s expectations", outcome)
	}
	for category, covered := range requiredCategories {
		assert.True(t, covered, "corpus must cover %s behavior", strings.ReplaceAll(category, "_", "/"))
	}
}
