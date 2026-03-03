package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewForTest(srv.Client())
	// Override baseURL by making requests go to test server
	// We'll use a custom approach: patch the client's http transport
	// Actually we need to override baseURL. Let's use a different approach.
	return c, srv
}

// clientWithBaseURL creates a test client that talks to the given server.
// It works by wrapping the http.Client with a custom RoundTripper that
// rewrites URLs to point at the test server.
func clientWithServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	transport := &rewriteTransport{
		base:    http.DefaultTransport,
		baseURL: srv.URL,
	}
	httpClient := &http.Client{Transport: transport}
	return NewForTest(httpClient)
}

// rewriteTransport rewrites all requests to go to the test server.
type rewriteTransport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme = "http"
	// Parse test server URL to get host
	req.URL.Host = t.baseURL[len("http://"):]
	return t.base.RoundTrip(req)
}

func TestClient_GET_200(t *testing.T) {
	var gotPath string
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/hal+json")
		json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "Test"})
	})

	data, err := c.ListMailboxes(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "/v2/mailboxes", gotPath)
	assert.Contains(t, string(data), `"name"`)
}

func TestClient_GET_404(t *testing.T) {
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	})

	_, err := c.GetMailbox(context.Background(), "999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "API error 404: not found")
}

func TestFormatAPIError_EmptyBody(t *testing.T) {
	got := formatAPIError(http.StatusNotFound, nil)
	assert.Equal(t, "Not Found", got)
}

func TestFormatAPIError_JSONMessage(t *testing.T) {
	body := []byte(`{"logRef":"abc","message":"Bad request","_embedded":{"errors":[]},"_links":{}}`)
	got := formatAPIError(http.StatusBadRequest, body)
	assert.Equal(t, "Bad request", got)
}

func TestFormatAPIError_JSONMessageWithFieldErrors(t *testing.T) {
	body := []byte(`{"message":"Validation failed","_embedded":{"errors":[{"path":"subject","message":"may not be empty"}]}}`)
	got := formatAPIError(http.StatusBadRequest, body)
	assert.Equal(t, "Validation failed (subject: may not be empty)", got)
}

func TestFormatAPIError_NonJSON(t *testing.T) {
	body := []byte("plain text error")
	got := formatAPIError(http.StatusInternalServerError, body)
	assert.Equal(t, "plain text error", got)
}

func TestClient_429_Retry(t *testing.T) {
	calls := 0
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 1})
	})

	data, err := c.GetMailbox(context.Background(), "1")
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Contains(t, string(data), `"id"`)
}

func TestClient_POST_Location(t *testing.T) {
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Location", "https://api.helpscout.net/v2/conversations/42")
		w.WriteHeader(http.StatusCreated)
	})

	id, err := c.CreateConversation(context.Background(), map[string]any{"subject": "test"})
	require.NoError(t, err)
	assert.Equal(t, "42", id)
}

func TestClient_DELETE(t *testing.T) {
	var gotMethod string
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	err := c.DeleteConversation(context.Background(), "123")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, gotMethod)
}

func TestClient_ContentType_OnBody(t *testing.T) {
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		}
		w.Header().Set("Location", "https://api.helpscout.net/v2/webhooks/1")
		w.WriteHeader(http.StatusCreated)
	})

	_, err := c.CreateWebhook(context.Background(), map[string]any{"url": "https://example.com"})
	require.NoError(t, err)
}

func TestClient_ContentType_NoBody(t *testing.T) {
	c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
		// GET requests should not have Content-Type
		assert.Empty(t, r.Header.Get("Content-Type"))
		json.NewEncoder(w).Encode(map[string]any{"id": 1})
	})

	_, err := c.GetMailbox(context.Background(), "1")
	require.NoError(t, err)
}
