package evaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GeneratePrivacyFilterTestdata rewrites only deterministic synthetic fixture
// files. It is used by the checked generator command and byte-for-byte tests.
func GeneratePrivacyFilterTestdata(repoRoot string) error {
	smokeDir := filepath.Join(repoRoot, "internal", "pii", "testdata", "privacy-filter", "v1")
	broadDir := filepath.Join(repoRoot, "internal", "pii", "testdata", "privacy-filter", "broad", "v1")
	if err := writeCorpusDocument(filepath.Join(smokeDir, "secrets.json"), generateSmokeSecrets()); err != nil {
		return err
	}
	if err := rewriteCommandBoundaryFixtures(filepath.Join(smokeDir, "command-output-payloads.json")); err != nil {
		return err
	}
	if err := rewriteSupplementalPreservationFixtures(filepath.Join(smokeDir, "preservation.json")); err != nil {
		return err
	}
	return generateBroadCorpus(smokeDir, broadDir)
}

type secretContext struct {
	shape           string
	detectContext   string
	preserveContext string
}

var smokeSecretContexts = map[string]secretContext{
	"api-key":              {"prose", "The integration credential is ", "The configuration field is "},
	"access-token":         {"json", "The response included ", "The schema property is "},
	"oauth-token":          {"yaml", "The authorization value is ", "The authorization scheme is "},
	"password":             {"shell", "The copied password is ", "The documented setting is "},
	"jwt":                  {"log", "The authentication token was ", "The token grammar is "},
	"one-time-code":        {"prose", "The recovery code is ", "The public build number is "},
	"private-key":          {"code", "The pasted credential block follows:\n", "The published verification material follows:\n"},
	"database-connection":  {"url", "The service connection is ", "The public service endpoint is "},
	"cookie-authorization": {"quoted-reply", "The copied cookie was ", "The cookie policy attribute is "},
	"webhook-secret":       {"html", "The webhook signing value is ", "The webhook event name is "},
	"cloud-credential":     {"stack-trace", "The cloud access identifier is ", "The public cloud account number is "},
	"source-control-token": {"markdown", "The repository token is ", "The public commit identifier is "},
	"payment-credential":   {"json", "The payment credential is ", "The publishable client identifier is "},
	"observability-token":  {"yaml", "The telemetry credential is ", "The public trace identifier is "},
}

func generateSmokeSecrets() CorpusDocument {
	document := CorpusDocument{Schema: CorpusSchemaVersion, Partition: "secrets", Cases: make([]Case, 0, len(RequiredSecretFamilies)*2)}
	for _, family := range RequiredSecretFamilies {
		context := smokeSecretContexts[family]
		for _, role := range []string{"must-detect", "preserve"} {
			seed := "smoke-" + family + "-" + role
			provenance := SyntheticProvenance{
				Generator: SyntheticSecretGenerator, Version: SyntheticSecretVersion,
				Seed: seed, Recipe: family, Purpose: role,
			}
			value, err := GenerateSyntheticValue(provenance)
			if err != nil {
				panic(err)
			}
			contextText := context.detectContext
			if role == "preserve" {
				contextText = context.preserveContext
			}
			text, start := wrapFixtureShape(context.shape, contextText, value, 0)
			actions := ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact}
			risk := RiskCritical
			match := MatchCovering
			if role == "preserve" {
				actions = ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}
				risk = RiskPreservation
				match = MatchExact
			}
			document.Cases = append(document.Cases, makeGeneratedCase(
				"secret-"+family+"-"+role, "en", "Latin", context.shape, risk,
				"Offline-generated "+family+" "+role+" contract fixture.", text,
				Target{ID: "secret-value", Kind: SpanSecret, Start: start, End: start + len(value), Value: value,
					Match: match, Synthetic: &provenance, Actions: actions},
				[]string{"secret", "secret-family:" + family, "synthetic-generator:v1"},
				&SecretFixtureRole{Family: family, Role: role},
			))
		}
	}
	return document
}

