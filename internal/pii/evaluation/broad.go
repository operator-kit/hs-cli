package evaluation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

const (
	BroadCorpusSchemaVersion = 1
	BroadCorpusVersion       = "broad-v1"
	BroadCorpusGenerator     = "hs-privacy-broad-corpus"
	BroadCorpusGeneratorSeed = "broad-v1"
)

var BroadCorpusPartitions = []string{
	"secrets",
	"account-identifiers",
	"private-public-dates",
	"multilingual",
	"preservation",
}

type BroadCorpusManifest struct {
	Schema                   int                      `json:"schema"`
	CorpusVersion            string                   `json:"corpus_version"`
	SchemaFile               string                   `json:"schema_file"`
	SchemaSHA256             string                   `json:"schema_sha256"`
	Generator                BroadGeneratorIdentity   `json:"generator"`
	MinimumDenominators      BroadMinimumDenominators `json:"minimum_denominators"`
	MaximumCaseBasisPoints   int                      `json:"maximum_case_basis_points"`
	MinimumSecretFamilyCases int                      `json:"minimum_secret_family_cases"`
	Partitions               []BroadPartitionIdentity `json:"partitions"`
}

type BroadGeneratorIdentity struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Seed    string `json:"seed"`
}

type BroadMinimumDenominators struct {
	SecretRedactCases      int `json:"secret_redact_cases"`
	AccountRedactCases     int `json:"account_redact_cases"`
	PrivateDateRedactCases int `json:"private_date_redact_cases"`
	PreservationCases      int `json:"preservation_cases"`
	LanguageRedactCases    int `json:"language_redact_cases"`
	LanguagePreserveCases  int `json:"language_preserve_cases"`
}

type BroadPartitionIdentity struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	Cases   int    `json:"cases"`
	Targets int    `json:"targets"`
}

type BroadCorpus struct {
	Manifest    BroadCorpusManifest
	Documents   []CorpusDocument
	Cases       []Case
	Fingerprint string
	Coverage    BroadCoverage
}

type BroadCoverage struct {
	SecretRedactCases      int
	AccountRedactCases     int
	PrivateDateRedactCases int
	PreservationCases      int
	LanguageRedactCases    map[string]int
	LanguagePreserveCases  map[string]int
}

func LoadBroadCorpusDir(dir string) (*BroadCorpus, error) {
	manifestRaw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("read broad corpus manifest: %w", err)
	}
	var manifest BroadCorpusManifest
	if err := decodeStrict(manifestRaw, &manifest); err != nil {
		return nil, fmt.Errorf("decode broad corpus manifest: %w", err)
	}
	if err := validateBroadManifest(&manifest); err != nil {
		return nil, err
	}
	schemaPath := filepath.Clean(filepath.Join(dir, manifest.SchemaFile))
	expectedSchemaPath := filepath.Clean(filepath.Join(dir, "..", "..", "v1", "schema.json"))
	if schemaPath != expectedSchemaPath {
		return nil, fmt.Errorf("broad corpus schema path %q is not the locked smoke schema", manifest.SchemaFile)
	}
	schemaRaw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read broad corpus schema: %w", err)
	}
	if hashBytes(schemaRaw) != manifest.SchemaSHA256 {
		return nil, fmt.Errorf("broad corpus schema hash differs from its manifest identity")
	}

	hasher := sha256.New()
	hasher.Write([]byte("manifest.json"))
	hasher.Write([]byte{0})
	hasher.Write(manifestRaw)
	hasher.Write([]byte{0})
	hasher.Write([]byte("schema.json"))
	hasher.Write([]byte{0})
	hasher.Write(schemaRaw)
	hasher.Write([]byte{0})

	documents := make([]CorpusDocument, 0, len(manifest.Partitions))
	cases := make([]Case, 0)
	seenCases := make(map[string]string)
	for _, identity := range manifest.Partitions {
		raw, readErr := os.ReadFile(filepath.Join(dir, identity.File))
		if readErr != nil {
			return nil, fmt.Errorf("read broad corpus partition %q: %w", identity.Name, readErr)
		}
		if hashBytes(raw) != identity.SHA256 {
			return nil, fmt.Errorf("broad corpus partition %q hash differs from its manifest identity", identity.Name)
		}
		if err := ValidateJSONDocument(schemaRaw, raw); err != nil {
			return nil, fmt.Errorf("validate broad corpus partition %q against schema: %w", identity.Name, err)
		}
		var document CorpusDocument
		if err := decodeStrict(raw, &document); err != nil {
			return nil, fmt.Errorf("decode broad corpus partition %q: %w", identity.Name, err)
		}
		if document.Schema != CorpusSchemaVersion || document.Partition != identity.Name || len(document.Cases) != identity.Cases {
			return nil, fmt.Errorf("broad corpus partition %q metadata differs from its manifest", identity.Name)
		}
		targets := 0
		for index := range document.Cases {
			fixture := &document.Cases[index]
			if err := validateCase(fixture); err != nil {
				return nil, err
			}
			if len(fixture.Targets) != 1 {
				return nil, fmt.Errorf("broad corpus case %q must have exactly one target so one case is one percentage unit", fixture.ID)
			}
			if prior, exists := seenCases[fixture.ID]; exists {
				return nil, fmt.Errorf("broad corpus duplicate case ID %q in %q and %q", fixture.ID, prior, identity.Name)
			}
			seenCases[fixture.ID] = identity.Name
			targets += len(fixture.Targets)
			cases = append(cases, *fixture)
		}
		if targets != identity.Targets {
			return nil, fmt.Errorf("broad corpus partition %q target count differs from its manifest", identity.Name)
		}
		hasher.Write([]byte(identity.File))
		hasher.Write([]byte{0})
		hasher.Write(raw)
		hasher.Write([]byte{0})
		documents = append(documents, document)
	}

	coverage, err := validateBroadCoverage(cases, manifest)
	if err != nil {
		return nil, err
	}
	return &BroadCorpus{
		Manifest: manifest, Documents: documents, Cases: cases,
		Fingerprint: hex.EncodeToString(hasher.Sum(nil)), Coverage: coverage,
	}, nil
}

