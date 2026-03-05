package pii

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactJSON_Disabled(t *testing.T) {
	e := NewEngine(ModeOff, "")
	input := json.RawMessage(`{"subject":"Hello Alice","email":"alice@test.com"}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("RedactJSON error: %v", err)
	}
	if string(out) != string(input) {
		t.Fatalf("disabled mode should passthrough, got %s", out)
	}
}

func TestRedactJSON_EmptyInput(t *testing.T) {
	e := NewEngine(ModeAll, "")
	out, err := e.RedactJSON(nil)
	if err != nil {
		t.Fatalf("RedactJSON error: %v", err)
	}
	if out != nil {
		t.Fatalf("nil input should return nil, got %s", out)
	}
}

func TestRedactJSON_InvalidJSON(t *testing.T) {
	e := NewEngine(ModeAll, "")
	_, err := e.RedactJSON(json.RawMessage(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRedactJSON_StructuredCustomerFields(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"primaryCustomer": {
			"type": "customer",
			"first": "Alice",
			"last": "Smith",
			"email": "alice@test.com"
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "Alice") || strings.Contains(s, "Smith") || strings.Contains(s, "alice@test.com") {
		t.Fatalf("structured customer fields should be redacted: %s", s)
	}
}

func TestRedactJSON_StructuredUserFields(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"assignee": {
			"type": "user",
			"first": "Ross",
			"last": "M",
			"email": "ross@company.com"
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "ross@company.com") {
		t.Fatalf("user email should be redacted in all mode: %s", s)
	}
}

func TestRedactJSON_CustomersMode_SkipsUser(t *testing.T) {
	e := NewEngine(ModeCustomers, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"assignee": {
			"type": "user",
			"first": "Ross",
			"last": "M",
			"email": "ross@company.com"
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	// User fields should NOT be redacted in customers mode
	if !strings.Contains(s, "ross@company.com") {
		t.Fatalf("user email should not be redacted in customers mode: %s", s)
	}
}

func TestRedactJSON_FirstNameLastNameVariant(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"customer": {
			"type": "customer",
			"firstName": "Alice",
			"lastName": "Smith",
			"email": "alice@test.com",
			"phone": "555-123-4567"
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "Alice") || strings.Contains(s, "alice@test.com") || strings.Contains(s, "555-123-4567") {
		t.Fatalf("firstName/lastName/email/phone should be redacted: %s", s)
	}
}

func TestRedactJSON_SentinelPersonSkipped(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	// Sentinel: id=0 should be skipped
	input := json.RawMessage(`{
		"assignee": {
			"id": 0,
			"type": "user",
			"first": "Unassigned",
			"last": "",
			"email": "unknown"
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	// Sentinel person should not be redacted
	if !strings.Contains(s, "Unassigned") {
		t.Fatalf("sentinel person should not be redacted: %s", s)
	}
}

func TestRedactJSON_EmailsArray(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"customer": {
			"type": "customer",
			"first": "Alice",
			"last": "Smith",
			"emails": [{"value": "alice@test.com"}, {"value": "alice2@test.com"}]
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "alice@test.com") || strings.Contains(s, "alice2@test.com") {
		t.Fatalf("emails array should be redacted: %s", s)
	}
}

func TestRedactJSON_PhonesArray(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"customer": {
			"type": "customer",
			"first": "Alice",
			"last": "Smith",
			"phones": [{"value": "555-123-4567"}]
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "555-123-4567") {
		t.Fatalf("phones array should be redacted: %s", s)
	}
}

func TestRedactJSON_FreeformSubject(t *testing.T) {
	d := nerWith(NameSpan{Text: "Alice Smith", Start: 11, End: 22, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))
	input := json.RawMessage(`{
		"subject": "Email from Alice Smith",
		"primaryCustomer": {"type":"customer","first":"Alice","last":"Smith","email":"alice@test.com"}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "Alice Smith") {
		t.Fatalf("subject name should be redacted: %s", s)
	}
}

func TestRedactJSON_NestedObjects(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"conversation": {
			"threads": [{
				"customer": {
					"type": "customer",
					"first": "Alice",
					"last": "Smith",
					"email": "alice@test.com"
				},
				"body": "Hello"
			}]
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "alice@test.com") {
		t.Fatalf("deeply nested email should be redacted: %s", s)
	}
}

func TestRedactJSON_ToCcBccArrays(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"to": ["alice@test.com", "bob@test.com"],
		"cc": ["carol@test.com"],
		"bcc": ["dave@test.com"]
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	for _, email := range []string{"alice@test.com", "bob@test.com", "carol@test.com", "dave@test.com"} {
		if strings.Contains(s, email) {
			t.Fatalf("%s should be redacted: %s", email, s)
		}
	}
}

func TestRedactJSON_NonRedactableFieldPassthrough(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"id": 12345,
		"status": "active",
		"tags": ["billing", "urgent"],
		"number": 42
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "active") || !strings.Contains(s, "billing") {
		t.Fatalf("non-redactable fields should pass through: %s", s)
	}
}

func TestRedactJSON_InferEntityType_FromKey(t *testing.T) {
	e := NewEngine(ModeCustomers, "", WithNER(noNER()))
	// No explicit "type" field — should infer from key "primaryCustomer"
	input := json.RawMessage(`{
		"primaryCustomer": {
			"first": "Alice",
			"last": "Smith",
			"email": "alice@test.com"
		}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "alice@test.com") {
		t.Fatalf("customer inferred from key should be redacted: %s", s)
	}
}

func TestRedactJSON_Deterministic(t *testing.T) {
	e := NewEngine(ModeAll, "", WithNER(noNER()))
	input := json.RawMessage(`{
		"primaryCustomer": {"type":"customer","first":"Alice","last":"Smith","email":"alice@test.com"}
	}`)
	out1, _ := e.RedactJSON(input)
	// Re-parse since walkMap mutates in place
	input2 := json.RawMessage(`{
		"primaryCustomer": {"type":"customer","first":"Alice","last":"Smith","email":"alice@test.com"}
	}`)
	out2, _ := e.RedactJSON(input2)
	if string(out1) != string(out2) {
		t.Fatalf("RedactJSON should be deterministic:\n  %s\n  %s", out1, out2)
	}
}

func TestRedactJSON_StructuredAndFreeformTogether(t *testing.T) {
	// NER detects name in subject; structured fields also redacted
	d := nerWith(NameSpan{Text: "Alice Smith", Start: 0, End: 11, Score: 0.95})
	e := NewEngine(ModeAll, "", WithNER(d))
	input := json.RawMessage(`{
		"subject": "Alice Smith needs help",
		"primaryCustomer": {"type":"customer","first":"Alice","last":"Smith","email":"alice@test.com"}
	}`)
	out, err := e.RedactJSON(input)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "alice@test.com") {
		t.Fatalf("structured email should be redacted: %s", s)
	}
	// Subject name redacted via NER
	if strings.Contains(s, "Alice Smith") {
		t.Fatalf("subject name should be redacted via NER: %s", s)
	}
}

// --- helper function unit tests ---

func TestShouldRedactTextField(t *testing.T) {
	redactable := []string{"subject", "preview", "body", "text", "raw", "source", "content", "message", "snippet", "html"}
	for _, key := range redactable {
		if !shouldRedactTextField(key) {
			t.Fatalf("shouldRedactTextField(%q) = false, want true", key)
		}
	}
	nonRedactable := []string{"id", "status", "type", "number", "tags", "href", "first", "last", "email"}
	for _, key := range nonRedactable {
		if shouldRedactTextField(key) {
			t.Fatalf("shouldRedactTextField(%q) = true, want false", key)
		}
	}
}

func TestInferEntityType_FromMap(t *testing.T) {
	m := map[string]any{"type": "customer", "first": "Alice"}
	if got := inferEntityType(m, "person", ""); got != "customer" {
		t.Fatalf("expected customer from map type, got %q", got)
	}
}

func TestInferEntityType_FromKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"primaryCustomer", "customer"},
		{"assignee", "user"},
		{"assignedTo", "user"},
		{"owner", "user"},
		{"member", "user"},
	}
	for _, tt := range tests {
		if got := inferEntityType(nil, tt.key, ""); got != tt.want {
			t.Fatalf("inferEntityType(nil, %q, \"\") = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestInferEntityType_FallsBackToParentHint(t *testing.T) {
	if got := inferEntityType(nil, "unknown", "customer"); got != "customer" {
		t.Fatalf("expected parent hint fallback, got %q", got)
	}
}

func TestKnownIdentityFromMap_FirstLast(t *testing.T) {
	m := map[string]any{"first": "Alice", "last": "Smith", "email": "alice@test.com"}
	id, ok := knownIdentityFromMap(m, "customer")
	if !ok {
		t.Fatal("expected identity from first/last map")
	}
	if id.First != "Alice" || id.Last != "Smith" || id.Email != "alice@test.com" {
		t.Fatalf("unexpected identity: %+v", id)
	}
}

func TestKnownIdentityFromMap_FirstNameLastName(t *testing.T) {
	m := map[string]any{"firstName": "Alice", "lastName": "Smith"}
	id, ok := knownIdentityFromMap(m, "")
	if !ok {
		t.Fatal("expected identity from firstName/lastName map")
	}
	if id.First != "Alice" || id.Last != "Smith" {
		t.Fatalf("unexpected identity: %+v", id)
	}
	if id.Type != "customer" {
		t.Fatalf("expected default type 'customer', got %q", id.Type)
	}
}

func TestKnownIdentityFromMap_PhoneOnly(t *testing.T) {
	m := map[string]any{"phone": "555-1234"}
	id, ok := knownIdentityFromMap(m, "customer")
	if !ok {
		t.Fatal("expected identity from phone-only map")
	}
	if id.Phone != "555-1234" {
		t.Fatalf("unexpected phone: %q", id.Phone)
	}
}

func TestKnownIdentityFromMap_Empty(t *testing.T) {
	m := map[string]any{"id": float64(42), "status": "active"}
	_, ok := knownIdentityFromMap(m, "customer")
	if ok {
		t.Fatal("expected no identity from non-person map")
	}
}

func TestKnownIdentityFromMap_Sentinel(t *testing.T) {
	m := map[string]any{"id": float64(0), "first": "Unassigned", "email": "unknown"}
	_, ok := knownIdentityFromMap(m, "customer")
	if ok {
		t.Fatal("sentinel map should not produce identity")
	}
}

func TestIsSentinelPersonMap(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want bool
	}{
		{"id=0", map[string]any{"id": float64(0)}, true},
		{"email=unknown", map[string]any{"email": "unknown"}, true},
		{"email=Unknown", map[string]any{"email": "Unknown"}, true},
		{"normal", map[string]any{"id": float64(42), "email": "a@b.com"}, false},
		{"empty", map[string]any{}, false},
	}
	for _, tt := range tests {
		if got := isSentinelPersonMap(tt.m); got != tt.want {
			t.Fatalf("isSentinelPersonMap(%s) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
