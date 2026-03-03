package cmd

import (
	"encoding/json"
	"fmt"
)

// docsCleanMinimal is a passthrough clean function for Docs resources.
// Docs API has no _links/_embedded to strip — just return as-is.
func docsCleanMinimal(m map[string]any) map[string]any {
	return m
}

// jsonStr extracts a string representation of a value from a map.
// Works with string, float64, bool, nil — covers all JSON primitive types.
func jsonStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// extractDocsID extracts an "id" field from a Docs API create response.
// Docs API returns {"<wrapper>":{"id":"...","name":"...",...}} for creates.
func extractDocsID(data json.RawMessage, wrapperKey string) string {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return ""
	}
	// Try wrapper key first
	if inner, ok := outer[wrapperKey]; ok {
		var obj map[string]any
		if err := json.Unmarshal(inner, &obj); err == nil {
			return jsonStr(obj, "id")
		}
	}
	// Fallback: top-level id
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err == nil {
		return jsonStr(obj, "id")
	}
	return ""
}
