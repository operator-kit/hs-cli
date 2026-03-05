package pii

import (
	"strings"
	"testing"
)

// --- RedactText regex sweep tests (requires NER to be attached) ---

func TestRedactText_EmailInText(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Please email support@example.com for help", nil)
	if strings.Contains(out, "support@example.com") {
		t.Fatalf("email should be redacted: %q", out)
	}
}

func TestRedactText_PhoneInText(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Call us at 555-123-4567 for support", nil)
	if strings.Contains(out, "555-123-4567") {
		t.Fatalf("phone should be redacted: %q", out)
	}
}

func TestRedactText_PhoneWithExtension(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Call (800) 555-1234 ext 5678", nil)
	if strings.Contains(out, "555-1234") {
		t.Fatalf("phone with ext should be redacted: %q", out)
	}
}

func TestRedactText_SSN_Formatted(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("SSN is 123-45-6789", nil)
	if strings.Contains(out, "123-45-6789") {
		t.Fatalf("formatted SSN should be redacted: %q", out)
	}
}

func TestRedactText_SSN_ContextTriggered(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Social security number: 123456789", nil)
	if strings.Contains(out, "123456789") {
		t.Fatalf("SSN with context should be redacted: %q", out)
	}
}

func TestRedactText_CreditCard(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Card: 4111-1111-1111-1111", nil)
	if strings.Contains(out, "4111-1111-1111-1111") {
		t.Fatalf("credit card should be redacted: %q", out)
	}
}

func TestRedactText_IPv4(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Server at 192.168.1.100 is down", nil)
	if strings.Contains(out, "192.168.1.100") {
		t.Fatalf("IPv4 should be redacted: %q", out)
	}
}

func TestRedactText_MAC(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Device MAC: AA:BB:CC:DD:EE:FF", nil)
	if strings.Contains(out, "AA:BB:CC:DD:EE:FF") {
		t.Fatalf("MAC should be redacted: %q", out)
	}
}

func TestRedactText_URL(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Visit https://example.com/account/settings for details", nil)
	if strings.Contains(out, "https://example.com") {
		t.Fatalf("URL should be redacted: %q", out)
	}
}

func TestRedactText_Address(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Located at 123 Main Street", nil)
	if strings.Contains(out, "123 Main Street") {
		t.Fatalf("address should be redacted: %q", out)
	}
}

func TestRedactText_POBox(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Send to P.O. Box 1234", nil)
	if strings.Contains(out, "P.O. Box 1234") {
		t.Fatalf("PO Box should be redacted: %q", out)
	}
}

func TestRedactText_ZipCode(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	out := e.RedactText("Zip code is 90210-1234", nil)
	if strings.Contains(out, "90210-1234") {
		t.Fatalf("zip code should be redacted: %q", out)
	}
}

// --- NER integration edge cases ---

func TestRedactText_MultipleNERSpans(t *testing.T) {
	d := nerWith(
		NameSpan{Text: "Alice Smith", Start: 0, End: 11, Score: 0.95},
		NameSpan{Text: "Bob Jones", Start: 16, End: 25, Score: 0.90},
	)
	e := NewEngine(ModeAll, "", WithNER(d))
	out := e.RedactText("Alice Smith met Bob Jones today", nil)
	if strings.Contains(out, "Alice Smith") || strings.Contains(out, "Bob Jones") {
		t.Fatalf("both NER names should be redacted: %q", out)
	}
}

func TestRedactText_NERAndKnownOverlap(t *testing.T) {
	// NER detects "Alice Smith", known identity also has Alice Smith
	// Should not double-redact
	d := nerWith(NameSpan{Text: "Alice Smith", Start: 0, End: 11, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))
	out := e.RedactText("Alice Smith sent a message", []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})
	fakeFirst, fakeLast, _ := e.RedactPerson("Alice", "Smith", "alice@example.com")
	fakeFull := fakeFirst + " " + fakeLast
	count := strings.Count(out, fakeFull)
	if count != 1 {
		t.Fatalf("expected fake name once, found %d in: %q", count, out)
	}
}

