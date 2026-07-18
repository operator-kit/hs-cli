package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDebugTransport_LogsRequestAndResponse(t *testing.T) {
	var buf bytes.Buffer
	dt := &debugTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Header:     http.Header{"X-Test": {"val"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":1}`)),
			}, nil
		}),
		out: &buf,
	}

	req, _ := http.NewRequest("GET", "https://api.helpscout.net/v2/mailboxes", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Cookie", "session=alice.critical@example.com")
	req.Header.Set("X-Api-Key", "private-api-key")
	req.Header.Set("Content-Type", "application/json")

	resp, err := dt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	log := buf.String()
	assert.Contains(t, log, "GET https://api.helpscout.net/v2/mailboxes")
	assert.Contains(t, log, "[redacted]")
	assert.NotContains(t, log, "secret-token")
	assert.NotContains(t, log, "alice.critical@example.com")
	assert.NotContains(t, log, "private-api-key")
	assert.Contains(t, log, "200 OK")
	assert.Contains(t, log, `{"id":1}`)

	// Response body should still be readable
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `{"id":1}`, string(body))
}

func TestDebugTransport_PreservesRequestBodyAndSanitizesLog(t *testing.T) {
	var buf bytes.Buffer
	dt := &debugTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			// Verify body was restored for the actual request
			b, _ := io.ReadAll(req.Body)
			assert.Equal(t, `{"text":"hello"}`, string(b))
			return &http.Response{
				StatusCode: 201,
				Status:     "201 Created",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
		out: &buf,
	}

	req, _ := http.NewRequest("POST", "https://api.helpscout.net/v2/conversations/1/notes", io.NopCloser(strings.NewReader(`{"text":"hello"}`)))

	_, err := dt.RoundTrip(req)
	require.NoError(t, err)

	log := buf.String()
	assert.NotContains(t, log, `{"text":"hello"}`)
	assert.Contains(t, log, "redacted")
}

func TestPIIRegression_Critical03_DebugTransportDoesNotPersistPII(t *testing.T) {
	var buf bytes.Buffer
	const responseBody = `{"comments":"Call Alice at 415-555-0199 or email alice.critical@example.com"}`
	dt := &debugTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(responseBody)),
			}, nil
		}),
		out: &buf,
	}

	req, err := http.NewRequest(
		"POST",
		"https://api.helpscout.net/v2/customers?query=alice.critical%40example.com",
		strings.NewReader(`{"email":"alice.critical@example.com"}`),
	)
	require.NoError(t, err)

	resp, err := dt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	log := buf.String()
	assert.NotContains(t, log, "alice.critical%40example.com", "debug logs must sanitize PII in request URLs")
	assert.NotContains(t, log, "alice.critical@example.com", "debug logs must sanitize PII in request and response bodies")
	assert.NotContains(t, log, "415-555-0199", "debug logs must sanitize PII in response bodies")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, responseBody, string(body), "sanitizing logs must not alter the response returned to the caller")
}

func TestDebugTransport_SkipsAuthRequests(t *testing.T) {
	var buf bytes.Buffer
	dt := &debugTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
		out: &buf,
	}

	req, _ := http.NewRequest("POST", "https://api.helpscout.net/v2/oauth2/token", strings.NewReader("grant_type=client_credentials"))

	_, err := dt.RoundTrip(req)
	require.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestDebugTransport_LogsError(t *testing.T) {
	var buf bytes.Buffer
	dt := &debugTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		}),
		out: &buf,
	}

	req, _ := http.NewRequest("GET", "https://api.helpscout.net/v2/mailboxes", nil)

	_, err := dt.RoundTrip(req)
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "error:")
}

func TestDiagnosticSanitizersUseIndependentEphemeralSecrets(t *testing.T) {
	first := newSafeDiagnosticSanitizerWithRandom(bytes.NewReader(bytes.Repeat([]byte{0x11}, 32)))
	second := newSafeDiagnosticSanitizerWithRandom(bytes.NewReader(bytes.Repeat([]byte{0x22}, 32)))
	const body = `{"email":"alice.critical@example.com"}`

	firstOutput := first.sanitizeBody([]byte(body))
	secondOutput := second.sanitizeBody([]byte(body))
	assert.NotContains(t, firstOutput, "alice.critical@example.com")
	assert.NotContains(t, secondOutput, "alice.critical@example.com")
	assert.NotEqual(t, firstOutput, secondOutput, "diagnostic identities must not correlate across sanitizers")
}

func TestDiagnosticSanitizerRandomFailureFallsBackToOpaqueOutput(t *testing.T) {
	sanitizer := newSafeDiagnosticSanitizerWithRandom(failingReader{err: errors.New("entropy unavailable")})
	const piiBody = `{"email":"alice.critical@example.com"}`

	assert.NotContains(t, sanitizer.sanitizeBody([]byte(piiBody)), "alice.critical@example.com")
	assert.Contains(t, sanitizer.sanitizeBody([]byte(piiBody)), "redacted")
	assert.Equal(t, "[redacted URL]", sanitizer.sanitizeURL(mustURL(t, "https://api.helpscout.net/v2/customers/123")))
}

type failingReader struct {
	err error
}

func (r failingReader) Read([]byte) (int, error) {
	return 0, r.err
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	return parsed
}