func rewriteCommandBoundaryFixtures(path string) error {
	document, err := readCorpusDocument(path)
	if err != nil {
		return err
	}
	retained := make([]Case, 0, len(document.Cases)+len(RequiredOutputBoundaries)*2)
	for _, fixture := range document.Cases {
		if fixture.ID == "command-secret-all-boundaries" || strings.HasPrefix(fixture.ID, "command-boundary-") ||
			strings.HasPrefix(fixture.ID, "command-secret-boundary-") {
			continue
		}
		retained = append(retained, fixture)
	}
	generated := make([]Case, 0, len(RequiredOutputBoundaries)*2)
	for _, boundary := range RequiredOutputBoundaries {
		for _, role := range []string{"must-detect", "preserve"} {
			recipe := "command-secret"
			actions := ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact}
			risk := RiskCritical
			match := MatchCovering
			if role == "preserve" {
				recipe = "checksum"
				actions = ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}
				risk = RiskPreservation
				match = MatchExact
			}
			provenance := SyntheticProvenance{
				Generator: SyntheticSecretGenerator, Version: SyntheticSecretVersion,
				Seed: "smoke-boundary-" + boundary + "-" + role, Recipe: recipe, Purpose: role,
			}
			value, generationErr := GenerateSyntheticValue(provenance)
			if generationErr != nil {
				return generationErr
			}
			ticket := "TICKET-EXAMPLE-42"
			text := fmt.Sprintf(`{"ticket":"%s","message":"value=%s"}`, ticket, value)
			secretStart := strings.Index(text, value)
			ticketStart := strings.Index(text, ticket)
			fixture := makeGeneratedCase(
				"command-boundary-"+boundary+"-"+role, "en", "Latin", "command-payload", risk,
				"The independently exercised "+boundary+" boundary has a locked "+role+" expectation.", text,
				Target{ID: "secret-shape", Kind: SpanSecret, Start: secretStart, End: secretStart + len(value), Value: value,
					Match: match, Synthetic: &provenance, Actions: actions},
				[]string{"command-output", "boundary:" + boundary, "synthetic-generator:v1"}, nil,
			)
			fixture.Targets = append([]Target{{
				ID: "ticket", Kind: SpanAccountNumber, Start: ticketStart, End: ticketStart + len(ticket), Value: ticket,
				Match: MatchExact, Actions: ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve},
			}}, fixture.Targets...)
			for _, mode := range Modes {
				expected := fixture.Outputs.For(mode)
				expected.RequiredPresent = append(expected.RequiredPresent, ticket)
				sort.Strings(expected.RequiredPresent)
				setOutputExpectation(&fixture.Outputs, mode, expected)
			}
			generated = append(generated, fixture)
		}
	}
	document.Cases = append(generated, retained...)
	return writeCorpusDocument(path, document)
}

func rewriteSupplementalPreservationFixtures(path string) error {
	document, err := readCorpusDocument(path)
	if err != nil {
		return err
	}
	retained := document.Cases[:0]
	for _, fixture := range document.Cases {
		if fixture.ID != "preservation-signature-public-organization" {
			retained = append(retained, fixture)
		}
	}
	document.Cases = retained
	for index := range document.Cases {
		fixture := &document.Cases[index]
		switch fixture.ID {
		case "preservation-checksum-and-email":
			provenance := SyntheticProvenance{Generator: SyntheticSecretGenerator, Version: 1, Seed: "smoke-checksum-preserve", Recipe: "checksum", Purpose: "preserve"}
			if err := replaceGeneratedTarget(fixture, "checksum", provenance); err != nil {
				return err
			}
		case "preservation-redacted-marker":
			provenance := SyntheticProvenance{Generator: SyntheticSecretGenerator, Version: 1, Seed: "smoke-redacted-marker-preserve", Recipe: "redacted-marker", Purpose: "preserve"}
			if err := replaceGeneratedTarget(fixture, "redacted-marker", provenance); err != nil {
				return err
			}
		}
	}
	value := "Atlas Labs"
	text := "--\nAtlas Labs Support\nDocumentation: https://docs.example.invalid"
	start := strings.Index(text, value)
	document.Cases = append(document.Cases, makeGeneratedCase(
		"preservation-signature-public-organization", "en", "Latin", "signature", RiskPreservation,
		"A public organization in a signature must remain unchanged.", text,
		Target{ID: "public-organization", Kind: SpanPerson, Start: start, End: start + len(value), Value: value,
			Match: MatchExact, Actions: ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}},
		[]string{"preservation", "format:signature"}, nil,
	))
	return writeCorpusDocument(path, document)
}

