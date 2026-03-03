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

func TestMailboxFoldersList(t *testing.T) {
	mock := &mockClient{
		ListMailboxFoldersFn: func(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "10", mailboxID)
			assert.Equal(t, "1", params.Get("page"))
			assert.Equal(t, "25", params.Get("pageSize"))
			return halJSON("folders", `[{"id":1,"name":"Inbox","type":"active"}]`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "folders", "list", "10"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Inbox")
}

func TestMailboxCustomFieldsList(t *testing.T) {
	mock := &mockClient{
		ListMailboxCustomFieldsFn: func(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "10", mailboxID)
			return halJSON("customFields", `[{"id":5,"name":"Priority","type":"text"}]`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "custom-fields", "list", "10"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Priority")
}

func TestMailboxRoutingGet(t *testing.T) {
	mock := &mockClient{
		GetMailboxRoutingFn: func(ctx context.Context, mailboxID string) (json.RawMessage, error) {
			assert.Equal(t, "10", mailboxID)
			return json.RawMessage(`{"enabled":true}`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "routing", "get", "10"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "enabled")
}

func TestMailboxRoutingUpdate(t *testing.T) {
	mock := &mockClient{
		UpdateMailboxRoutingFn: func(ctx context.Context, mailboxID string, body any) error {
			assert.Equal(t, "10", mailboxID)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, true, payload["enabled"])
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "mailboxes", "routing", "update", "10", "--json", `{"enabled":true}`})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated routing for mailbox 10")
}
