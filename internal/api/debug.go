package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// debugTransport wraps an http.RoundTripper and records requests/responses
// through an always-on diagnostic sanitizer. Debug logging is deliberately
// independent of the user-facing --unredacted policy.
type debugTransport struct {
	base      http.RoundTripper
	out       io.Writer
	sanitizer diagnosticSanitizer
	mu        sync.Mutex
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	if strings.Contains(req.URL.Path, "oauth2/token") {
		return base.RoundTrip(req)
	}

	sanitizer := t.sanitizer
	if sanitizer == nil {
		sanitizer = newSafeDiagnosticSanitizer()
	}

	var record bytes.Buffer
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(&record, "\n--- %s %s %s ---\n", ts, req.Method, sanitizer.sanitizeURL(req.URL))
	writeSanitizedHeaders(&record, ">>", req.Header, sanitizer)

	if req.Body != nil && req.Body != http.NoBody {
		body, readErr := io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
		if readErr != nil {
			fmt.Fprintln(&record, ">> [redacted unreadable body]")
		} else if len(body) > 0 {
			fmt.Fprintf(&record, ">> %s\n", sanitizer.sanitizeBody(body))
		}
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		// Transport errors often embed the full request URL. Preserve the error
		// category for diagnosis without persisting its potentially sensitive text.
		fmt.Fprintf(&record, "<< error: request failed (%T)\n", err)
		t.writeRecord(record.String())
		return nil, err
	}

	fmt.Fprintf(&record, "<< %s\n", resp.Status)
	writeSanitizedHeaders(&record, "<<", resp.Header, sanitizer)

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr != nil {
		fmt.Fprintln(&record, "<< [redacted unreadable body]")
	} else if len(body) > 0 {
		fmt.Fprintf(&record, "<< %s\n", sanitizer.sanitizeBody(body))
	}

	t.writeRecord(record.String())
	return resp, nil
}

func writeSanitizedHeaders(out io.Writer, prefix string, headers http.Header, sanitizer diagnosticSanitizer) {
	for key, values := range headers {
		fmt.Fprintf(out, "%s %s: %s\n", prefix, key, sanitizer.sanitizeHeader(key, values))
	}
}

func (t *debugTransport) writeRecord(record string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = io.WriteString(t.out, record)
}

// setupDebugLog creates a private debug log file and wraps the client transport.
func setupDebugLog(httpClient *http.Client) {
	path := "hs-debug.log"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create debug log: %v\n", err)
		return
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		fmt.Fprintf(os.Stderr, "Warning: could not secure debug log: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Debug log: %s\n", path)
	httpClient.Transport = &debugTransport{
		base: httpClient.Transport,
		out:  f,
	}
}
