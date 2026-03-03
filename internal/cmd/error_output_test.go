package cmd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldShowUsageForError_UnknownCommand(t *testing.T) {
	err := errors.New(`unknown command "nope" for "hs inbox"`)
	assert.True(t, shouldShowUsageForError(err))
}

func TestExecute_UnknownFlag_PrintsUsage(t *testing.T) {
	_, buf := setupE2E(t)

	rootCmd.SetArgs([]string{"inbox", "config", "get", "--wat"})
	err := Execute()
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "Error:")
	assert.Contains(t, out, "unknown flag")
	assert.Contains(t, out, "Usage:")
}

func TestExecute_ArgCountMismatch_PrintsUsage(t *testing.T) {
	_, buf := setupE2E(t)

	rootCmd.SetArgs([]string{"inbox", "config", "get", "a", "b"})
	err := Execute()
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "Error:")
	assert.Contains(t, out, "accepts at most 1 arg(s), received 2")
	assert.Contains(t, out, "Usage:")
}

func TestExecute_MissingRequiredFlag_PrintsUsage(t *testing.T) {
	_, buf := setupE2E(t)
	t.Setenv("HS_INBOX_APP_ID", "test-id")
	t.Setenv("HS_INBOX_APP_SECRET", "test-secret")

	rootCmd.SetArgs([]string{"inbox", "saved-replies", "create"})
	err := Execute()
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "Error:")
	assert.Contains(t, out, "required flag")
	assert.Contains(t, out, "Usage:")
}

func TestExecute_RuntimeAuthError_DoesNotPrintUsage(t *testing.T) {
	_, buf := setupE2E(t)
	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	apiClient = nil

	rootCmd.SetArgs([]string{"inbox", "customers", "list"})
	err := Execute()
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "Error:")
	assert.Contains(t, out, "not authenticated")
	assert.Contains(t, out, "HS_INBOX_APP_ID")
	assert.Contains(t, out, "HS_INBOX_APP_SECRET")
	assert.Contains(t, out, "MCP server env")
	assert.Contains(t, out, "hs inbox auth login")
	assert.Contains(t, out, "npx -y @operatorkit/hs inbox auth login")
	assert.NotContains(t, out, "Usage:")
}

func TestExecute_RuntimeError_DoesNotPrintUsage(t *testing.T) {
	_, buf := setupE2E(t)

	rootCmd.SetArgs([]string{"inbox", "config", "set", "--inbox-pii-mode", "invalid"})
	err := Execute()
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "Error:")
	assert.Contains(t, err.Error(), "invalid --inbox-pii-mode")
	assert.NotContains(t, buf.String(), "Usage:")
}
