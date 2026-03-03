package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-kit/hs-cli/internal/config"
)

func TestConfigSet(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgPath = cfgFile

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "set", "--inbox-app-id", "myid", "--inbox-app-secret", "mysecret", "--inbox-default-mailbox", "42", "--format", "json", "--inbox-pii-mode", "customers", "--inbox-pii-allow-unredacted"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Config saved")

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "myid", loaded.InboxAppID)
	assert.Equal(t, "mysecret", loaded.InboxAppSecret)
	assert.Equal(t, 42, loaded.InboxDefaultMailbox)
	assert.Equal(t, "json", loaded.Format)
	assert.Equal(t, "customers", loaded.InboxPIIMode)
	assert.True(t, loaded.InboxPIIAllowUnredacted)
}

func TestConfigSet_Partial(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgPath = cfgFile

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:          "original-id",
		InboxAppSecret:      "original-secret",
		InboxDefaultMailbox: 10,
		Format:              "table",
		InboxPIIMode:        "off",
	}))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "set", "--inbox-app-id", "new-id"})
	require.NoError(t, rootCmd.Execute())

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "new-id", loaded.InboxAppID)
	assert.Equal(t, "original-secret", loaded.InboxAppSecret)
	assert.Equal(t, 10, loaded.InboxDefaultMailbox)
	assert.Equal(t, "table", loaded.Format)
	assert.Equal(t, "off", loaded.InboxPIIMode)
}

func TestConfigSet_MutualFields(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgPath = cfgFile

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "set", "--inbox-app-id", "myid", "--inbox-default-mailbox", "99"})
	require.NoError(t, rootCmd.Execute())

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "myid", loaded.InboxAppID)
	assert.Equal(t, 99, loaded.InboxDefaultMailbox)
}

func TestConfigGet(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgPath = cfgFile

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:              "myid",
		InboxAppSecret:          "mysecret",
		InboxDefaultMailbox:     42,
		Format:                  "json",
		InboxPIIMode:            "all",
		InboxPIIAllowUnredacted: true,
	}))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "get"})
	require.NoError(t, rootCmd.Execute())

	output := buf.String()
	assert.Contains(t, output, "inbox-app-id: myid")
	assert.Contains(t, output, "inbox-app-secret: mysecret")
	assert.Contains(t, output, "inbox-default-mailbox: 42")
	assert.Contains(t, output, "format: json")
	assert.Contains(t, output, "inbox-pii-mode: all")
	assert.Contains(t, output, "inbox-pii-allow-unredacted: true")
}

func TestConfigGet_SingleKey(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgPath = cfgFile

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:          "myid",
		InboxAppSecret:      "mysecret",
		InboxDefaultMailbox: 42,
	}))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "get", "inbox-app-id"})
	require.NoError(t, rootCmd.Execute())

	assert.Equal(t, "myid\n", buf.String())
}

func TestConfigPath(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	cfgPath = cfgFile

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "path"})
	require.NoError(t, rootCmd.Execute())

	assert.Equal(t, cfgFile+"\n", buf.String())
}

func TestConfigGet_SinglePIIModeKey(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	cfgPath = cfgFile

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxPIIMode: "customers",
	}))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "get", "inbox-pii-mode"})
	require.NoError(t, rootCmd.Execute())

	assert.Equal(t, "customers\n", buf.String())
}

func TestConfigSet_InvalidPIIMode(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	cfgPath = filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("HS_INBOX_PII_MODE", "")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"inbox", "config", "set", "--inbox-pii-mode", "bad"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --inbox-pii-mode")
}
