package evaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
	if len(corpus.Cases) != 2892 {
		t.Fatalf("broad corpus case count changed: got %d want 2892", len(corpus.Cases))
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
	forbiddenProviderSignatures := []string{
		"ya29" + ".",
		"whsec" + "_",
		"AK" + "IA",
		"ghp" + "_",
		"rk_" + "live_",
		"ddapi" + "_",
		"-----BEGIN PRIVATE" + " KEY-----",
	}
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
				for _, signature := range forbiddenProviderSignatures {
					if strings.Contains(target.Value, signature) {
						t.Fatalf("case %q evaluated credential uses production-provider signature %q; use the provider-neutral synthetic namespace", fixture.ID, signature)
					}
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
	for _, name := range []string{"schema.json", "command-output-payloads.json", "people-third-parties.json", "preservation.json"} {
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
		filepath.Join(smokeRelative, "people-third-parties.json"),
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

func TestBroadGeneratedFormatsAreSemanticallyStructured(t *testing.T) {
	corpus, err := LoadBroadCorpusDir(broadCorpusDir(t))
	if err != nil {
		t.Fatal(err)
	}
	jsonCases, urlCases := 0, 0
	for _, fixture := range corpus.Cases {
		target := fixture.Targets[0]
		switch fixture.Shape {
		case "json":
			jsonCases++
			var decoded map[string]string
			if !json.Valid([]byte(fixture.Text)) || json.Unmarshal([]byte(fixture.Text), &decoded) != nil || len(decoded) != 1 {
				t.Fatalf("case %q advertises invalid or empty JSON", fixture.ID)
			}
		case "yaml":
			var decoded map[string]any
			if err := yaml.Unmarshal([]byte(fixture.Text), &decoded); err != nil || len(decoded) != 1 {
				t.Fatalf("case %q advertises invalid YAML: %v", fixture.ID, err)
			}
		case "url":
			urlCases++
			rawURL := strings.TrimPrefix(fixture.Text, "endpoint=")
			parsed, parseErr := url.Parse(rawURL)
			if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Query().Get("credential") == "" {
				t.Fatalf("case %q does not contain a credential inside a valid URL: %v", fixture.ID, parseErr)
			}
			if !strings.Contains(parsed.RawQuery, "credential="+target.Value) {
				t.Fatalf("case %q target is outside the URL credential parameter", fixture.ID)
			}
		case "markdown":
			validMarkdownTarget := strings.Contains(fixture.Text, "`"+target.Value+"`")
			if strings.Contains(target.Value, "**") {
				validMarkdownTarget = strings.Count(target.Value, "**") == 2 && strings.Contains(fixture.Text, target.Value)
			}
			if !validMarkdownTarget {
				t.Fatalf("case %q target is not embedded in Markdown", fixture.ID)
			}
		case "html":
			if !strings.HasPrefix(fixture.Text, "<p>") || !strings.Contains(fixture.Text, "<code>"+target.Value+"</code></p>") {
				t.Fatalf("case %q target is not embedded in the HTML element", fixture.ID)
			}
		case "shell":
			if !strings.HasPrefix(fixture.Text, "MESSAGE_") || !strings.HasSuffix(fixture.Text, "'") {
				t.Fatalf("case %q is not a shell assignment", fixture.ID)
			}
		case "log":
			if !strings.Contains(fixture.Text, " credential=\""+target.Value+"\"") {
				t.Fatalf("case %q target is not inside the structured log field", fixture.ID)
			}
		case "command-payload":
			if !strings.Contains(fixture.Text, " --credential=\""+target.Value+"\"") {
				t.Fatalf("case %q target is not inside the command argument", fixture.ID)
			}
		}

		if strings.Contains(target.Value, "<wbr>") && fixture.Shape != "html" {
			t.Fatalf("case %q applies HTML obfuscation outside HTML", fixture.ID)
		}
		if strings.Contains(target.Value, "**") && fixture.Shape != "markdown" {
			t.Fatalf("case %q applies Markdown obfuscation outside Markdown", fixture.ID)
		}
		if fixture.SecretFixture != nil && fixture.SecretFixture.Role == "must-detect" {
			variation := syntheticVariation(*target.Synthetic)
			for _, fragment := range syntheticLeakFragments(target.Value, variation) {
				for _, mode := range []Mode{ModeCustomers, ModeAll} {
					if !contains(fixture.Outputs.For(mode).RequiredAbsent, fragment) {
						t.Fatalf("case %q lacks %s fragment leak sentinel", fixture.ID, mode)
					}
				}
			}
		}
	}
	if jsonCases == 0 || urlCases == 0 {
		t.Fatalf("semantic format validation did not exercise JSON and URL cases")
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
