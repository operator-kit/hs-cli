package pii

import (
	"encoding/json"
	"strings"
)

func (e *Engine) RedactJSON(data json.RawMessage) (json.RawMessage, error) {
	if !e.Enabled() || len(data) == 0 {
		return data, nil
	}

	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}

	redacted := e.walkAny(v, "", nil, "")
	out, err := json.Marshal(redacted)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (e *Engine) walkAny(v any, parentKey string, known []KnownIdentity, hintType string) any {
	switch x := v.(type) {
	case map[string]any:
		return e.walkMap(x, parentKey, known, hintType)
	case []any:
		return e.walkSlice(x, parentKey, known, hintType)
	case string:
		return e.redactStringByKey(parentKey, x, known, hintType)
	default:
		return v
	}
}

func (e *Engine) walkMap(m map[string]any, parentKey string, inherited []KnownIdentity, hintType string) map[string]any {
	entityType := inferEntityType(m, parentKey, hintType)
	known := make([]KnownIdentity, len(inherited))
	copy(known, inherited)

	if id, ok := knownIdentityFromMap(m, entityType); ok {
		known = append(known, id)
	}

	e.redactStructuredMap(m, entityType)

	for k, v := range m {
		childHint := inferEntityType(nil, k, entityType)
		m[k] = e.walkAny(v, k, known, childHint)
	}
	return m
}

func (e *Engine) walkSlice(items []any, parentKey string, known []KnownIdentity, hintType string) []any {
	for i, v := range items {
		switch x := v.(type) {
		case string:
			items[i] = e.redactSliceString(parentKey, x, known, hintType)
		default:
			items[i] = e.walkAny(v, parentKey, known, hintType)
		}
	}
	return items
}

func (e *Engine) redactSliceString(parentKey, v string, known []KnownIdentity, hintType string) string {
	switch strings.ToLower(strings.TrimSpace(parentKey)) {
	case "to", "cc", "bcc":
		return e.RedactEmail(v)
	case "emails":
		return e.RedactEmail(v)
	case "phones":
		return e.RedactPhone(v)
	default:
		return e.redactStringByKey(parentKey, v, known, hintType)
	}
}

func (e *Engine) redactStringByKey(key, value string, known []KnownIdentity, hintType string) string {
	_ = hintType
	if shouldRedactTextField(key) {
		return e.RedactText(value, known)
	}
	return value
}

func shouldRedactTextField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "subject", "preview", "body", "text", "raw", "source", "content", "message", "snippet", "html", "customer", "assignee", "from", "assignedto", "created_by", "action":
		return true
	default:
		return false
	}
}

func inferEntityType(m map[string]any, key string, parentHint string) string {
	if m != nil {
		if t, ok := m["type"].(string); ok && strings.TrimSpace(t) != "" {
			return strings.ToLower(strings.TrimSpace(t))
		}
	}

	k := strings.ToLower(strings.TrimSpace(key))
	switch {
	case strings.Contains(k, "customer"):
		return "customer"
	case strings.Contains(k, "assignee"), strings.Contains(k, "assigned"), strings.Contains(k, "user"), strings.Contains(k, "owner"), strings.Contains(k, "member"):
		return "user"
	}
	return strings.ToLower(strings.TrimSpace(parentHint))
}

func knownIdentityFromMap(m map[string]any, entityType string) (KnownIdentity, bool) {
	if isSentinelPersonMap(m) {
		return KnownIdentity{}, false
	}

	first, _ := m["first"].(string)
	last, _ := m["last"].(string)
	email, _ := m["email"].(string)
	if first != "" || last != "" || email != "" {
		return KnownIdentity{
			Type:  entityType,
			First: first,
			Last:  last,
			Email: email,
		}, true
	}

	firstName, _ := m["firstName"].(string)
	lastName, _ := m["lastName"].(string)
	if firstName != "" || lastName != "" || email != "" {
		return KnownIdentity{
			Type:  defaultIfEmpty(entityType, "customer"),
			First: firstName,
			Last:  lastName,
			Email: email,
			Phone: getMapString(m, "phone"),
		}, true
	}

	if phone := getMapString(m, "phone"); phone != "" {
		return KnownIdentity{
			Type:  entityType,
			Phone: phone,
		}, true
	}

	return KnownIdentity{}, false
}

func (e *Engine) redactStructuredMap(m map[string]any, entityType string) {
	if !e.ShouldRedactType(entityType) {
		return
	}
	if isSentinelPersonMap(m) {
		return
	}

	// Person shape: {first,last,email}
	if _, ok := m["first"]; ok || m["last"] != nil {
		first, _ := m["first"].(string)
		last, _ := m["last"].(string)
		email, _ := m["email"].(string)
		rf, rl, re := e.RedactPerson(first, last, email)
		if first != "" || last != "" {
			m["first"] = rf
			m["last"] = rl
		}
		if email != "" {
			m["email"] = re
		}
	}

	// Customer/user shape: {firstName,lastName,email,phone}
	if _, ok := m["firstName"]; ok || m["lastName"] != nil || m["email"] != nil || m["phone"] != nil {
		first, _ := m["firstName"].(string)
		last, _ := m["lastName"].(string)
		email, _ := m["email"].(string)
		rf, rl, re := e.RedactPerson(first, last, email)
		if first != "" || last != "" {
			m["firstName"] = rf
			m["lastName"] = rl
		}
		if email != "" {
			m["email"] = re
		}
		if phone, ok := m["phone"].(string); ok && phone != "" {
			m["phone"] = e.RedactPhone(phone)
		}
	}

	// Embedded emails/phones arrays.
	if v, ok := m["emails"].([]any); ok {
		for i, raw := range v {
			em, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if val, ok := em["value"].(string); ok && val != "" {
				em["value"] = e.RedactEmail(val)
			}
			v[i] = em
		}
		m["emails"] = v
	}
	if v, ok := m["phones"].([]any); ok {
		for i, raw := range v {
			pm, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if val, ok := pm["value"].(string); ok && val != "" {
				pm["value"] = e.RedactPhone(val)
			}
			v[i] = pm
		}
		m["phones"] = v
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

func defaultIfEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