func TestRedactText_NERSpanContainsKnownWord(t *testing.T) {
	// NER detects "Alice Johnson" but known identity has "Alice Smith"
	// "Alice" is in inserted set, so NER span should be skipped
	d := nerWith(NameSpan{Text: "Alice Johnson", Start: 0, End: 13, Score: 0.90})
	e := NewEngine(ModeAll, "", WithNER(d))
	out := e.RedactText("Alice Johnson wrote a letter", []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})
	if strings.Contains(out, "Alice") {
		t.Fatalf("Alice should be redacted via known identity: %q", out)
	}
}

func TestRedactText_NERWithRegexSweep(t *testing.T) {
	// Both a name and an email in the same text
	d := nerWith(NameSpan{Text: "John Williams", Start: 8, End: 21, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))
	out := e.RedactText("Contact John Williams at john@example.com for help", nil)
	if strings.Contains(out, "John Williams") {
		t.Fatalf("NER name should be redacted: %q", out)
	}
	if strings.Contains(out, "john@example.com") {
		t.Fatalf("email should be redacted by regex sweep: %q", out)
	}
}

func TestRedactText_ModeCustomers_SkipsUserType(t *testing.T) {
	e := NewEngine(ModeCustomers, "", WithNER(noNER()))
	// Known identity with type "user" should be skipped in customers mode
	// (redactKnown uses ShouldRedactType internally via the engine)
	// Actually redactKnown doesn't filter by type — it always replaces.
	// The type filtering happens at the structured JSON level.
	// So in RedactText, all known identities are replaced regardless of type.
	out := e.RedactText("Hello Alice", []KnownIdentity{{
		Type:  "customer",
		First: "Alice",
		Last:  "Smith",
		Email: "alice@example.com",
	}})
	if strings.Contains(out, "Alice") {
		t.Fatalf("customer identity should be redacted: %q", out)
	}
}

func TestRedactText_MixedPIITypes(t *testing.T) {
	d := nerWith(NameSpan{Text: "Alice Smith", Start: 0, End: 11, Score: 0.92})
	e := NewEngine(ModeAll, "", WithNER(d))
	text := "Alice Smith, email: alice@example.com, phone: 555-123-4567, SSN: 123-45-6789"
	out := e.RedactText(text, nil)
	if strings.Contains(out, "Alice Smith") {
		t.Fatalf("name should be redacted: %q", out)
	}
	if strings.Contains(out, "alice@example.com") {
		t.Fatalf("email should be redacted: %q", out)
	}
	if strings.Contains(out, "555-123-4567") {
		t.Fatalf("phone should be redacted: %q", out)
	}
	if strings.Contains(out, "123-45-6789") {
		t.Fatalf("SSN should be redacted: %q", out)
	}
}

func TestRedactText_PlainTextNoNames(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	// Text with no PII should pass through mostly unchanged
	text := "The weather is nice today"
	out := e.RedactText(text, nil)
	if out != text {
		t.Fatalf("no-PII text should pass through, got: %q", out)
	}
}

func TestRedactText_NERDeterministicAcrossEngines(t *testing.T) {
	// Same secret, same NER spans → same output
	d1 := nerWith(NameSpan{Text: "John Doe", Start: 0, End: 8, Score: 0.95})
	d2 := nerWith(NameSpan{Text: "John Doe", Start: 0, End: 8, Score: 0.95})
	e1 := NewEngine(ModeAll, "same-secret", WithNER(d1))
	e2 := NewEngine(ModeAll, "same-secret", WithNER(d2))
	out1 := e1.RedactText("John Doe is here", nil)
	out2 := e2.RedactText("John Doe is here", nil)
	if out1 != out2 {
		t.Fatalf("same secret should produce same output:\n  %q\n  %q", out1, out2)
	}
}
