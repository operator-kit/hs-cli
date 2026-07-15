package api

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode"

	"github.com/operator-kit/hs-cli/internal/pii"
)

const diagnosticRedacted = "[redacted]"

type diagnosticSanitizer interface {
	sanitizeURL(*url.URL) string
	sanitizeHeader(string, []string) string
	sanitizeBody([]byte) string
}

type safeDiagnosticSanitizer struct {
	engine *pii.Engine
}

func newSafeDiagnosticSanitizer() diagnosticSanitizer {
	// Diagnostics always use the strongest mode and deliberately omit NER.
	// Free-form fields therefore collapse to RedactTextNotice rather than risk
	// persisting a name the model did not inspect.
	return &safeDiagnosticSanitizer{engine: pii.NewEngine(pii.ModeAll, "")}
}

func (s *safeDiagnosticSanitizer) sanitizeURL(input *url.URL) string {
	if input == nil {
		return ""
	}
	clean := *input
	clean.User = nil
	clean.Fragment = ""

	query := clean.Query()
	for key, values := range query {
		for i := range values {
			values[i] = diagnosticRedacted
		}
		query[key] = values
	}
	clean.RawQuery = query.Encode()

	segments := strings.Split(clean.Path, "/")
	for i, segment := range segments {
		if pathSegmentMayIdentifyPerson(segment) {
			segments[i] = diagnosticRedacted
		}
	}
	clean.Path = strings.Join(segments, "/")
	clean.RawPath = ""
	return clean.String()
}

func (s *safeDiagnosticSanitizer) sanitizeHeader(key string, values []string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "accept", "content-type", "content-length", "date", "retry-after",
		"x-ratelimit-limit", "x-ratelimit-remaining", "x-ratelimit-reset":
		return strings.Join(values, ", ")
	default:
		return diagnosticRedacted
	}
}

func (s *safeDiagnosticSanitizer) sanitizeBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	redacted, err := s.engine.RedactJSONWithContext(json.RawMessage(body), pii.JSONContext{Resource: pii.ResourceDiagnostic})
	if err != nil {
		return "[redacted non-JSON body]"
	}
	return string(redacted)
}

func pathSegmentMayIdentifyPerson(segment string) bool {
	decoded, err := url.PathUnescape(segment)
	if err != nil {
		return true
	}
	if strings.Contains(decoded, "@") {
		return true
	}

	digits := 0
	other := false
	for _, r := range decoded {
		switch {
		case unicode.IsDigit(r):
			digits++
		case r == '-' || r == '+' || r == '(' || r == ')' || unicode.IsSpace(r):
		default:
			other = true
		}
	}
	// Numeric identifiers and phone-like path values are not useful enough in a
	// persistent log to justify retaining them.
	return digits > 0 && !other
}