func replaceGeneratedTarget(fixture *Case, targetID string, provenance SyntheticProvenance) error {
	value, err := GenerateSyntheticValue(provenance)
	if err != nil {
		return err
	}
	old := ""
	for index := range fixture.Targets {
		if fixture.Targets[index].ID == targetID {
			old = fixture.Targets[index].Value
			fixture.Targets[index].Value = value
			fixture.Targets[index].Synthetic = &provenance
		}
	}
	if old == "" {
		return fmt.Errorf("case %q lacks generated target %q", fixture.ID, targetID)
	}
	fixture.Text = strings.ReplaceAll(fixture.Text, old, value)
	for index := range fixture.Targets {
		start := strings.Index(fixture.Text, fixture.Targets[index].Value)
		if start < 0 {
			return fmt.Errorf("case %q target %q disappeared while regenerating", fixture.ID, fixture.Targets[index].ID)
		}
		fixture.Targets[index].Start = start
		fixture.Targets[index].End = start + len(fixture.Targets[index].Value)
	}
	for _, mode := range Modes {
		expected := fixture.Outputs.For(mode)
		for index := range expected.RequiredAbsent {
			if expected.RequiredAbsent[index] == old {
				expected.RequiredAbsent[index] = value
			}
		}
		for index := range expected.RequiredPresent {
			if expected.RequiredPresent[index] == old {
				expected.RequiredPresent[index] = value
			}
		}
		setOutputExpectation(&fixture.Outputs, mode, expected)
	}
	return nil
}

func generateBroadCorpus(smokeDir, broadDir string) error {
	if err := os.MkdirAll(broadDir, 0o755); err != nil {
		return fmt.Errorf("create broad corpus directory: %w", err)
	}
	documents := []CorpusDocument{
		generateBroadSecrets(),
		generateBroadAccounts(),
		generateBroadDates(),
		generateBroadLanguages(),
		generateBroadPreservation(),
	}
	identities := make([]BroadPartitionIdentity, 0, len(documents))
	for _, document := range documents {
		raw, err := marshalBroadCorpusDocument(document)
		if err != nil {
			return err
		}
		path := filepath.Join(broadDir, document.Partition+".json")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return fmt.Errorf("write broad corpus partition %q: %w", document.Partition, err)
		}
		targets := 0
		for _, fixture := range document.Cases {
			targets += len(fixture.Targets)
		}
		identities = append(identities, BroadPartitionIdentity{
			Name: document.Partition, File: document.Partition + ".json", SHA256: hashBytes(raw),
			Cases: len(document.Cases), Targets: targets,
		})
	}
	schemaRaw, err := os.ReadFile(filepath.Join(smokeDir, "schema.json"))
	if err != nil {
		return fmt.Errorf("read smoke schema for broad manifest: %w", err)
	}
	manifest := BroadCorpusManifest{
		Schema: BroadCorpusSchemaVersion, CorpusVersion: BroadCorpusVersion,
		SchemaFile: "../../v1/schema.json", SchemaSHA256: hashBytes(schemaRaw),
		Generator: BroadGeneratorIdentity{Name: BroadCorpusGenerator, Version: 1, Seed: BroadCorpusGeneratorSeed},
		MinimumDenominators: BroadMinimumDenominators{
			SecretRedactCases: 100, AccountRedactCases: 100, PrivateDateRedactCases: 100,
			PreservationCases: 100, LanguageRedactCases: 100, LanguagePreserveCases: 100,
		},
		MaximumCaseBasisPoints: 100, MinimumSecretFamilyCases: 8, Partitions: identities,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode broad corpus manifest: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(filepath.Join(broadDir, "manifest.json"), raw, 0o644); err != nil {
		return fmt.Errorf("write broad corpus manifest: %w", err)
	}
	return nil
}

