package pii

import (
	"encoding/json"
	"strings"
	"testing"
)

// mockNER returns pre-configured name spans. Simulates NER detection.
type mockNER struct {
	spans []NameSpan
}

func (m *mockNER) DetectNames(string) ([]NameSpan, error) { return m.spans, nil }

// noNER returns no names — exercises known-identity path without NER interference.
func noNER() NameDetector { return &mockNER{} }

func nerWith(spans ...NameSpan) NameDetector { return &mockNER{spans: spans} }

func TestRedactPersonDeterministic(t *testing.T) {
	e := NewEngine(ModeAll, "")
	f1, l1, em1 := e.RedactPerson("Alice", "Smith", "alice@example.com")
	f2, l2, em2 := e.RedactPerson("Alice", "Smith", "alice@example.com")
	if f1 != f2 || l1 != l2 || em1 != em2 {
		t.Fatalf("expected deterministic person redaction")
	}
	if strings.EqualFold(em1, "alice@example.com") {
		t.Fatalf("email was not redacted")
	}
}

func TestRedactText_WithoutNER_ReturnsNotice(t *testing.T) {
	e := NewEngine(ModeAll, "")
	out := e.RedactText("Hello from Alice Smith", nil)
	if out != RedactTextNotice {
		t.Fatalf("expected notice, got: %q", out)
	}
}

func TestRedactText_WithoutNER_EmptyPassthrough(t *testing.T) {
	e := NewEngine(ModeAll, "")
	if out := e.RedactText("", nil); out != "" {
		t.Fatalf("expected empty, got: %q", out)
	}
}

func TestRedactText_Disabled_Passthrough(t *testing.T) {
	e := NewEngine(ModeOff, "")
	out := e.RedactText("Alice Smith", nil)
	if out != "Alice Smith" {
		t.Fatalf("expected passthrough when disabled, got: %q", out)
	}
}

func TestRedactTextUsesKnownIdentity(t *testing.T) {
	e := NewEngine(ModeCustomers, "", WithNER(noNER()))
	text := "Alice Smith wrote from alice@example.com"
	out := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})
	if strings.Contains(out, "Alice Smith") || strings.Contains(out, "alice@example.com") {
		t.Fatalf("known identity was not redacted: %q", out)
	}
}

func TestRedactText_KnownIdentity_UsesFakeName(t *testing.T) {
	e := NewEngine(ModeCustomers, "", WithNER(noNER()))

	fakeFirst, fakeLast, _ := e.RedactPerson("Alice", "Smith", "alice@example.com")

	text := "Alice Smith wrote about her account"
	out := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})

	if !strings.Contains(out, fakeFirst+" "+fakeLast) {
		t.Fatalf("expected fake full name %q in output, got: %q", fakeFirst+" "+fakeLast, out)
	}
}

func TestRedactText_KnownIdentity_ConsistentWithStructured(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))

	sFirst, sLast, _ := e.RedactPerson("Alice", "Smith", "alice@example.com")

	text := "Message from Alice Smith about her ticket"
	out := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})

	if !strings.Contains(out, sFirst+" "+sLast) {
		t.Fatalf("free-text fake name should match structured: want %q %q, got: %q", sFirst, sLast, out)
	}
}

func TestRedactText_NERDetectedName_UsesFakeName(t *testing.T) {
	d := nerWith(NameSpan{Text: "John Williams", Start: 8, End: 21, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))

	text := "Contact John Williams for details"
	out := e.RedactText(text, nil)

	if strings.Contains(out, "John Williams") {
		t.Fatalf("NER-detected name should be redacted: %q", out)
	}
	if out == RedactTextNotice {
		t.Fatalf("should not return notice when NER is available")
	}
}

func TestRedactText_NERDetectedName_Deterministic(t *testing.T) {
	d := nerWith(NameSpan{Text: "John Williams", Start: 8, End: 21, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))

	text := "Contact John Williams for details"
	out1 := e.RedactText(text, nil)
	out2 := e.RedactText(text, nil)

	if out1 != out2 {
		t.Fatalf("expected deterministic output:\n  %q\n  %q", out1, out2)
	}
}

func TestRedactText_NoDoubleRedaction(t *testing.T) {
	// NER also detects "Alice Smith", but known identity handles it first
	d := nerWith(NameSpan{Text: "Alice Smith", Start: 0, End: 11, Score: 0.92})
	e := NewEngine(ModeAll, "", WithNER(d))

	text := "Alice Smith sent a message"
	out1 := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})

	out2 := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})

	if out1 != out2 {
		t.Fatalf("expected stable output across calls:\n  %q\n  %q", out1, out2)
	}

	fakeFirst, fakeLast, _ := e.RedactPerson("Alice", "Smith", "alice@example.com")
	fakeFull := fakeFirst + " " + fakeLast
	count := strings.Count(out1, fakeFull)
	if count != 1 {
		t.Fatalf("expected fake name %q exactly once, found %d times in: %q", fakeFull, count, out1)
	}
}

