package pii

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestRedactTextUsesKnownIdentity(t *testing.T) {
	e := NewEngine(ModeCustomers, "")
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