func generateBroadSecrets() CorpusDocument {
	document := CorpusDocument{Schema: 1, Partition: "secrets", Cases: make([]Case, 0, len(RequiredSecretFamilies)*16)}
	detectContexts := []string{
		"Credential copied from a support reply: ", "Authorization header value: ", "Log field auth_value=",
		"Rotating this exposed value: ", "Quoted credential follows: ", "Configuration secret: ",
		"Stack trace captured credential ", "Wrapped message credential: ",
	}
	preserveContexts := []string{
		"Public configuration vocabulary: ", "Documentation syntax: ", "Public operational identifier: ",
		"Schema field label: ", "Non-confidential protocol token: ", "Release metadata: ",
		"Public verification material: ", "Troubleshooting reference: ",
	}
	for familyIndex, family := range RequiredSecretFamilies {
		for variation := 0; variation < 8; variation++ {
			for _, role := range []string{"must-detect", "preserve"} {
				provenance := SyntheticProvenance{
					Generator: SyntheticSecretGenerator, Version: 1,
					Seed: fmt.Sprintf("broad-secret-%s-%s-%02d", family, role, variation), Recipe: family, Purpose: role,
				}
				value, err := GenerateSyntheticValue(provenance)
				if err != nil {
					panic(err)
				}
				shape := requiredShapes[(familyIndex*8+variation)%len(requiredShapes)]
				context := detectContexts[variation]
				actions := ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact}
				risk := RiskCritical
				match := MatchCovering
				if role == "preserve" {
					context = preserveContexts[variation]
					actions = ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}
					risk = RiskPreservation
					match = MatchExact
				}
				text, start := wrapFixtureShape(shape, context, value, variation)
				document.Cases = append(document.Cases, makeGeneratedCase(
					fmt.Sprintf("broad-secret-%s-%s-%02d", family, role, variation), "en", "Latin", shape, risk,
					"Locked broad "+family+" variation.", text,
					Target{ID: "value", Kind: SpanSecret, Start: start, End: start + len(value), Value: value,
						Match: match, Synthetic: &provenance, Actions: actions},
					[]string{"broad-quality", "secret-family:" + family, fmt.Sprintf("variation:%02d", variation)},
					&SecretFixtureRole{Family: family, Role: role},
				))
			}
		}
	}
	return document
}

func generateBroadAccounts() CorpusDocument {
	document := CorpusDocument{Schema: 1, Partition: "account-identifiers", Cases: make([]Case, 0, 200)}
	for index := 0; index < 100; index++ {
		for _, preserve := range []bool{false, true} {
			role := "redact"
			prefix := "Customer account reference: "
			value := fmt.Sprintf("CUST-%04d-%s", index, generatedPublicCode("account-private", index, 8))
			actions := ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact}
			risk := RiskHigh
			if preserve {
				role = "preserve"
				prefix = "Published documentation reference: "
				value = fmt.Sprintf("DOC-%04d-%s", index, generatedPublicCode("account-public", index, 8))
				actions = ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}
				risk = RiskPreservation
			}
			shape := requiredShapes[index%len(requiredShapes)]
			text, start := wrapFixtureShape(shape, prefix, value, index)
			document.Cases = append(document.Cases, makeGeneratedCase(
				fmt.Sprintf("broad-account-%s-%03d", role, index), "en", "Latin", shape, risk,
				"Locked broad account identifier variation.", text,
				Target{ID: "account", Kind: SpanAccountNumber, Start: start, End: start + len(value), Value: value,
					Match: MatchExact, Actions: actions}, []string{"broad-quality", "account-identifier"}, nil,
			))
		}
	}
	return document
}

func generateBroadDates() CorpusDocument {
	document := CorpusDocument{Schema: 1, Partition: "private-public-dates", Cases: make([]Case, 0, 200)}
	for index := 0; index < 100; index++ {
		date := fmt.Sprintf("%04d-%02d-%02d", 1960+index%50, 1+(index/7)%12, 1+index%28)
		for _, preserve := range []bool{false, true} {
			role := "redact"
			prefix := "Customer date of birth: "
			actions := ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact}
			risk := RiskHigh
			if preserve {
				role = "preserve"
				prefix = "Public release date: "
				actions = ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}
				risk = RiskPreservation
			}
			shape := requiredShapes[(index+3)%len(requiredShapes)]
			text, start := wrapFixtureShape(shape, prefix, date, index)
			document.Cases = append(document.Cases, makeGeneratedCase(
				fmt.Sprintf("broad-date-%s-%03d", role, index), "en", "Latin", shape, risk,
				"Locked broad private/public date variation.", text,
				Target{ID: "date", Kind: SpanPrivateDate, Start: start, End: start + len(date), Value: date,
					Match: MatchExact, Actions: actions}, []string{"broad-quality", "date-policy"}, nil,
			))
		}
	}
	return document
}

