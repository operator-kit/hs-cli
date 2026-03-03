package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "table", cfg.Format)
	assert.Empty(t, cfg.InboxAppID)
	assert.Empty(t, cfg.InboxAppSecret)
	assert.Zero(t, cfg.InboxDefaultMailbox)
	assert.Equal(t, "", cfg.InboxPIIMode)
	assert.False(t, cfg.InboxPIIAllowUnredacted)
}

func TestLoad_ParseYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
inbox_app_id: myid
inbox_app_secret: mysecret
inbox_default_mailbox: 42
format: json
inbox_pii_mode: customers
inbox_pii_allow_unredacted: true
`), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "myid", cfg.InboxAppID)
	assert.Equal(t, "mysecret", cfg.InboxAppSecret)
	assert.Equal(t, 42, cfg.InboxDefaultMailbox)
	assert.Equal(t, "json", cfg.Format)
	assert.Equal(t, "customers", cfg.InboxPIIMode)
	assert.True(t, cfg.InboxPIIAllowUnredacted)
}

func TestLoad_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
inbox_app_id: fromfile
format: table
inbox_pii_mode: all
`), 0o600))

	t.Setenv("HS_INBOX_APP_ID", "fromenv")
	t.Setenv("HS_FORMAT", "csv")
	t.Setenv("HS_INBOX_PII_MODE", "customers")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "1")

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "fromenv", cfg.InboxAppID)
	assert.Equal(t, "csv", cfg.Format)
	assert.Equal(t, "customers", cfg.InboxPIIMode)
	assert.True(t, cfg.InboxPIIAllowUnredacted)
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`{{{invalid`), 0o600))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.yaml")

	orig := &Config{
		InboxAppID:              "id1",
		InboxAppSecret:          "secret1",
		InboxDefaultMailbox:     10,
		Format:                  "csv",
		InboxPIIMode:            "all",
		InboxPIIAllowUnredacted: true,
	}
	require.NoError(t, Save(path, orig))

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, orig.InboxAppID, loaded.InboxAppID)
	assert.Equal(t, orig.InboxAppSecret, loaded.InboxAppSecret)
	assert.Equal(t, orig.InboxDefaultMailbox, loaded.InboxDefaultMailbox)
	assert.Equal(t, orig.Format, loaded.Format)
	assert.Equal(t, orig.InboxPIIMode, loaded.InboxPIIMode)
	assert.Equal(t, orig.InboxPIIAllowUnredacted, loaded.InboxPIIAllowUnredacted)
}