func TestRedactText_KnownFirstNameOnly(t *testing.T) {
	e := NewEngine(ModeCustomers, "", WithNER(noNER()))

	text := "Hey Alice, here is your update"
	out := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})

	if strings.Contains(out, "Alice") {
		t.Fatalf("first name should be redacted: %q", out)
	}
	fakeFirst, _, _ := e.RedactPerson("Alice", "Smith", "alice@example.com")
	if !strings.Contains(out, fakeFirst) {
		t.Fatalf("expected fake first name %q in output: %q", fakeFirst, out)
	}
}

func TestRedactText_EmailOnlyCustomer_PrefixRedacted(t *testing.T) {
	// NER detects "Marco Rossi" in the text
	d := nerWith(NameSpan{Text: "Marco Rossi", Start: 1, End: 12, Score: 0.90})
	e := NewEngine(ModeCustomers, "", WithNER(d))

	text := "[Marco Rossi] New Form Submission"
	out := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		Email: "marco@testdomain.com",
	}})

	if strings.Contains(out, "Marco") || strings.Contains(out, "marco") {
		t.Fatalf("email prefix should be redacted: %q", out)
	}
}

func TestRedactText_EmailInText_StaysRedacted(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))

	text := "Contact alice@example.com for help"
	out := e.RedactText(text, []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})

	if strings.Contains(out, "alice@example.com") {
		t.Fatalf("email should be redacted: %q", out)
	}
}

