package pii

import (
	"encoding/json"
	"strings"
)

// ResourceKind describes the API resource being presented. It lets the JSON
// walker make narrow, path-aware decisions without coupling the PII package to
// command or Help Scout response types.
type ResourceKind string

const (
	ResourceGeneric      ResourceKind = "generic"
	ResourceCustomer     ResourceKind = "customer"
	ResourceUser         ResourceKind = "user"
	ResourceConversation ResourceKind = "conversation"
	ResourceRating       ResourceKind = "rating"
	ResourceReport       ResourceKind = "report"
	ResourceAttachment   ResourceKind = "attachment"
	ResourceDiagnostic   ResourceKind = "diagnostic"
)

// JSONContext supplies facts known by the caller but not always encoded in a
// payload. RootEntity is normally "customer" or "user".
type JSONContext struct {
	RootEntity string
	Resource   ResourceKind
}

// RedactedOpaqueData is emitted in place of attachment or diagnostic payloads
// whose contents cannot safely be inspected as text.
const RedactedOpaqueData = "[redacted opaque data]"

func (e *Engine) RedactJSON(data json.RawMessage) (json.RawMessage, error) {
	return e.RedactJSONWithContext(data, JSONContext{})
}

// RedactJSONWithContext redacts a JSON payload using explicit resource and
// entity context supplied by the presentation boundary.
func (e *Engine) RedactJSONWithContext(data json.RawMessage, ctx JSONContext) (json.RawMessage, error) {
	if !e.Enabled() || len(data) == 0 {
		return data, nil
	}

	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	redacted := e.walkAny(v, "", nil, nil, normalizeEntityType(ctx.RootEntity), ctx)
	out, err := json.Marshal(redacted)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (e *Engine) walkAny(v any, parentKey string, path []string, known []KnownIdentity, hintType string, ctx JSONContext) any {
	switch x := v.(type) {
	case map[string]any:
		return e.walkMap(x, parentKey, path, known, hintType, ctx)
	case []any:
		return e.walkSlice(x, parentKey, path, known, hintType, ctx)
	case string:
		return e.redactStringByPath(path, parentKey, x, known, hintType, ctx)
	default:
		return v
	}
}

func (e *Engine) walkMap(m map[string]any, parentKey string, path []string, inherited []KnownIdentity, hintType string, ctx JSONContext) map[string]any {
	// Classification happens before identity extraction. Identity fields must
	// never be responsible for deciding whether policy applies to the object.
	entityType := inferEntityType(m, parentKey, hintType)
	known := append([]KnownIdentity(nil), inherited...)
	if id, ok := knownIdentityFromMap(m, entityType); ok {
		known = append(known, id)
	}

	e.redactStructuredMap(m, entityType)

	for k, v := range m {
		childPath := appendPath(path, k)
		childHint := inferEntityType(nil, k, entityType)
		m[k] = e.walkAny(v, k, childPath, known, childHint, ctx)
	}
	return m
}

func (e *Engine) walkSlice(items []any, parentKey string, path []string, known []KnownIdentity, hintType string, ctx JSONContext) []any {
	for i, v := range items {
		switch x := v.(type) {
		case string:
			items[i] = e.redactSliceString(parentKey, path, x, known, hintType, ctx)
		default:
			items[i] = e.walkAny(v, parentKey, path, known, hintType, ctx)
		}
	}
	return items
}

func (e *Engine) redactSliceString(parentKey string, path []string, value string, known []KnownIdentity, hintType string, ctx JSONContext) string {
	switch normalizedKey(parentKey) {
	case "to", "cc", "bcc", "emails":
		return e.RedactEmail(value)
	case "phones":
		return e.RedactPhone(value)
	default:
		return e.redactStringByPath(path, parentKey, value, known, hintType, ctx)
	}
}

func (e *Engine) redactStringByPath(path []string, key, value string, known []KnownIdentity, hintType string, ctx JSONContext) string {
	_ = hintType
	key = normalizedKey(key)

	if key == "data" && (ctx.Resource == ResourceAttachment || ctx.Resource == ResourceDiagnostic || pathContains(path, "attachments")) {
		return RedactedOpaqueData
	}
	if key == "value" {
		switch {
		case pathContains(path, "emails"):
			return e.RedactEmail(value)
		case pathContains(path, "phones"):
			return e.RedactPhone(value)
		case pathContains(path, "customfields") || pathContains(path, "custom_fields") || pathContains(path, "fields"):
			return e.RedactText(value, known)
		}
	}
	if shouldRedactTextField(key) {
		return e.RedactText(value, known)
	}
	return value
}

func shouldRedactTextField(key string) bool {
	switch normalizedKey(key) {
	case "subject", "preview", "body", "text", "raw", "source", "content", "message", "snippet", "html", "customer", "assignee", "from", "assignedto", "created_by", "action", "comments", "comment", "filename", "background", "location":
		return true
	default:
		return false
	}
}

// inferEntityType accepts only policy-relevant values from a payload's type
// field. Values such as "work" on email/phone records must not erase an
// inherited customer context.
func inferEntityType(m map[string]any, key string, parentHint string) string {
	if m != nil {
		if t, ok := m["type"].(string); ok {
			if normalized := normalizeEntityType(t); normalized != "" {
				return normalized
			}
		}
	}

	k := normalizedKey(key)
	switch {
	case strings.Contains(k, "customer"):
		return "customer"
	case strings.Contains(k, "assignee"), strings.Contains(k, "assigned"), strings.Contains(k, "user"), strings.Contains(k, "owner"), strings.Contains(k, "member"):
		return "user"
	}
	if parent := normalizeEntityType(parentHint); parent != "" {
		return parent
	}

	// Help Scout's otherwise-unclassified firstName/lastName shape defaults to
	// customer. Explicit keys and caller context above always take precedence.
	if m != nil && hasAnyKey(m, "firstName", "lastName", "phone") {
		return "customer"
	}
	return ""
}

func knownIdentityFromMap(m map[string]any, entityType string) (KnownIdentity, bool) {
	if isSentinelPersonMap(m) {
		return KnownIdentity{}, false
	}

	if hasAnyKey(m, "first", "last") {
		first, _ := m["first"].(string)
		last, _ := m["last"].(string)
		return KnownIdentity{
			Type:  entityType,
			First: first,
			Last:  last,
			Email: getMapString(m, "email"),
			Phone: getMapString(m, "phone"),
		}, true
	}

	if hasAnyKey(m, "firstName", "lastName", "phone") {
		return KnownIdentity{
			Type:  defaultIfEmpty(entityType, "customer"),
			First: getMapString(m, "firstName"),
			Last:  getMapString(m, "lastName"),
			Email: getMapString(m, "email"),
			Phone: getMapString(m, "phone"),
		}, true
	}

	if email := getMapString(m, "email"); email != "" {
		return KnownIdentity{Type: entityType, Email: email}, true
	}
	return KnownIdentity{}, false
}

func (e *Engine) redactStructuredMap(m map[string]any, entityType string) {
	if !e.ShouldRedactType(entityType) || isSentinelPersonMap(m) {
		return
	}

	if hasAnyKey(m, "first", "last") {
		first := getMapString(m, "first")
		last := getMapString(m, "last")
		email := getMapString(m, "email")
		rf, rl, re := e.RedactPerson(first, last, email)
		if first != "" || last != "" {
			m["first"] = rf
			m["last"] = rl
		}
		if email != "" {
			m["email"] = re
		}
		if phone := getMapString(m, "phone"); phone != "" {
			m["phone"] = e.RedactPhone(phone)
		}
	}

	if hasAnyKey(m, "firstName", "lastName", "email", "phone") {
		first := getMapString(m, "firstName")
		last := getMapString(m, "lastName")
		email := getMapString(m, "email")
		rf, rl, re := e.RedactPerson(first, last, email)
		if first != "" || last != "" {
			m["firstName"] = rf
			m["lastName"] = rl
		}
		if email != "" {
			m["email"] = re
		}
		if phone := getMapString(m, "phone"); phone != "" {
			m["phone"] = e.RedactPhone(phone)
		}
	}
}

func isSentinelPersonMap(m map[string]any) bool {
	if id, ok := m["id"].(float64); ok && id == 0 {
		return true
	}
	if email, ok := m["email"].(string); ok && strings.EqualFold(email, "unknown") {
		return true
	}
	return false
}

func getMapString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func hasAnyKey(m map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}

func normalizeEntityType(value string) string {
	switch normalizedKey(value) {
	case "customer":
		return "customer"
	case "user":
		return "user"
	default:
		return ""
	}
}

func normalizedKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func appendPath(path []string, key string) []string {
	next := make([]string, len(path)+1)
	copy(next, path)
	next[len(path)] = normalizedKey(key)
	return next
}

func pathContains(path []string, key string) bool {
	key = normalizedKey(key)
	for _, part := range path {
		if normalizedKey(part) == key {
			return true
		}
	}
	return false
}

func defaultIfEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
