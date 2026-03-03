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

func TestSavedRepliesList(t *testing.T) {
	mock := &mockClient{
		ListSavedRepliesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "10", params.Get("mailboxId"))
			assert.Equal(t, "welcome", params.Get("query"))
			return halJSON("savedReplies", `[{"id":1,"name":"Welcome","subject":"Hello","isPrivate":false}]`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "saved-replies", "list", "--mailbox-id", "10", "--query", "welcome"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Welcome")
}

func TestSavedRepliesGet(t *testing.T) {
	mock := &mockClient{
		GetSavedReplyFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{"id":1,"name":"Welcome","subject":"Hello","text":"Hi there","isPrivate":false}`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "saved-replies", "get", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Hi there")
}

func TestSavedRepliesCreate(t *testing.T) {
	mock := &mockClient{
		CreateSavedReplyFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, 10, payload["mailboxId"])
			assert.Equal(t, "Welcome", payload["name"])
			assert.Equal(t, "Hi there", payload["text"])
			assert.Equal(t, true, payload["isPrivate"])
			return "55", nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "saved-replies", "create",
		"--mailbox-id", "10",
		"--name", "Welcome",
		"--body", "Hi there",
		"--private",
	})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Created saved reply 55")
}

func TestSavedRepliesUpdate(t *testing.T) {
	mock := &mockClient{
		UpdateSavedReplyFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "55", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Updated", payload["name"])
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "saved-replies", "update", "55", "--name", "Updated"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated saved reply 55")
}

func TestSavedRepliesDelete(t *testing.T) {
	mock := &mockClient{
		DeleteSavedReplyFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "55", id)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "saved-replies", "delete", "55"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Deleted saved reply 55")
}