type languageGeneratorProfile struct {
	code, script, separator, privateContext, publicContext string
	privateLeft, privateRight, publicLeft, publicRight     []string
}

func generateBroadLanguages() CorpusDocument {
	profiles := broadLanguageProfiles()
	document := CorpusDocument{Schema: 1, Partition: "multilingual", Cases: make([]Case, 0, len(profiles)*200)}
	for _, profile := range profiles {
		for index := 0; index < 100; index++ {
			for _, preserve := range []bool{false, true} {
				role := "redact"
				value := profile.privateLeft[index/10] + profile.separator + profile.privateRight[index%10]
				context := profile.privateContext
				actions := ModeActions{Off: ActionPreserve, Customers: ActionRedact, All: ActionRedact}
				risk := RiskHigh
				if preserve {
					role = "preserve"
					value = profile.publicLeft[index/10] + profile.separator + profile.publicRight[index%10]
					context = profile.publicContext
					actions = ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}
					risk = RiskPreservation
				}
				shape := requiredShapes[index%len(requiredShapes)]
				text, start := wrapFixtureShape(shape, context, value, index)
				document.Cases = append(document.Cases, makeGeneratedCase(
					fmt.Sprintf("broad-language-%s-%s-%03d", profile.code, role, index), profile.code, profile.script, shape, risk,
					"Locked broad multilingual person/preservation variation.", text,
					Target{ID: "person", Kind: SpanPerson, Start: start, End: start + len(value), Value: value,
						Match: MatchExact, Actions: actions}, []string{"broad-quality", "multilingual"}, nil,
				))
			}
		}
	}
	return document
}

func generateBroadPreservation() CorpusDocument {
	document := CorpusDocument{Schema: 1, Partition: "preservation", Cases: make([]Case, 0, 100)}
	for index := 0; index < 100; index++ {
		provenance := SyntheticProvenance{
			Generator: SyntheticSecretGenerator, Version: 1, Seed: fmt.Sprintf("broad-checksum-preserve-%03d", index),
			Recipe: "checksum", Purpose: "preserve",
		}
		value, err := GenerateSyntheticValue(provenance)
		if err != nil {
			panic(err)
		}
		shape := requiredShapes[(index+7)%len(requiredShapes)]
		text, start := wrapFixtureShape(shape, "Published artifact checksum: ", value, index)
		document.Cases = append(document.Cases, makeGeneratedCase(
			fmt.Sprintf("broad-preservation-checksum-%03d", index), "en", "Latin", shape, RiskPreservation,
			"Locked broad high-entropy preservation variation.", text,
			Target{ID: "checksum", Kind: SpanSecret, Start: start, End: start + len(value), Value: value, Match: MatchExact,
				Synthetic: &provenance, Actions: ModeActions{Off: ActionPreserve, Customers: ActionPreserve, All: ActionPreserve}},
			[]string{"broad-quality", "preservation", "checksum"}, nil,
		))
	}
	return document
}

