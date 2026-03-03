package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// debugTransport wraps an http.RoundTripper and logs requests/responses to a file.
// Auth token requests are skipped to avoid exposing credentials.
type debugTransport struct {
	base http.RoundTripper
	out  io.Writer
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "oauth2/token") {
		return t.base.RoundTrip(req)
	}

	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(t.out, "\n--- %s %s %s ---\n", ts, req.Method, req.URL)

	for k, vals := range req.Header {
		if strings.EqualFold(k, "Authorization") {
			fmt.Fprintf(t.out, ">> %s: [redacted]\n", k)
			continue
		}
		fmt.Fprintf(t.out, ">> %s: %s\n", k, strings.Join(vals, ", "))
	}

	if req.Body != nil && req.Body != http.NoBody {
		body, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err == nil && len(body) > 0 {
			fmt.Fprintf(t.out, ">> %s\n", body)
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		fmt.Fprintf(t.out, "<< error: %v\n", err)
		return nil, err
	}

	fmt.Fprintf(t.out, "<< %s\n", resp.Status)
	for k, vals := range resp.Header {
		fmt.Fprintf(t.out, "<< %s: %s\n", k, strings.Join(vals, ", "))
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err == nil && len(body) > 0 {
		fmt.Fprintf(t.out, "<< %s\n", body)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return resp, nil
}

// setupDebugLog creates the debug log file and wraps the client transport.
func setupDebugLog(httpClient *http.Client) {
	path := "hs-debug.log"
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create debug log: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Debug log: %s\n", path)
	httpClient.Transport = &debugTransport{
		base: httpClient.Transport,
		out:  f,
	}
}
