package cmd

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhooksList(t *testing.T) {
	mock := &mockClient{
		ListWebhooksFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("webhooks", `[{
				"id":1,"url":"https://example.com/hook","state":"enabled","events":["convo.created"]
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "webhooks", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "example.com/hook")
	assert.Contains(t, out, "enabled")
}

func TestWebhooksGet(t *testing.T) {
	mock := &mockClient{
		GetWebhookFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{
				"id":1,"url":"https://example.com/hook","state":"enabled",
				"events":["convo.created","convo.deleted"],"secret":"s3cret"
			}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "webhooks", "get", "1"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "example.com/hook")
	assert.Contains(t, out, "s3cret")
}

func TestWebhooksCreate(t *testing.T) {
	mock := &mockClient{
		CreateWebhookFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(types.WebhookCreate)
			require.True(t, ok)
			require.Equal(t, "https://example.com/hook", payload.URL)
			require.Equal(t, []string{"convo.created"}, payload.Events)
			require.Equal(t, "sec", payload.Secret)
			require.Equal(t, "V2", payload.PayloadVersion)
			require.Equal(t, []int{1, 2}, payload.MailboxIDs)
			require.NotNil(t, payload.Notification)
			require.True(t, *payload.Notification)
			require.Equal(t, "Primary", payload.Label)
			return "55", nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "webhooks", "create",
		"--url", "https://example.com/hook",
		"--events", "convo.created",
		"--secret", "sec",
		"--payload-version", "V2",
		"--mailbox-ids", "1,2",
		"--notification",
		"--label", "Primary"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Created webhook 55")
}

func TestWebhooksUpdate(t *testing.T) {
	mock := &mockClient{
		UpdateWebhookFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "1", id)
			payload, ok := body.(types.WebhookUpdate)
			require.True(t, ok)
			require.Equal(t, "https://new.com/hook", payload.URL)
			require.Equal(t, "V1", payload.PayloadVersion)
			require.Equal(t, []int{10}, payload.MailboxIDs)
			require.Equal(t, "Updated", payload.Label)
			require.NotNil(t, payload.Notification)
			require.False(t, *payload.Notification)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "webhooks", "update", "1",
		"--url", "https://new.com/hook",
		"--payload-version", "V1",
		"--mailbox-ids", "10",
		"--notification=false",
		"--label", "Updated"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Updated webhook 1")
}

func TestWebhooksDelete(t *testing.T) {
	mock := &mockClient{
		DeleteWebhookFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "1", id)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "webhooks", "delete", "1"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Deleted webhook 1")
}