func validateBroadManifest(manifest *BroadCorpusManifest) error {
	if manifest.Schema != BroadCorpusSchemaVersion || manifest.CorpusVersion != BroadCorpusVersion {
		return fmt.Errorf("broad corpus manifest has an unsupported schema or corpus version")
	}
	if manifest.SchemaFile != "../../v1/schema.json" || len(manifest.SchemaSHA256) != sha256HexLength {
		return fmt.Errorf("broad corpus manifest does not pin the smoke corpus schema")
	}
	if manifest.Generator != (BroadGeneratorIdentity{Name: BroadCorpusGenerator, Version: 1, Seed: BroadCorpusGeneratorSeed}) {
		return fmt.Errorf("broad corpus manifest generator identity changed")
	}
	wantMinimums := BroadMinimumDenominators{
		SecretRedactCases: 100, AccountRedactCases: 100, PrivateDateRedactCases: 100,
		PreservationCases: 100, LanguageRedactCases: 100, LanguagePreserveCases: 100,
	}
	if manifest.MinimumDenominators != wantMinimums || manifest.MaximumCaseBasisPoints != 100 || manifest.MinimumSecretFamilyCases != 8 {
		return fmt.Errorf("broad corpus statistical denominator contract changed")
	}
	if len(manifest.Partitions) != len(BroadCorpusPartitions) {
		return fmt.Errorf("broad corpus manifest must contain exactly %d partitions", len(BroadCorpusPartitions))
	}
	for index, expected := range BroadCorpusPartitions {
		identity := manifest.Partitions[index]
		if identity.Name != expected || identity.File != expected+".json" || len(identity.SHA256) != sha256HexLength ||
			identity.Cases <= 0 || identity.Targets != identity.Cases {
			return fmt.Errorf("broad corpus partition identity %d is incomplete or out of order", index)
		}
	}
	return nil
}

func validateBroadCoverage(cases []Case, manifest BroadCorpusManifest) (BroadCoverage, error) {
	coverage := BroadCoverage{
		LanguageRedactCases:   make(map[string]int, len(RequiredLanguages)),
		LanguagePreserveCases: make(map[string]int, len(RequiredLanguages)),
	}
	secretFamilies := make(map[string]map[string]int, len(RequiredSecretFamilies))
	formats := make(map[string]struct{ redact, preserve bool }, len(requiredShapes))
	for _, fixture := range cases {
		target := fixture.Targets[0]
		redact := target.Actions.All == ActionRedact
		if redact {
			coverage.LanguageRedactCases[fixture.Language]++
		} else {
			coverage.LanguagePreserveCases[fixture.Language]++
			coverage.PreservationCases++
		}
		format := formats[fixture.Shape]
		format.redact = format.redact || redact
		format.preserve = format.preserve || !redact
		formats[fixture.Shape] = format
		if redact {
			switch target.Kind {
			case SpanSecret:
				coverage.SecretRedactCases++
			case SpanAccountNumber:
				coverage.AccountRedactCases++
			case SpanPrivateDate:
				coverage.PrivateDateRedactCases++
			}
		}
		if fixture.SecretFixture != nil {
			if secretFamilies[fixture.SecretFixture.Family] == nil {
				secretFamilies[fixture.SecretFixture.Family] = make(map[string]int)
			}
			secretFamilies[fixture.SecretFixture.Family][fixture.SecretFixture.Role]++
		}
	}
	minimums := manifest.MinimumDenominators
	if coverage.SecretRedactCases < minimums.SecretRedactCases ||
		coverage.AccountRedactCases < minimums.AccountRedactCases ||
		coverage.PrivateDateRedactCases < minimums.PrivateDateRedactCases ||
		coverage.PreservationCases < minimums.PreservationCases {
		return BroadCoverage{}, fmt.Errorf("broad corpus does not satisfy a frozen kind or preservation denominator")
	}
	for _, language := range RequiredLanguages {
		if coverage.LanguageRedactCases[language] < minimums.LanguageRedactCases ||
			coverage.LanguagePreserveCases[language] < minimums.LanguagePreserveCases {
			return BroadCoverage{}, fmt.Errorf("broad corpus language %q does not satisfy its redact/preserve denominators", language)
		}
	}
	for _, family := range RequiredSecretFamilies {
		if secretFamilies[family]["must-detect"] < manifest.MinimumSecretFamilyCases ||
			secretFamilies[family]["preserve"] < manifest.MinimumSecretFamilyCases {
			return BroadCoverage{}, fmt.Errorf("broad corpus secret family %q lacks required variations", family)
		}
	}
	for _, formatName := range requiredShapes {
		format := formats[formatName]
		if !format.redact || !format.preserve {
			return BroadCoverage{}, fmt.Errorf("broad corpus format %q lacks redact and preservation coverage", formatName)
		}
	}
	return coverage, nil
}

func hashBytes(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