func broadLanguageProfiles() []languageGeneratorProfile {
	latinFirst := []string{"Amina", "Bruno", "Clara", "Diego", "Elena", "Farid", "Greta", "Hugo", "Ines", "Jonas"}
	latinLast := []string{"Arden", "Borel", "Costa", "Durand", "Esteves", "Fischer", "Garcia", "Holm", "Ibarra", "Jensen"}
	latinPublic := []string{"Atlas", "Boreal", "Cedar", "Delta", "Ember", "Flora", "Gaia", "Helios", "Ion", "Juniper"}
	latinEntity := []string{"Labs", "Works", "Studio", "Systems", "Collective", "Foundation", "Network", "Group", "Project", "Press"}
	return []languageGeneratorProfile{
		{"en", "Latin", " ", "Customer mentioned ", "Public organization ", latinFirst, latinLast, latinPublic, latinEntity},
		{"fr", "Latin", " ", "Le client mentionne ", "Organisation publique ", latinFirst, latinLast, latinPublic, latinEntity},
		{"de", "Latin", " ", "Der Kunde nennt ", "Öffentliche Organisation ", latinFirst, latinLast, latinPublic, latinEntity},
		{"es", "Latin", " ", "El cliente menciona a ", "Organización pública ", latinFirst, latinLast, latinPublic, latinEntity},
		{"pt", "Latin", " ", "O cliente mencionou ", "Organização pública ", latinFirst, latinLast, latinPublic, latinEntity},
		{"ar", "Arabic", " ", "ذكر العميل ", "المؤسسة العامة ",
			[]string{"أمين", "باسل", "جمانة", "داليا", "رامي", "سلمى", "طارق", "عالية", "فارس", "ليلى"},
			[]string{"الحداد", "البكري", "الجابر", "الخطيب", "الدرويش", "الراوي", "السالم", "الطائي", "العلي", "المصري"},
			[]string{"أطلس", "بستان", "جسر", "دليل", "رؤية", "سحاب", "شمس", "طيف", "عمران", "فضاء"},
			[]string{"العامة", "للبحوث", "للنشر", "للعلوم", "للفنون", "للتقنية", "للثقافة", "للتعليم", "للبيئة", "للتنمية"}},
		{"zh", "Han", "", "客户提到了", "公共机构", []string{"王", "李", "张", "刘", "陈", "杨", "赵", "黄", "周", "吴"},
			[]string{"明", "芳", "伟", "秀英", "强", "磊", "洋", "艳", "勇", "静"},
			[]string{"北辰", "长河", "春山", "东海", "飞云", "光华", "禾木", "金石", "蓝桥", "明川"},
			[]string{"研究院", "出版社", "基金会", "实验室", "中心", "协会", "大学", "博物馆", "图书馆", "剧团"}},
		{"ja", "Hiragana", "", "顧客が話した相手は", "公開組織は", []string{"あおい", "かえで", "さくら", "たくみ", "なお", "はる", "ひなた", "まこと", "ゆい", "りん"},
			[]string{"あらい", "いとう", "うえだ", "えんどう", "おおた", "かとう", "きむら", "こばやし", "さとう", "たなか"},
			[]string{"あさひ", "いずみ", "うみ", "えにし", "おおぞら", "かすみ", "きぼう", "こもれび", "さざなみ", "つばさ"},
			[]string{"けんきゅうじょ", "しゅっぱん", "ざいだん", "こうぼう", "がくえん", "ぶんかかい", "としょかん", "げきだん", "はくぶつかん", "きょうかい"}},
		{"ko", "Hangul", " ", "고객이 언급한 사람은 ", "공개 기관은 ", []string{"가람", "나래", "다온", "라온", "마루", "바다", "새봄", "아람", "이든", "하늘"},
			[]string{"김", "이", "박", "최", "정", "강", "조", "윤", "장", "임"},
			[]string{"가온", "나무", "다솜", "라움", "마중", "바람", "새길", "아침", "이음", "한빛"},
			[]string{"연구소", "출판사", "재단", "공방", "학교", "협회", "박물관", "도서관", "극단", "센터"}},
		{"hi", "Devanagari", " ", "ग्राहक ने उल्लेख किया ", "सार्वजनिक संस्था ", []string{"आरव", "इशान", "काव्या", "दीया", "नील", "प्रिया", "रवि", "सिया", "तनय", "वाणी"},
			[]string{"अरोड़ा", "बंसल", "चौहान", "देसाई", "गोयल", "कपूर", "मेहता", "नायर", "पटेल", "शर्मा"},
			[]string{"आकाश", "उदय", "किरण", "गंगा", "चेतना", "तरंग", "दिशा", "नव", "प्रकाश", "संगम"},
			[]string{"संस्थान", "प्रकाशन", "फाउंडेशन", "प्रयोगशाला", "विद्यालय", "संग्रहालय", "पुस्तकालय", "परिषद", "केंद्र", "मंच"}},
	}
}

