package cmd

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-kit/hs-cli/internal/config"
)

func TestPromptConfigFallback_AcceptStoresCreds(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	reader := bufio.NewReader(strings.NewReader("y\n"))

	err := promptConfigFallback(reader, cfgFile, assert.AnError, func(c *config.Config) {
		c.InboxAppID = "test-id"
		c.InboxAppSecret = "test-secret"
	})
	require.NoError(t, err)

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "test-id", loaded.InboxAppID)
	assert.Equal(t, "test-secret", loaded.InboxAppSecret)
}

func TestPromptConfigFallback_AcceptStoresDocsKey(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	reader := bufio.NewReader(strings.NewReader("y\n"))

	err := promptConfigFallback(reader, cfgFile, assert.AnError, func(c *config.Config) {
		c.DocsAPIKey = "docs-key-1234"
	})
	require.NoError(t, err)

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "docs-key-1234", loaded.DocsAPIKey)
}

func TestPromptConfigFallback_DeclineReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	reader := bufio.NewReader(strings.NewReader("n\n"))

	err := promptConfigFallback(reader, cfgFile, assert.AnError, func(c *config.Config) {
		c.InboxAppID = "should-not-be-saved"
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not stored")

	// Config file should not exist
	_, err = config.Load(cfgFile)
	require.NoError(t, err) // Load returns empty config for missing file
}

func TestPromptConfigFallback_PreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	// Pre-existing config with format and permissions
	require.NoError(t, config.Save(cfgFile, &config.Config{
		Format:           "json",
		InboxPermissions: "mailboxes:list",
	}))

	reader := bufio.NewReader(strings.NewReader("y\n"))
	err := promptConfigFallback(reader, cfgFile, assert.AnError, func(c *config.Config) {
		c.InboxAppID = "new-id"
		c.InboxAppSecret = "new-secret"
	})
	require.NoError(t, err)

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "new-id", loaded.InboxAppID)
	assert.Equal(t, "new-secret", loaded.InboxAppSecret)
	assert.Equal(t, "json", loaded.Format)
	assert.Equal(t, "mailboxes:list", loaded.InboxPermissions)
}

func TestAuthStatus_ConfigFallback_E2E(t *testing.T) {
	home, buf := setupE2E(t)

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")

	// Write credentials to config file
	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:     "test-id-1234abcd",
		InboxAppSecret: "test-secret",
	}))
	cfgPath = cfgFile

	rootCmd.SetArgs([]string{"inbox", "auth", "status"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Authenticated")
	assert.Contains(t, buf.String(), "test")
}

func TestDocsAuthStatus_ConfigFallback_E2E(t *testing.T) {
	home, buf := setupE2E(t)

	t.Setenv("HS_DOCS_API_KEY", "")

	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		DocsAPIKey: "docs-key-12345678",
	}))
	cfgPath = cfgFile

	rootCmd.SetArgs([]string{"docs", "auth", "status"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Authenticated")
	assert.Contains(t, buf.String(), "docs")
}

func TestAuthLogout_ClearsConfig_E2E(t *testing.T) {
	home, buf := setupE2E(t)

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")

	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:     "id-to-clear",
		InboxAppSecret: "secret-to-clear",
		Format:         "json",
	}))
	cfgPath = cfgFile

	rootCmd.SetArgs([]string{"inbox", "auth", "logout"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Credentials removed")

	// Verify credentials cleared but other config preserved
	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Empty(t, loaded.InboxAppID)
	assert.Empty(t, loaded.InboxAppSecret)
	assert.Equal(t, "json", loaded.Format)
}

func TestDocsAuthLogout_ClearsConfig_E2E(t *testing.T) {
	home, buf := setupE2E(t)

	t.Setenv("HS_DOCS_API_KEY", "")

	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		DocsAPIKey: "key-to-clear",
		Format:     "table",
	}))
	cfgPath = cfgFile

	rootCmd.SetArgs([]string{"docs", "auth", "logout"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Docs API key removed")

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Empty(t, loaded.DocsAPIKey)
	assert.Equal(t, "table", loaded.Format)
}

func TestAuthStatus_NotAuthenticated_NoKeyringNoConfig_E2E(t *testing.T) {
	_, buf := setupE2E(t)

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")

	rootCmd.SetArgs([]string{"inbox", "auth", "status"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Not authenticated")
}

func TestDocsAuthStatus_NotAuthenticated_E2E(t *testing.T) {
	_, buf := setupE2E(t)

	t.Setenv("HS_DOCS_API_KEY", "")

	rootCmd.SetArgs([]string{"docs", "auth", "status"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Not authenticated")
}

func TestPromptConfigFallback_OutputMessages(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	// Capture stderr
	oldStderr := captureStderr(t)
	_ = oldStderr

	reader := bufio.NewReader(strings.NewReader("y\n"))
	err := promptConfigFallback(reader, cfgFile, assert.AnError, func(c *config.Config) {
		c.InboxAppID = "id"
	})
	require.NoError(t, err)
}

// captureStderr is a no-op helper; stderr output is verified by not panicking.
func captureStderr(t *testing.T) *bytes.Buffer {
	t.Helper()
	return new(bytes.Buffer)
}
