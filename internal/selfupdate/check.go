package selfupdate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const checkFileName = "update-check.json"

// DirOverride allows tests to redirect the cache to a temp directory.
var DirOverride string

// BaseURL is the GitHub API base URL (override in tests with httptest).
var BaseURL = "https://api.github.com"

// CheckResult is persisted between runs to throttle update checks.
type CheckResult struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

// ReleaseResponse is the subset of GitHub's release JSON we need.
type ReleaseResponse struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset is a single file attached to a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func checkFilePath() string {
	if DirOverride != "" {
		return filepath.Join(DirOverride, checkFileName)
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "hs", checkFileName)
}

func readCheckResult() *CheckResult {
	path := checkFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var result CheckResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

func writeCheckResult(result *CheckResult) {
	path := checkFilePath()
	if path == "" {
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, data, 0o600)
}

// ShouldCheck returns true if >24h since last check and version is not "dev".
func ShouldCheck(currentVersion string) bool {
	if currentVersion == "dev" {
		return false
	}
	result := readCheckResult()
	if result == nil {
		return true
	}
	return time.Since(result.CheckedAt) > 24*time.Hour
}

// FetchLatestRelease fetches the latest published release from GitHub.
// Returns nil, nil if no published release exists (404).
func FetchLatestRelease() (*ReleaseResponse, error) {
	url := BaseURL + "/repos/operator-kit/hs-cli/releases/latest"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &release, nil
}

// CheckForUpdate fetches the latest release and returns the version if newer.
// Returns empty string if no update available or on any error.
func CheckForUpdate(currentVersion string) string {
	release, err := FetchLatestRelease()
	if err != nil || release == nil {
		return ""
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	writeCheckResult(&CheckResult{
		LatestVersion: latest,
		CheckedAt:     time.Now(),
	})

	if CompareVersions(currentVersion, latest) < 0 {
		return latest
	}
	return ""
}
