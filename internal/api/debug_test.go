package api

import (
	"bytes"
	"io"
	"net/http"
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
	req.Header.Set("Content-Type", "application/json")

	resp, err := dt.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	log := buf.String()
	assert.Contains(t, log, "GET https://api.helpscout.net/v2/mailboxes")
	assert.Contains(t, log, "[redacted]")
	assert.NotContains(t, log, "secret-token")
	assert.Contains(t, log, "200 OK")
	assert.Contains(t, log, `{"id":1}`)

	// Response body should still be readable
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, `{"id":1}`, string(body))
}

func TestDebugTransport_LogsRequestBody(t *testing.T) {
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
	assert.Contains(t, log, `{"text":"hello"}`)
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
