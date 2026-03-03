package selfupdate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func isolateCheck(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	origDir := DirOverride
	DirOverride = dir
	t.Cleanup(func() { DirOverride = origDir })
}

func setBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := BaseURL
	BaseURL = url
	t.Cleanup(func() { BaseURL = orig })
}

func TestShouldCheck(t *testing.T) {
	t.Run("dev version", func(t *testing.T) {
		if ShouldCheck("dev") {
			t.Error("ShouldCheck should return false for dev")
		}
	})

	t.Run("no cache file", func(t *testing.T) {
		isolateCheck(t)
		if !ShouldCheck("1.0.0") {
			t.Error("ShouldCheck should return true when no cache")
		}
	})

	t.Run("recent cache", func(t *testing.T) {
		isolateCheck(t)
		writeCheckResult(&CheckResult{
			LatestVersion: "1.0.0",
			CheckedAt:     time.Now(),
		})
		if ShouldCheck("1.0.0") {
			t.Error("ShouldCheck should return false with recent cache")
		}
	})

	t.Run("stale cache", func(t *testing.T) {
		isolateCheck(t)
		writeCheckResult(&CheckResult{
			LatestVersion: "1.0.0",
			CheckedAt:     time.Now().Add(-25 * time.Hour),
		})
		if !ShouldCheck("1.0.0") {
			t.Error("ShouldCheck should return true with stale cache")
		}
	})

	t.Run("corrupt cache", func(t *testing.T) {
		isolateCheck(t)
		path := filepath.Join(DirOverride, checkFileName)
		_ = os.WriteFile(path, []byte("not json"), 0o600)
		if !ShouldCheck("1.0.0") {
			t.Error("ShouldCheck should return true with corrupt cache")
		}
	})
}

func TestFetchLatestRelease(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/repos/operator-kit/hs-cli/releases/latest" {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(ReleaseResponse{
				TagName: "v0.2.0",
				Assets: []Asset{
					{Name: "checksums.txt", BrowserDownloadURL: "http://example.com/checksums.txt"},
				},
			})
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		release, err := FetchLatestRelease()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if release == nil {
			t.Fatal("expected release, got nil")
		}
		if release.TagName != "v0.2.0" {
			t.Errorf("TagName = %q, want %q", release.TagName, "v0.2.0")
		}
	})

	t.Run("404 no published release", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		release, err := FetchLatestRelease()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if release != nil {
			t.Errorf("expected nil release for 404, got %+v", release)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		_, err := FetchLatestRelease()
		if err == nil {
			t.Fatal("expected error for 500 response")
		}
	})
}

func TestCheckForUpdate(t *testing.T) {
	t.Run("newer version available", func(t *testing.T) {
		isolateCheck(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ReleaseResponse{TagName: "v0.3.0"})
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		got := CheckForUpdate("0.2.0")
		if got != "0.3.0" {
			t.Errorf("CheckForUpdate = %q, want %q", got, "0.3.0")
		}
	})

	t.Run("already up to date", func(t *testing.T) {
		isolateCheck(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ReleaseResponse{TagName: "v0.2.0"})
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		got := CheckForUpdate("0.2.0")
		if got != "" {
			t.Errorf("CheckForUpdate = %q, want empty", got)
		}
	})

	t.Run("error returns empty", func(t *testing.T) {
		isolateCheck(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		got := CheckForUpdate("0.2.0")
		if got != "" {
			t.Errorf("CheckForUpdate = %q, want empty on error", got)
		}
	})

	t.Run("caches result", func(t *testing.T) {
		isolateCheck(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ReleaseResponse{TagName: "v0.3.0"})
		}))
		defer srv.Close()
		setBaseURL(t, srv.URL)

		CheckForUpdate("0.2.0")

		result := readCheckResult()
		if result == nil {
			t.Fatal("expected cached result")
		}
		if result.LatestVersion != "0.3.0" {
			t.Errorf("cached version = %q, want %q", result.LatestVersion, "0.3.0")
		}
	})
}
