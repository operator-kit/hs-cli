package evaluation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const CorpusSchemaVersion = 1

type Mode string

const (
	ModeOff       Mode = "off"
	ModeCustomers Mode = "customers"
	ModeAll       Mode = "all"
)

var Modes = []Mode{ModeOff, ModeCustomers, ModeAll}

type SpanKind string

const (
	SpanPerson        SpanKind = "person"
	SpanAddress       SpanKind = "address"
	SpanEmail         SpanKind = "email"
	SpanPhone         SpanKind = "phone"
	SpanURL           SpanKind = "url"
	SpanPrivateDate   SpanKind = "private_date"
	SpanAccountNumber SpanKind = "account_number"
	SpanSecret        SpanKind = "secret"
)

var SpanKinds = []SpanKind{
	SpanPerson,
	SpanAddress,
	SpanEmail,
	SpanPhone,
	SpanURL,
	SpanPrivateDate,
	SpanAccountNumber,
	SpanSecret,
}

type Action string

const (
	ActionPreserve Action = "preserve"
	ActionRedact   Action = "redact"
)

type MatchPolicy string

const (
	MatchExact    MatchPolicy = "exact"
	MatchCovering MatchPolicy = "covering"
)

type RiskTier string

const (
	RiskCritical     RiskTier = "critical"
	RiskHigh         RiskTier = "high"
	RiskStandard     RiskTier = "standard"
	RiskPreservation RiskTier = "preservation"
)

type CorpusDocument struct {
	Schema    int    `json:"schema"`
	Partition string `json:"partition"`
	Cases     []Case `json:"cases"`
}

type Case struct {
	ID              string             `json:"id"`
	Language        string             `json:"language"`
	Script          string             `json:"script"`
	Shape           string             `json:"shape"`
	Risk            RiskTier           `json:"risk"`
	Reason          string             `json:"reason"`
	Text            string             `json:"text"`
	KnownIdentities []KnownIdentity    `json:"known_identities"`
	Targets         []Target           `json:"targets"`
	Outputs         ModeOutputs        `json:"outputs"`
	Tags            []string           `json:"tags"`
	SecretFixture   *SecretFixtureRole `json:"secret_fixture,omitempty"`
	CorpusTier      string             `json:"-"`
	Partition       string             `json:"-"`
}

