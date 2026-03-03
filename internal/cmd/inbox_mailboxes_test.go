package cmd

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMailboxesList_Table(t *testing.T) {
	mock := &mockClient{
		ListMailboxesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("mailboxes", `[{"id":1,"name":"Support","email":"support@test.com","slug":"support"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Support")
	assert.Contains(t, out, "support@test.com")
}

func TestMailboxesList_JSON(t *testing.T) {
	mock := &mockClient{
		ListMailboxesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("mailboxes", `[{"id":1,"name":"Support"}]`), nil
		},
	}
	buf := setupTest(mock)
	format = "json"
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "list", "--format", "json"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, `"id"`)
	assert.Contains(t, out, `"name"`)
}

func TestMailboxesGet(t *testing.T) {
	mock := &mockClient{
		GetMailboxFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{"id":1,"name":"Support","email":"support@test.com","slug":"support"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "get", "1"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Support")
	assert.Contains(t, out, "support@test.com")
}
