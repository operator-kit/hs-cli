package pii

import (
	"bytes"
	"encoding/json"
	"io"
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
	require.Equal(t, firstA, firstB,
		"fixture must retain the original first-name collision")
	require.Equal(t, strings.SplitN(lastA, " [", 2)[0], strings.SplitN(lastB, " [", 2)[0],
		"fixture must retain the original last-name collision")
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
	firstEngine, err := NewEngine(ModeAll,
		mustTestPseudonym("medium-rotation-secret-a", "rotation-a"))
	require.NoError(t, err)
	secondEngine, err := NewEngine(ModeAll,
		mustTestPseudonym("medium-rotation-secret-b", "rotation-b"))
	require.NoError(t, err)

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
	_, firstLast, _ := firstEngine.RedactPerson("Alice", "Fixture", "alice@example.test")
	_, secondLast, _ := secondEngine.RedactPerson("Alice", "Fixture", "alice@example.test")
	assert.Contains(t, firstLast, "[rotation-a-")
	assert.Contains(t, secondLast, "[rotation-b-")
}

type multilingualPrivacyCorpus struct {
	Schema int                      `json:"schema"`
	Cases  []multilingualCorpusCase `json:"cases"`
}

type multilingualCorpusCase struct {
	ID           string          `json:"id"`
	Language     string          `json:"language"`
	Script       string          `json:"script"`
	Category     string          `json:"category"`
	Expected     string          `json:"expected"`
	Mode         string          `json:"mode,omitempty"`
	Text         string          `json:"text"`
	RepeatPrefix string          `json:"repeat_prefix,omitempty"`
	Repeat       int             `json:"repeat,omitempty"`
	DetectNames  []string        `json:"detect_names,omitempty"`
	Known        []KnownIdentity `json:"known,omitempty"`
	Absent       []string        `json:"absent,omitempty"`
	Present      []string        `json:"present,omitempty"`
}

type corpusNameDetector struct {
	names []string
}

func (d corpusNameDetector) DetectNames(text string) ([]NameSpan, error) {
	spans := make([]NameSpan, 0, len(d.names))
	for _, name := range d.names {
		start := strings.Index(text, name)
		if start < 0 {
			continue
		}
		spans = append(spans, NameSpan{Text: name, Start: start, End: start + len(name), Score: 1})
	}
	return spans, nil
}

func TestPIIRegression_Medium14_MultilingualPrivacyCorpusIsMaintained(t *testing.T) {
	corpusPath := filepath.Join("testdata", "multilingual_privacy_corpus.json")
	raw, err := os.ReadFile(corpusPath)
	require.NoError(t, err,
		"a synthetic multilingual corpus is required to measure privacy recall and false positives")

	var corpus multilingualPrivacyCorpus
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&corpus))
	var extra any
	require.ErrorIs(t, decoder.Decode(&extra), io.EOF)
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
		require.Contains(t, requiredScripts, fixture.Script, "corpus case %q uses an unsupported script", fixture.ID)
		require.Contains(t, requiredOutcomes, fixture.Expected, "corpus case %q uses an unsupported outcome", fixture.ID)
		require.Contains(t, requiredCategories, fixture.Category, "corpus case %q uses an unsupported category", fixture.ID)
		require.GreaterOrEqual(t, fixture.Repeat, 0, "corpus case %q has a negative repeat count", fixture.ID)
		require.LessOrEqual(t, fixture.Repeat, 1000, "corpus case %q has an unreasonable repeat count", fixture.ID)
		if fixture.Expected == "redact" {
			require.NotEmpty(t, fixture.Absent, "redact corpus case %q has no absence assertion", fixture.ID)
		} else {
			require.NotEmpty(t, fixture.Present, "preserve corpus case %q has no preservation assertion", fixture.ID)
		}
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

		rawMode := fixture.Mode
		if rawMode == "" {
			rawMode = ModeAll.String()
		}
		mode, modeErr := ParseMode(rawMode)
		require.NoError(t, modeErr, "corpus case %q has invalid mode %q", fixture.ID, rawMode)
		text := strings.Repeat(fixture.RepeatPrefix, fixture.Repeat) + fixture.Text
		engine := mustTestEngine(mode, "multilingual-corpus-secret",
			WithNER(corpusNameDetector{names: fixture.DetectNames}))
		output := engine.RedactText(text, fixture.Known)
		for _, rawPII := range fixture.Absent {
			assert.NotContains(t, output, rawPII, "corpus case %q leaked expected redaction", fixture.ID)
		}
		for _, preserved := range fixture.Present {
			assert.Contains(t, output, preserved, "corpus case %q over-redacted expected content", fixture.ID)
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