type KnownIdentity struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	First string `json:"first"`
	Last  string `json:"last"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type Target struct {
	ID         string               `json:"id"`
	Kind       SpanKind             `json:"kind"`
	Start      int                  `json:"start"`
	End        int                  `json:"end"`
	Value      string               `json:"value"`
	Match      MatchPolicy          `json:"match"`
	IdentityID string               `json:"identity_id,omitempty"`
	Synthetic  *SyntheticProvenance `json:"synthetic,omitempty"`
	Actions    ModeActions          `json:"actions"`
}

type ModeActions struct {
	Off       Action `json:"off"`
	Customers Action `json:"customers"`
	All       Action `json:"all"`
}

func (a ModeActions) For(mode Mode) Action {
	switch mode {
	case ModeOff:
		return a.Off
	case ModeCustomers:
		return a.Customers
	case ModeAll:
		return a.All
	default:
		return ""
	}
}

type ModeOutputs struct {
	Off       OutputExpectation `json:"off"`
	Customers OutputExpectation `json:"customers"`
	All       OutputExpectation `json:"all"`
}

func (o ModeOutputs) For(mode Mode) OutputExpectation {
	switch mode {
	case ModeOff:
		return o.Off
	case ModeCustomers:
		return o.Customers
	case ModeAll:
		return o.All
	default:
		return OutputExpectation{}
	}
}

type OutputExpectation struct {
	RequiredAbsent  []string `json:"required_absent"`
	RequiredPresent []string `json:"required_present"`
}

type SecretFixtureRole struct {
	Family string `json:"family"`
	Role   string `json:"role"`
}

type Corpus struct {
	Documents   []CorpusDocument
	Cases       []Case
	Fingerprint string
}

var RequiredPartitions = []string{
	"secrets",
	"people-third-parties",
	"account-identifiers",
	"private-public-dates",
	"addresses",
	"emails-phones-urls",
	"obfuscated-messy-pii",
	"multilingual",
	"preservation",
	"long-input",
	"command-output-payloads",
}

var RequiredSecretFamilies = []string{
	"api-key",
	"access-token",
	"oauth-token",
	"password",
	"jwt",
	"one-time-code",
	"private-key",
	"database-connection",
	"cookie-authorization",
	"webhook-secret",
	"cloud-credential",
	"source-control-token",
	"payment-credential",
	"observability-token",
}

var RequiredLanguages = []string{"en", "fr", "de", "es", "ar", "zh", "ja", "ko", "hi", "pt"}

var RequiredOutputBoundaries = []string{
	"table", "json", "json-full", "csv", "source", "rfc822", "mcp", "argv",
	"stdout", "stderr", "error", "panic", "diagnostic",
}

var requiredShapes = []string{
	"prose", "json", "yaml", "shell", "url", "markdown", "html", "quoted-reply",
	"log", "stack-trace", "code", "signature", "long-thread", "command-payload",
}

var allowedScripts = stringSet(
	"Latin", "Arabic", "Han", "Hiragana", "Hangul", "Devanagari", "Common", "Mixed",
)

var allowedShapes = stringSet(requiredShapes...)

var allowedRisks = map[RiskTier]bool{
	RiskCritical: true, RiskHigh: true, RiskStandard: true, RiskPreservation: true,
}

var allowedKinds = func() map[SpanKind]bool {
	out := make(map[SpanKind]bool, len(SpanKinds))
	for _, kind := range SpanKinds {
		out[kind] = true
	}
	return out
}()

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func LoadCorpusDir(dir string) (*Corpus, error) {
	documents := make([]CorpusDocument, 0, len(RequiredPartitions))
	cases := make([]Case, 0)
	seenIDs := make(map[string]string)
	hasher := sha256.New()
	schemaRaw, err := os.ReadFile(filepath.Join(dir, "schema.json"))
	if err != nil {
		return nil, fmt.Errorf("read privacy corpus schema: %w", err)
	}
	if _, err := decodeJSONValue(schemaRaw); err != nil {
		return nil, fmt.Errorf("decode privacy corpus schema: %w", err)
	}
	hasher.Write([]byte("schema.json"))
	hasher.Write([]byte{0})
	hasher.Write(schemaRaw)
	hasher.Write([]byte{0})

	for _, partition := range RequiredPartitions {
		name := partition + ".json"
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read privacy corpus partition %q: %w", partition, err)
		}
		if err := ValidateJSONDocument(schemaRaw, raw); err != nil {
			return nil, fmt.Errorf("validate privacy corpus partition %q against schema: %w", partition, err)
		}
		var document CorpusDocument
		if err := decodeStrict(raw, &document); err != nil {
			return nil, fmt.Errorf("decode privacy corpus partition %q: %w", partition, err)
		}
		if document.Schema != CorpusSchemaVersion {
			return nil, fmt.Errorf("privacy corpus partition %q: unsupported schema %d", partition, document.Schema)
		}
		if document.Partition != partition {
			return nil, fmt.Errorf("privacy corpus partition %q: document names partition %q", partition, document.Partition)
		}
		if len(document.Cases) == 0 {
			return nil, fmt.Errorf("privacy corpus partition %q is empty", partition)
		}

		for i := range document.Cases {
			fixture := &document.Cases[i]
			fixture.CorpusTier = "smoke"
			fixture.Partition = partition
			if err := validateCase(fixture); err != nil {
				return nil, err
			}
			if prior, exists := seenIDs[fixture.ID]; exists {
				return nil, fmt.Errorf("privacy corpus duplicate case ID %q in partitions %q and %q", fixture.ID, prior, partition)
			}
			seenIDs[fixture.ID] = partition
			cases = append(cases, *fixture)
		}

		hasher.Write([]byte(name))
		hasher.Write([]byte{0})
		hasher.Write(raw)
		hasher.Write([]byte{0})
		documents = append(documents, document)
	}

	corpus := &Corpus{
		Documents:   documents,
		Cases:       cases,
		Fingerprint: hex.EncodeToString(hasher.Sum(nil)),
	}
	if err := corpus.ValidateCoverage(); err != nil {
		return nil, err
	}
	return corpus, nil
}

func decodeStrict(raw []byte, destination any) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("document is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("document contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing data: %w", err)
	}
	return nil
}

func validateCase(fixture *Case) error {
	prefix := fmt.Sprintf("privacy corpus case %q", fixture.ID)
	if !idPattern.MatchString(fixture.ID) {
		return fmt.Errorf("%s: invalid or empty ID", prefix)
	}
	if fixture.Language == "" || fixture.Script == "" || fixture.Shape == "" || fixture.Reason == "" {
		return fmt.Errorf("%s: language, script, shape, and reason are required", prefix)
	}
	if !allowedScripts[fixture.Script] {
		return fmt.Errorf("%s: unsupported script %q", prefix, fixture.Script)
	}
	if !allowedShapes[fixture.Shape] {
		return fmt.Errorf("%s: unsupported content shape %q", prefix, fixture.Shape)
	}
	if !allowedRisks[fixture.Risk] {
		return fmt.Errorf("%s: unsupported risk tier %q", prefix, fixture.Risk)
	}
	if fixture.Text == "" || !utf8.ValidString(fixture.Text) {
		return fmt.Errorf("%s: text must be non-empty valid UTF-8", prefix)
	}
	if fixture.KnownIdentities == nil || fixture.Targets == nil || fixture.Tags == nil {
		return fmt.Errorf("%s: known_identities, targets, and tags arrays are required", prefix)
	}
	if len(fixture.Targets) == 0 {
		return fmt.Errorf("%s: at least one typed target is required", prefix)
	}

	known := make(map[string]KnownIdentity, len(fixture.KnownIdentities))
	for _, identity := range fixture.KnownIdentities {
		if !idPattern.MatchString(identity.ID) {
			return fmt.Errorf("%s: known identity has an invalid ID", prefix)
		}
		if identity.Type != "customer" && identity.Type != "user" {
			return fmt.Errorf("%s: identity %q has unsupported type %q", prefix, identity.ID, identity.Type)
		}
		if identity.First == "" && identity.Last == "" && identity.Email == "" && identity.Phone == "" {
			return fmt.Errorf("%s: identity %q has no identifying fields", prefix, identity.ID)
		}
		if _, exists := known[identity.ID]; exists {
			return fmt.Errorf("%s: duplicate known identity ID %q", prefix, identity.ID)
		}
		known[identity.ID] = identity
	}

	seenTargets := make(map[string]struct{}, len(fixture.Targets))
	for i := range fixture.Targets {
		target := &fixture.Targets[i]
		if !idPattern.MatchString(target.ID) {
			return fmt.Errorf("%s: target %d has an invalid ID", prefix, i)
		}
		if _, exists := seenTargets[target.ID]; exists {
			return fmt.Errorf("%s: duplicate target ID %q", prefix, target.ID)
		}
		seenTargets[target.ID] = struct{}{}
		if !allowedKinds[target.Kind] {
			return fmt.Errorf("%s: target %q has unsupported kind %q", prefix, target.ID, target.Kind)
		}
		if target.Match != MatchExact && target.Match != MatchCovering {
			return fmt.Errorf("%s: target %q has unsupported match policy %q", prefix, target.ID, target.Match)
		}
		if !validByteRange(fixture.Text, target.Start, target.End) {
			return fmt.Errorf("%s: target %q has an invalid UTF-8 byte range [%d:%d]", prefix, target.ID, target.Start, target.End)
		}
		if target.Value == "" || fixture.Text[target.Start:target.End] != target.Value {
			return fmt.Errorf("%s: target %q value does not match its declared byte range", prefix, target.ID)
		}
		if target.Kind == SpanSecret {
			if target.Synthetic == nil {
				return fmt.Errorf("%s: secret target %q lacks synthetic provenance", prefix, target.ID)
			}
			generated, err := GenerateSyntheticValue(*target.Synthetic)
			if err != nil {
				return fmt.Errorf("%s: secret target %q provenance: %w", prefix, target.ID, err)
			}
			if generated != target.Value {
				return fmt.Errorf("%s: secret target %q does not match its deterministic synthetic provenance", prefix, target.ID)
			}
			if target.Synthetic.Purpose == "must-detect" &&
				(target.Actions.Customers != ActionRedact || target.Actions.All != ActionRedact) {
				return fmt.Errorf("%s: must-detect secret target %q must redact in customers and all", prefix, target.ID)
			}
			if target.Synthetic.Purpose == "preserve" &&
				(target.Actions.Customers != ActionPreserve || target.Actions.All != ActionPreserve) {
				return fmt.Errorf("%s: preservation secret target %q must preserve in customers and all", prefix, target.ID)
			}
		} else if target.Synthetic != nil {
			return fmt.Errorf("%s: non-secret target %q declares secret synthetic provenance", prefix, target.ID)
		}
		if target.IdentityID != "" {
			identity, exists := known[target.IdentityID]
			if !exists {
				return fmt.Errorf("%s: target %q references unknown identity %q", prefix, target.ID, target.IdentityID)
			}
			if target.Kind == SpanSecret || target.Kind == SpanAccountNumber || target.Kind == SpanPrivateDate {
				return fmt.Errorf("%s: target %q kind %q cannot be protected by identity attribution", prefix, target.ID, target.Kind)
			}
			if target.Kind == SpanPerson && identity.Type == "user" &&
				(target.Actions.Customers != ActionPreserve || target.Actions.All != ActionRedact) {
				return fmt.Errorf("%s: known staff person target %q must preserve in customers and redact in all", prefix, target.ID)
			}
			if target.Kind == SpanPerson && identity.Type == "customer" &&
				(target.Actions.Customers != ActionRedact || target.Actions.All != ActionRedact) {
				return fmt.Errorf("%s: known customer person target %q must redact in customers and all", prefix, target.ID)
			}
		}
		if err := validateActions(prefix, target); err != nil {
			return err
		}
	}

	for i := 0; i < len(fixture.Targets); i++ {
		for j := i + 1; j < len(fixture.Targets); j++ {
			left, right := fixture.Targets[i], fixture.Targets[j]
			if left.Start < right.End && right.Start < left.End && left.Actions != right.Actions {
				return fmt.Errorf("%s: overlapping targets %q and %q have contradictory actions", prefix, left.ID, right.ID)
			}
		}
	}

	if err := validateOutputs(prefix, fixture); err != nil {
		return err
	}
	if err := validateUniqueStrings(prefix, "tags", fixture.Tags); err != nil {
		return err
	}
	boundaryTags := 0
	for _, tag := range fixture.Tags {
		if strings.HasPrefix(tag, "boundary:") {
			boundaryTags++
			if !contains(RequiredOutputBoundaries, strings.TrimPrefix(tag, "boundary:")) {
				return fmt.Errorf("%s: unknown output boundary tag %q", prefix, tag)
			}
		}
	}
	if boundaryTags > 1 {
		return fmt.Errorf("%s: each fixture may exercise at most one output boundary", prefix)
	}
	if fixture.SecretFixture != nil {
		if !contains(RequiredSecretFamilies, fixture.SecretFixture.Family) {
			return fmt.Errorf("%s: unsupported secret family %q", prefix, fixture.SecretFixture.Family)
		}
		if fixture.SecretFixture.Role != "must-detect" && fixture.SecretFixture.Role != "preserve" {
			return fmt.Errorf("%s: unsupported secret fixture role %q", prefix, fixture.SecretFixture.Role)
		}
		if err := validateSecretFixture(prefix, fixture); err != nil {
			return err
		}
	}
	return nil
}

func validateActions(prefix string, target *Target) error {
	if target.Actions.Off != ActionPreserve {
		return fmt.Errorf("%s: target %q must preserve exact display text in off mode", prefix, target.ID)
	}
	for _, mode := range []Mode{ModeCustomers, ModeAll} {
		action := target.Actions.For(mode)
		if action != ActionPreserve && action != ActionRedact {
			return fmt.Errorf("%s: target %q has invalid %s action %q", prefix, target.ID, mode, action)
		}
	}
	if target.Kind == SpanSecret && (target.Actions.Customers != target.Actions.All) {
		return fmt.Errorf("%s: secret target %q must have identical customers/all policy", prefix, target.ID)
	}
	return nil
}

func validateOutputs(prefix string, fixture *Case) error {
	for _, mode := range Modes {
		expected := fixture.Outputs.For(mode)
		if expected.RequiredAbsent == nil || expected.RequiredPresent == nil {
			return fmt.Errorf("%s: %s output must declare required_absent and required_present arrays", prefix, mode)
		}
		if err := validateUniqueStrings(prefix, string(mode)+" required_absent", expected.RequiredAbsent); err != nil {
			return err
		}
		if err := validateUniqueStrings(prefix, string(mode)+" required_present", expected.RequiredPresent); err != nil {
			return err
		}
		for _, value := range append(append([]string(nil), expected.RequiredAbsent...), expected.RequiredPresent...) {
			if !strings.Contains(fixture.Text, value) {
				return fmt.Errorf("%s: %s output expectation is absent from source text", prefix, mode)
			}
		}
		for _, absent := range expected.RequiredAbsent {
			if contains(expected.RequiredPresent, absent) {
				return fmt.Errorf("%s: %s output requires the same string absent and present", prefix, mode)
			}
		}
		if mode == ModeOff && len(expected.RequiredAbsent) != 0 {
			return fmt.Errorf("%s: off mode cannot require source text to be absent", prefix)
		}
		for _, target := range fixture.Targets {
			if target.Actions.For(mode) == ActionRedact && !contains(expected.RequiredAbsent, target.Value) {
				return fmt.Errorf("%s: %s output does not require redacted target %q to be absent", prefix, mode, target.ID)
			}
			if target.Actions.For(mode) == ActionPreserve && !contains(expected.RequiredPresent, target.Value) {
				return fmt.Errorf("%s: %s output does not require preserved target %q to be present", prefix, mode, target.ID)
			}
		}
	}
	return nil
}

func validateSecretFixture(prefix string, fixture *Case) error {
	hasSecret := false
	for _, target := range fixture.Targets {
		if target.Kind != SpanSecret {
			continue
		}
		hasSecret = true
		if target.Synthetic == nil || target.Synthetic.Recipe != fixture.SecretFixture.Family ||
			target.Synthetic.Purpose != fixture.SecretFixture.Role {
			return fmt.Errorf("%s: secret target %q provenance contradicts the fixture family or role", prefix, target.ID)
		}
	}
	if !hasSecret {
		return fmt.Errorf("%s: secret_fixture requires a typed secret target", prefix)
	}
	return nil
}

func validByteRange(text string, start, end int) bool {
	if start < 0 || end <= start || end > len(text) {
		return false
	}
	if start > 0 && !utf8.RuneStart(text[start]) {
		return false
	}
	if end < len(text) && !utf8.RuneStart(text[end]) {
		return false
	}
	return utf8.ValidString(text[start:end])
}

func validateUniqueStrings(prefix, field string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("%s: %s contains an empty value", prefix, field)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s: %s contains a duplicate value", prefix, field)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func (c *Corpus) ValidateCoverage() error {
	type outcomes struct{ redact, preserve bool }
	kinds := make(map[SpanKind]*outcomes, len(SpanKinds))
	for _, kind := range SpanKinds {
		kinds[kind] = &outcomes{}
	}
	languages := make(map[string]*outcomes, len(RequiredLanguages))
	for _, language := range RequiredLanguages {
		languages[language] = &outcomes{}
	}
	partitions := make(map[string]*outcomes, len(RequiredPartitions))
	for _, partition := range RequiredPartitions {
		partitions[partition] = &outcomes{}
	}
	secretFamilies := make(map[string]*outcomes, len(RequiredSecretFamilies))
	for _, family := range RequiredSecretFamilies {
		secretFamilies[family] = &outcomes{}
	}
	shapes := make(map[string]*outcomes, len(requiredShapes))
	for _, shape := range requiredShapes {
		shapes[shape] = &outcomes{}
	}
	boundaries := make(map[string]*outcomes, len(RequiredOutputBoundaries))
	for _, boundary := range RequiredOutputBoundaries {
		boundaries[boundary] = &outcomes{}
	}

	for _, document := range c.Documents {
		partitionOutcome := partitions[document.Partition]
		for _, fixture := range document.Cases {
			hasRedact, hasPreserve := false, false
			for _, target := range fixture.Targets {
				if target.Actions.All == ActionRedact {
					hasRedact = true
					kinds[target.Kind].redact = true
				} else {
					hasPreserve = true
					kinds[target.Kind].preserve = true
				}
			}
			partitionOutcome.redact = partitionOutcome.redact || hasRedact
			partitionOutcome.preserve = partitionOutcome.preserve || hasPreserve
			if languageOutcome := languages[fixture.Language]; languageOutcome != nil {
				languageOutcome.redact = languageOutcome.redact || hasRedact
				languageOutcome.preserve = languageOutcome.preserve || hasPreserve
			}
			shapeOutcome := shapes[fixture.Shape]
			shapeOutcome.redact = shapeOutcome.redact || hasRedact
			shapeOutcome.preserve = shapeOutcome.preserve || hasPreserve
			for _, tag := range fixture.Tags {
				if strings.HasPrefix(tag, "boundary:") {
					boundaryOutcome := boundaries[strings.TrimPrefix(tag, "boundary:")]
					if boundaryOutcome != nil {
						boundaryOutcome.redact = boundaryOutcome.redact || hasRedact
						boundaryOutcome.preserve = boundaryOutcome.preserve || hasPreserve
					}
				}
			}
			if fixture.SecretFixture != nil {
				family := secretFamilies[fixture.SecretFixture.Family]
				if fixture.SecretFixture.Role == "must-detect" {
					family.redact = true
				} else {
					family.preserve = true
				}
			}
		}
	}

	for _, kind := range SpanKinds {
		if outcome := kinds[kind]; !outcome.redact || !outcome.preserve {
			return fmt.Errorf("privacy corpus kind %q requires redact and preservation coverage", kind)
		}
	}
	for _, language := range RequiredLanguages {
		if outcome := languages[language]; !outcome.redact || !outcome.preserve {
			return fmt.Errorf("privacy corpus language %q requires redact and preservation coverage", language)
		}
	}
	for _, partition := range RequiredPartitions {
		if outcome := partitions[partition]; !outcome.redact || !outcome.preserve {
			return fmt.Errorf("privacy corpus partition %q requires redact and preservation coverage", partition)
		}
	}
	for _, family := range RequiredSecretFamilies {
		if outcome := secretFamilies[family]; !outcome.redact || !outcome.preserve {
			return fmt.Errorf("secret family %q requires must-detect and preservation fixtures", family)
		}
	}
	for _, shape := range requiredShapes {
		if outcome := shapes[shape]; !outcome.redact || !outcome.preserve {
			return fmt.Errorf("privacy corpus content shape %q requires redact and preservation coverage", shape)
		}
	}
	for _, boundary := range RequiredOutputBoundaries {
		if outcome := boundaries[boundary]; !outcome.redact || !outcome.preserve {
			return fmt.Errorf("privacy corpus output boundary %q requires independent redact and preservation coverage", boundary)
		}
	}
	return nil
}

func HasTag(fixture Case, wanted string) bool {
	return contains(fixture.Tags, wanted)
}

func HashFile(path string) (string, error) {
	return HashFiles(path)
}

func HashFiles(paths ...string) (string, error) {
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	hasher := sha256.New()
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		hasher.Write([]byte(filepath.Base(path)))
		hasher.Write([]byte{0})
		hasher.Write(raw)
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func stringSet(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func SortedCaseIDs(corpus *Corpus) []string {
	ids := make([]string, 0, len(corpus.Cases))
	for _, fixture := range corpus.Cases {
		ids = append(ids, fixture.ID)
	}
	sort.Strings(ids)
	return ids
}
