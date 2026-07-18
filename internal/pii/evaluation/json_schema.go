package evaluation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidateJSONDocument executes the deliberately small JSON Schema vocabulary
// used by the checked evaluation schemas. Unsupported keywords fail closed so a
// schema edit cannot silently weaken validation.
func ValidateJSONDocument(schemaRaw, documentRaw []byte) error {
	schemaValue, err := decodeJSONValue(schemaRaw)
	if err != nil {
		return fmt.Errorf("decode JSON schema: %w", err)
	}
	schema, ok := schemaValue.(map[string]any)
	if !ok {
		return fmt.Errorf("JSON schema root is not an object")
	}
	document, err := decodeJSONValue(documentRaw)
	if err != nil {
		return fmt.Errorf("decode JSON document: %w", err)
	}
	validator := schemaValidator{root: schema}
	if err := validator.validate(schema, document, "$"); err != nil {
		return fmt.Errorf("validate JSON document: %w", err)
	}
	return nil
}

type schemaValidator struct {
	root map[string]any
}

var supportedSchemaKeywords = stringSet(
	"$schema", "$id", "$defs", "$ref", "title", "description", "type", "additionalProperties",
	"required", "properties", "const", "enum", "pattern", "minLength", "minimum", "maximum", "minItems", "items", "uniqueItems",
)

func (v schemaValidator) validate(schema map[string]any, value any, path string) error {
	for keyword := range schema {
		if !supportedSchemaKeywords[keyword] {
			return fmt.Errorf("%s: unsupported JSON Schema keyword %q", path, keyword)
		}
	}
	if reference, ok := schema["$ref"].(string); ok {
		resolved, err := v.resolve(reference)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return v.validate(resolved, value, path)
	}
	if expected, ok := schema["type"].(string); ok && !jsonTypeMatches(expected, value) {
		return fmt.Errorf("%s: got %s, want %s", path, jsonType(value), expected)
	}
	if constant, exists := schema["const"]; exists && !reflect.DeepEqual(constant, value) {
		return fmt.Errorf("%s: value differs from schema constant", path)
	}
	if choices, ok := schema["enum"].([]any); ok {
		matched := false
		for _, choice := range choices {
			if reflect.DeepEqual(choice, value) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%s: value is outside the schema enum", path)
		}
	}

	switch typed := value.(type) {
	case map[string]any:
		if err := v.validateObject(schema, typed, path); err != nil {
			return err
		}
	case []any:
		if err := v.validateArray(schema, typed, path); err != nil {
			return err
		}
	case string:
		if err := validateSchemaString(schema, typed, path); err != nil {
			return err
		}
	case json.Number:
		if err := validateSchemaNumber(schema, typed, path); err != nil {
			return err
		}
	}
	return nil
}

func (v schemaValidator) validateObject(schema map[string]any, value map[string]any, path string) error {
	properties, _ := schema["properties"].(map[string]any)
	if additional, exists := schema["additionalProperties"]; exists && additional == false {
		for name := range value {
			if _, known := properties[name]; !known {
				return fmt.Errorf("%s: unknown field %q", path, name)
			}
		}
	}
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			name, ok := item.(string)
			if !ok {
				return fmt.Errorf("%s: schema required entry is not a string", path)
			}
			if _, exists := value[name]; !exists {
				return fmt.Errorf("%s: missing required field %q", path, name)
			}
		}
	}
	for name, childValue := range value {
		rawChildSchema, exists := properties[name]
		if !exists {
			continue
		}
		childSchema, ok := rawChildSchema.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: property schema %q is not an object", path, name)
		}
		if err := v.validate(childSchema, childValue, path+"."+name); err != nil {
			return err
		}
	}
	return nil
}

func (v schemaValidator) validateArray(schema map[string]any, value []any, path string) error {
	if minimum, exists := schemaInteger(schema["minItems"]); exists && len(value) < minimum {
		return fmt.Errorf("%s: has %d items, minimum is %d", path, len(value), minimum)
	}
	if unique, _ := schema["uniqueItems"].(bool); unique {
		seen := make(map[string]struct{}, len(value))
		for _, item := range value {
			encoded, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("%s: encode unique item: %w", path, err)
			}
			key := string(encoded)
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%s: contains duplicate array item", path)
			}
			seen[key] = struct{}{}
		}
	}
	if rawItems, exists := schema["items"]; exists {
		items, ok := rawItems.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: items schema is not an object", path)
		}
		for index, item := range value {
			if err := v.validate(items, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSchemaString(schema map[string]any, value, path string) error {
	if minimum, exists := schemaInteger(schema["minLength"]); exists && utf8.RuneCountInString(value) < minimum {
		return fmt.Errorf("%s: string is shorter than %d Unicode code points", path, minimum)
	}
	if pattern, ok := schema["pattern"].(string); ok {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("%s: schema has invalid pattern: %w", path, err)
		}
		if !compiled.MatchString(value) {
			return fmt.Errorf("%s: string does not match schema pattern", path)
		}
	}
	return nil
}

func validateSchemaNumber(schema map[string]any, value json.Number, path string) error {
	if expected, _ := schema["type"].(string); expected == "integer" {
		if _, err := value.Int64(); err != nil {
			return fmt.Errorf("%s: number is not an integer", path)
		}
	}
	number, err := value.Float64()
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return fmt.Errorf("%s: number is invalid", path)
	}
	if minimum, exists := schemaNumber(schema["minimum"]); exists && number < minimum {
		return fmt.Errorf("%s: number is below minimum %g", path, minimum)
	}
	if maximum, exists := schemaNumber(schema["maximum"]); exists && number > maximum {
		return fmt.Errorf("%s: number is above maximum %g", path, maximum)
	}
	return nil
}

func (v schemaValidator) resolve(reference string) (map[string]any, error) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(reference, prefix) || strings.Contains(strings.TrimPrefix(reference, prefix), "/") {
		return nil, fmt.Errorf("unsupported schema reference %q", reference)
	}
	definitions, ok := v.root["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema has no $defs object")
	}
	name := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(reference, prefix), "~1", "/"), "~0", "~")
	resolved, ok := definitions[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema reference %q does not resolve to an object", reference)
	}
	return resolved, nil
}

func decodeJSONValue(raw []byte) (any, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("document is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("document contains multiple JSON values")
		}
		return nil, err
	}
	return value, nil
}

func jsonTypeMatches(expected string, value any) bool {
	switch expected {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := number.Int64()
		return err == nil
	case "number":
		_, ok := value.(json.Number)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return value == nil
	default:
		return false
	}
}

func jsonType(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func schemaInteger(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(string(number))
	return parsed, err == nil
}

func schemaNumber(value any) (float64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Float64()
	return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
}