func TestEngineMode(t *testing.T) {
	tests := []struct {
		mode, want string
	}{
		{"all", ModeAll},
		{"customers", ModeCustomers},
		{"off", ModeOff},
		{"", ModeOff},
		{"unknown", ModeOff},
	}
	for _, tt := range tests {
		e := NewEngine(tt.mode, "")
		if got := e.Mode(); got != tt.want {
			t.Fatalf("NewEngine(%q).Mode() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestEngineEnabled(t *testing.T) {
	if NewEngine(ModeOff, "").Enabled() {
		t.Fatal("off engine should not be enabled")
	}
	if !NewEngine(ModeAll, "").Enabled() {
		t.Fatal("all engine should be enabled")
	}
	if !NewEngine(ModeCustomers, "").Enabled() {
		t.Fatal("customers engine should be enabled")
	}
}

func TestEngineShouldRedactType(t *testing.T) {
	e := NewEngine(ModeCustomers, "")
	if !e.ShouldRedactType("customer") {
		t.Fatal("customers mode should redact customer type")
	}
	if e.ShouldRedactType("user") {
		t.Fatal("customers mode should not redact user type")
	}
}

func TestRedactPerson_EmptyInputs(t *testing.T) {
	e := NewEngine(ModeAll, "")
	// All empty → passthrough
	f, l, em := e.RedactPerson("", "", "")
	if f != "" || l != "" || em != "" {
		t.Fatalf("expected empty passthrough, got %q %q %q", f, l, em)
	}
}

func TestRedactPerson_EmailOnly(t *testing.T) {
	e := NewEngine(ModeAll, "")
	f, l, em := e.RedactPerson("", "", "test@example.com")
	if f != "" || l != "" {
		t.Fatalf("expected empty name for email-only, got %q %q", f, l)
	}
	if em == "test@example.com" {
		t.Fatal("email should be redacted")
	}
}

func TestRedactPerson_NameOnly(t *testing.T) {
	e := NewEngine(ModeAll, "")
	f, l, em := e.RedactPerson("Alice", "Smith", "")
	if f == "Alice" || l == "Smith" {
		t.Fatalf("name should be redacted, got %q %q", f, l)
	}
	if em != "" {
		t.Fatalf("empty email should stay empty, got %q", em)
	}
}

func TestRedactPerson_SecretChangesOutput(t *testing.T) {
	e1 := NewEngine(ModeAll, "secret1")
	e2 := NewEngine(ModeAll, "secret2")
	_, _, em1 := e1.RedactPerson("Alice", "Smith", "alice@example.com")
	_, _, em2 := e2.RedactPerson("Alice", "Smith", "alice@example.com")
	if em1 == em2 {
		t.Fatal("different secrets should produce different redacted emails")
	}
}

func TestRedactEmail_Standalone(t *testing.T) {
	e := NewEngine(ModeAll, "")
	out := e.RedactEmail("test@example.com")
	if out == "test@example.com" {
		t.Fatal("email should be redacted")
	}
	if !strings.Contains(out, "@") {
		t.Fatalf("redacted email should still contain @: %q", out)
	}
}

func TestRedactEmail_Empty(t *testing.T) {
	e := NewEngine(ModeAll, "")
	if out := e.RedactEmail(""); out != "" {
		t.Fatalf("expected empty passthrough, got %q", out)
	}
	if out := e.RedactEmail("  "); out != "  " {
		t.Fatalf("expected whitespace passthrough, got %q", out)
	}
}

func TestRedactEmail_Deterministic(t *testing.T) {
	e := NewEngine(ModeAll, "")
	a := e.RedactEmail("test@example.com")
	b := e.RedactEmail("test@example.com")
	if a != b {
		t.Fatalf("redacted email should be deterministic: %q vs %q", a, b)
	}
}

func TestRedactPhone_Standalone(t *testing.T) {
	e := NewEngine(ModeAll, "")
	out := e.RedactPhone("+1 (555) 123-4567")
	if out == "+1 (555) 123-4567" {
		t.Fatal("phone should be redacted")
	}
	// Format should be preserved (parens, dashes, spaces)
	if !strings.Contains(out, "(") || !strings.Contains(out, ")") || !strings.Contains(out, "-") {
		t.Fatalf("phone format should be preserved: %q", out)
	}
}

func TestRedactPhone_Empty(t *testing.T) {
	e := NewEngine(ModeAll, "")
	if out := e.RedactPhone(""); out != "" {
		t.Fatalf("expected empty passthrough, got %q", out)
	}
}

func TestRedactPhone_Deterministic(t *testing.T) {
	e := NewEngine(ModeAll, "")
	a := e.RedactPhone("555-123-4567")
	b := e.RedactPhone("555-123-4567")
	if a != b {
		t.Fatalf("redacted phone should be deterministic: %q vs %q", a, b)
	}
}

func TestRedactPhone_DigitsOnly(t *testing.T) {
	e := NewEngine(ModeAll, "")
	out := e.RedactPhone("5551234567")
	if out == "5551234567" {
		t.Fatal("phone should be redacted")
	}
	if len(out) != 10 {
		t.Fatalf("expected 10-digit output, got %q (len %d)", out, len(out))
	}
}

func TestWithNER_AttachesDetector(t *testing.T) {
	d := noNER()
	e := NewEngine(ModeAll, "", WithNER(d))
	// Should not return notice when NER is attached
	out := e.RedactText("hello world", nil)
	if out == RedactTextNotice {
		t.Fatal("should not return notice when NER is attached")
	}
}

func TestRedactJSON(t *testing.T) {
	e := NewEngine(ModeAll, "")
	input := json.RawMessage(`{
		"subject":"Email from Alice Smith",
		"primaryCustomer":{"type":"customer","first":"Alice","last":"Smith","email":"alice@example.com"},
		"assignee":{"type":"user","first":"Ross","last":"M","email":"ross@example.com"},
		"preview":"Contact me at alice@example.com"
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("RedactJSON error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "alice@example.com") || strings.Contains(s, "ross@example.com") {
		t.Fatalf("expected redacted JSON, got %s", s)
	}
}

func TestRedactJSON_WithoutNER_FreeformIsNotice(t *testing.T) {
	e := NewEngine(ModeAll, "")
	input := json.RawMessage(`{
		"subject":"Email from Alice Smith",
		"primaryCustomer":{"type":"customer","first":"Alice","last":"Smith","email":"alice@example.com"}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("RedactJSON error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "hs pii-model install") {
		t.Fatalf("expected freeform text to contain pii-model install notice, got %s", s)
	}
	// Structured fields should still be redacted
	if strings.Contains(s, "alice@example.com") {
		t.Fatalf("structured email should still be redacted: %s", s)
	}
}

func TestRedactJSON_WithNER_FreeformRedacted(t *testing.T) {
	d := nerWith(NameSpan{Text: "Alice Smith", Start: 11, End: 22, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))
	input := json.RawMessage(`{
		"subject":"Email from Alice Smith",
		"primaryCustomer":{"type":"customer","first":"Alice","last":"Smith","email":"alice@example.com"}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("RedactJSON error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, RedactTextNotice) {
		t.Fatalf("should not contain notice when NER is present: %s", s)
	}
	if strings.Contains(s, "Alice Smith") {
		t.Fatalf("name should be redacted: %s", s)
	}
}
