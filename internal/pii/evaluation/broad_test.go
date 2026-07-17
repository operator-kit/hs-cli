package evaluation

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func broadCorpusDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "testdata", "privacy-filter", "broad", "v1")
}

func TestBroadQualityCorpusHasStatisticallyMeaningfulLockedDenominators(t *testing.T) {
	corpus, err := LoadBroadCorpusDir(broadCorpusDir(t))
	if err != nil {
		t.Fatalf("load locked broad quality corpus: %v", err)
	}
	if len(corpus.Cases) != 2724 {
		t.Fatalf("broad corpus case count changed: got %d want 2724", len(corpus.Cases))
	}
	if len(corpus.Fingerprint) != sha256HexLength {
		t.Fatalf("broad corpus lacks a stable SHA-256 identity: %q", corpus.Fingerprint)
	}
	minimums := corpus.Manifest.MinimumDenominators
	for name, count := range map[string]int{
		"secret":       corpus.Coverage.SecretRedactCases,
		"account":      corpus.Coverage.AccountRedactCases,
		"private-date": corpus.Coverage.PrivateDateRedactCases,
		"preservation": corpus.Coverage.PreservationCases,
	} {
		if count < 100 || 10000/count > corpus.Manifest.MaximumCaseBasisPoints {
			t.Fatalf("%s denominator %d lets one case move more than one percentage point", name, count)
		}
	}
	for _, language := range RequiredLanguages {
		if corpus.Coverage.LanguageRedactCases[language] < minimums.LanguageRedactCases ||
			corpus.Coverage.LanguagePreserveCases[language] < minimums.LanguagePreserveCases {
			t.Fatalf("language %q broad denominators are incomplete: redact=%d preserve=%d", language,
				corpus.Coverage.LanguageRedactCases[language], corpus.Coverage.LanguagePreserveCases[language])
		}
	}
}

func TestSyntheticSecretsMatchOutOfBandProvenanceWithoutMarkerDrivenValues(t *testing.T) {
	corpora := make([][]Case, 0, 2)
	smoke, err := LoadCorpusDir(corpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	corpora = append(corpora, smoke.Cases)
	broad, err := LoadBroadCorpusDir(broadCorpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	corpora = append(corpora, broad.Cases)
	for _, cases := range corpora {
		for _, fixture := range cases {
			for _, target := range fixture.Targets {
				if target.Kind != SpanSecret {
					continue
				}
				if target.Synthetic == nil {
					t.Fatalf("case %q secret target %q lacks provenance", fixture.ID, target.ID)
				}
				generated, generationErr := GenerateSyntheticValue(*target.Synthetic)
				if generationErr != nil || generated != target.Value {
					t.Fatalf("case %q secret target %q provenance mismatch: %v", fixture.ID, target.ID, generationErr)
				}
				if target.Synthetic.Purpose == "must-detect" {
					lower := strings.ToLower(target.Value)
					for _, marker := range []string{"example", "not-a-real", "not_a_real", "invalid", "placeholder", "synthetic"} {
						if strings.Contains(lower, marker) {
							t.Fatalf("case %q evaluated credential contains marker %q", fixture.ID, marker)
						}
					}
				}
			}
		}
	}
}

func TestPrivacyFilterCorpusGeneratorIsByteReproducible(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	tempRoot := t.TempDir()
	smokeRelative := filepath.Join("internal", "pii", "testdata", "privacy-filter", "v1")
	tempSmoke := filepath.Join(tempRoot, smokeRelative)
	if err := os.MkdirAll(tempSmoke, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"schema.json", "command-output-payloads.json", "preservation.json"} {
		raw, readErr := os.ReadFile(filepath.Join(repoRoot, smokeRelative, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(tempSmoke, name), raw, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := GeneratePrivacyFilterTestdata(tempRoot); err != nil {
		t.Fatalf("regenerate privacy corpus: %v", err)
	}
	generated := []string{
		filepath.Join(smokeRelative, "secrets.json"),
		filepath.Join(smokeRelative, "command-output-payloads.json"),
		filepath.Join(smokeRelative, "preservation.json"),
	}
	broadFiles := append(append([]string(nil), BroadCorpusPartitions...), "manifest")
	for _, name := range broadFiles {
		generated = append(generated, filepath.Join("internal", "pii", "testdata", "privacy-filter", "broad", "v1", name+".json"))
	}
	for _, relative := range generated {
		want, readErr := os.ReadFile(filepath.Join(repoRoot, relative))
		if readErr != nil {
			t.Fatal(readErr)
		}
		got, readErr := os.ReadFile(filepath.Join(tempRoot, relative))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("generated corpus file %s is stale; run %s", relative,
				fmt.Sprintf("go run ./internal/pii/evaluation/cmd/corpusgen -repo-root ."))
		}
	}
}

func TestCredentialShapedFixtureMaterialIsIndependentlyScannedForProvenance(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`CLDX[A-Z0-9]{16}`),
		regexp.MustCompile(`scm_[A-Za-z0-9]{36}`),
		regexp.MustCompile(`paylive_[A-Za-z0-9]{32}`),
		regexp.MustCompile(`whsig_[A-Za-z0-9]{40}`),
		regexp.MustCompile(`oat2_[A-Za-z0-9_-]{52}`),
		regexp.MustCompile(`access_token_[A-Za-z0-9]{40}`),
		regexp.MustCompile(`obsap_[a-f0-9]{32}`),
		regexp.MustCompile(`-----BEGIN SECRET-KEYX-----`),
	}
	smoke, err := LoadCorpusDir(corpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	broad, err := LoadBroadCorpusDir(broadCorpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range append(append([]Case(nil), smoke.Cases...), broad.Cases...) {
		for _, pattern := range patterns {
			for _, location := range pattern.FindAllStringIndex(fixture.Text, -1) {
				provenanced := false
				for _, target := range fixture.Targets {
					if target.Kind == SpanSecret && target.Synthetic != nil && target.Start <= location[0] && target.End >= location[1] {
						provenanced = true
						break
					}
				}
				if !provenanced {
					t.Fatalf("case %q contains unprovenanced credential shape %q", fixture.ID, pattern.String())
				}
			}
		}
	}
}

func TestBroadManifestRejectsDenominatorWeakening(t *testing.T) {
	corpus, err := LoadBroadCorpusDir(broadCorpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	weakened := corpus.Manifest
	weakened.MinimumDenominators.SecretRedactCases = 99
	if err := validateBroadManifest(&weakened); err == nil {
		t.Fatal("broad corpus accepted a weakened secret denominator")
	}
	weakened = corpus.Manifest
	weakened.MaximumCaseBasisPoints = 101
	if err := validateBroadManifest(&weakened); err == nil {
		t.Fatal("broad corpus accepted more than one percentage point per case")
	}
}