func makeGeneratedCase(id, language, script, shape string, risk RiskTier, reason, text string, target Target, tags []string, secret *SecretFixtureRole) Case {
	fixture := Case{
		ID: id, Language: language, Script: script, Shape: shape, Risk: risk, Reason: reason, Text: text,
		KnownIdentities: []KnownIdentity{}, Targets: []Target{target}, Tags: tags, SecretFixture: secret,
	}
	for _, mode := range Modes {
		expected := OutputExpectation{RequiredAbsent: []string{}, RequiredPresent: []string{}}
		if target.Actions.For(mode) == ActionRedact {
			expected.RequiredAbsent = append(expected.RequiredAbsent, target.Value)
		} else {
			expected.RequiredPresent = append(expected.RequiredPresent, target.Value)
		}
		setOutputExpectation(&fixture.Outputs, mode, expected)
	}
	return fixture
}

func setOutputExpectation(outputs *ModeOutputs, mode Mode, expected OutputExpectation) {
	switch mode {
	case ModeOff:
		outputs.Off = expected
	case ModeCustomers:
		outputs.Customers = expected
	case ModeAll:
		outputs.All = expected
	}
}

func wrapFixtureShape(shape, context, value string, variation int) (string, int) {
	prefix, suffix := context, ""
	switch shape {
	case "json":
		prefix, suffix = fmt.Sprintf(`{"message_%d":"%s`, variation, context), `"}`
	case "yaml":
		prefix = fmt.Sprintf("message_%d: %s", variation, context)
	case "shell":
		prefix, suffix = fmt.Sprintf("MESSAGE_%d='%s", variation, context), "'"
	case "url":
		prefix = fmt.Sprintf("endpoint=https://192.0.2.10/%d ; %s", variation, context)
	case "markdown":
		prefix, suffix = "**"+context, "**"
	case "html":
		prefix, suffix = "<p>"+context, "</p>"
	case "quoted-reply":
		prefix = "> " + context
	case "log":
		prefix = fmt.Sprintf("level=warn sample=%d message=%q value=", variation, context)
	case "stack-trace":
		prefix = fmt.Sprintf("Trace[%d]: %s", variation, context)
	case "code":
		prefix, suffix = fmt.Sprintf("fixture_%d := `%s", variation, context), "`"
	case "signature":
		prefix = "--\n" + context
	case "long-thread":
		prefix = strings.Repeat("Earlier public context. ", 20) + context
	case "command-payload":
		prefix = fmt.Sprintf("--sample=%d --message=%q --value=", variation, context)
	}
	text := prefix + value + suffix
	return text, len(prefix)
}

func generatedPublicCode(namespace string, index, length int) string {
	provenance := SyntheticProvenance{Generator: SyntheticSecretGenerator, Version: 1, Seed: fmt.Sprintf("broad-%s-%03d", namespace, index), Recipe: "checksum", Purpose: "preserve"}
	return syntheticAlphabet(provenance, namespace, upperAlphaNumeric, length)
}

func readCorpusDocument(path string) (CorpusDocument, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CorpusDocument{}, fmt.Errorf("read generated corpus document: %w", err)
	}
	var document CorpusDocument
	if err := decodeStrict(raw, &document); err != nil {
		return CorpusDocument{}, fmt.Errorf("decode generated corpus document: %w", err)
	}
	return document, nil
}

func marshalCorpusDocument(document CorpusDocument) ([]byte, error) {
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode generated corpus partition %q: %w", document.Partition, err)
	}
	return append(raw, '\n'), nil
}

func marshalBroadCorpusDocument(document CorpusDocument) ([]byte, error) {
	partition, err := json.Marshal(document.Partition)
	if err != nil {
		return nil, fmt.Errorf("encode broad corpus partition name: %w", err)
	}
	var output bytes.Buffer
	fmt.Fprintf(&output, "{\n  \"schema\": %d,\n  \"partition\": %s,\n  \"cases\": [\n", document.Schema, partition)
	for index, fixture := range document.Cases {
		raw, marshalErr := json.Marshal(fixture)
		if marshalErr != nil {
			return nil, fmt.Errorf("encode broad corpus case %q: %w", fixture.ID, marshalErr)
		}
		output.WriteString("    ")
		output.Write(raw)
		if index+1 != len(document.Cases) {
			output.WriteByte(',')
		}
		output.WriteByte('\n')
	}
	output.WriteString("  ]\n}\n")
	return output.Bytes(), nil
}

func writeCorpusDocument(path string, document CorpusDocument) error {
	raw, err := marshalCorpusDocument(document)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write generated corpus partition %q: %w", document.Partition, err)
	}
	return nil
}
