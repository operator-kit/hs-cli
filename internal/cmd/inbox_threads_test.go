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

func TestThreadsList(t *testing.T) {
	mock := &mockClient{
		ListThreadsFn: func(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "10", convID)
			return halJSON("threads", `[{
				"id":1,"type":"customer","body":"Hello there",
				"createdAt":"2025-01-01","createdBy":{"email":"alice@test.com"}
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "list", "10"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "customer")
	assert.Contains(t, out, "Hello there")
	assert.Contains(t, out, "alice@test.com")
}

func TestThreadsListLineitemAction(t *testing.T) {
	mock := &mockClient{
		ListThreadsFn: func(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
			return halJSON("threads", `[{
				"id":1,"type":"lineitem",
				"createdAt":"2025-01-01","createdBy":{"email":"agent@test.com"},
				"action":{"text":"You marked as Active","type":"default"}
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "list", "10"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "lineitem")
	assert.Contains(t, out, "You marked as Active")
}

func TestThreadsReply(t *testing.T) {
	mock := &mockClient{
		CreateReplyFn: func(ctx context.Context, convID string, body any) error {
			assert.Equal(t, "10", convID)
			payload, ok := body.(types.ReplyBody)
			require.True(t, ok)
			assert.Equal(t, "a@b.com", payload.Customer.Email)
			assert.Equal(t, "My reply", payload.Text)
			assert.Equal(t, "closed", payload.Status)
			assert.Equal(t, 5, payload.User)
			assert.Equal(t, []types.Person{{Email: "to@example.com"}}, payload.To)
			assert.Equal(t, []types.Person{{Email: "cc@example.com"}}, payload.CC)
			assert.Equal(t, []types.Person{{Email: "bcc@example.com"}}, payload.BCC)
			assert.Equal(t, "email", payload.Type)
			assert.Equal(t, "2026-01-01T00:00:00Z", payload.CreatedAt)
			assert.Equal(t, []int{1, 2}, payload.Attachments)
			require.NotNil(t, payload.Draft)
			assert.True(t, *payload.Draft)
			require.NotNil(t, payload.Imported)
			assert.True(t, *payload.Imported)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "reply", "10",
		"--customer", "a@b.com",
		"--body", "My reply",
		"--status", "closed",
		"--user-id", "5",
		"--to", "to@example.com",
		"--cc", "cc@example.com",
		"--bcc", "bcc@example.com",
		"--draft",
		"--imported",
		"--created-at", "2026-01-01T00:00:00Z",
		"--type", "email",
		"--attachment-id", "1,2"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Reply sent.")
}

func TestThreadsNote(t *testing.T) {
	mock := &mockClient{
		CreateNoteFn: func(ctx context.Context, convID string, body any) error {
			assert.Equal(t, "10", convID)
			payload, ok := body.(types.NoteBody)
			require.True(t, ok)
			assert.Equal(t, "Internal note", payload.Text)
			assert.Equal(t, "pending", payload.Status)
			assert.Equal(t, 6, payload.User)
			assert.Equal(t, []int{4, 5}, payload.Attachments)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "note", "10",
		"--body", "Internal note",
		"--user-id", "6",
		"--status", "pending",
		"--attachment-id", "4,5"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Note added.")
}

func TestThreadsCreateChat(t *testing.T) {
	mock := &mockClient{
		CreateChatThreadFn: func(ctx context.Context, convID string, body any) error {
			assert.Equal(t, "10", convID)
			payload, ok := body.(types.ThreadCreateBody)
			require.True(t, ok)
			assert.Equal(t, "chat@example.com", payload.Customer.Email)
			assert.Equal(t, "Chat body", payload.Text)
			assert.Equal(t, []int{7, 8}, payload.Attachments)
			require.NotNil(t, payload.Imported)
			assert.True(t, *payload.Imported)
			assert.Equal(t, "2026-01-02T00:00:00Z", payload.CreatedAt)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "create-chat", "10",
		"--customer", "chat@example.com",
		"--body", "Chat body",
		"--imported",
		"--created-at", "2026-01-02T00:00:00Z",
		"--attachment-id", "7,8"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Created create-chat thread on conversation 10.")
}

func TestThreadsCreateCustomer(t *testing.T) {
	mock := &mockClient{
		CreateCustomerThreadFn: func(ctx context.Context, convID string, body any) error {
			assert.Equal(t, "10", convID)
			payload, ok := body.(types.ThreadCreateBody)
			require.True(t, ok)
			assert.Equal(t, "customer@example.com", payload.Customer.Email)
			assert.Equal(t, "Customer body", payload.Text)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "create-customer", "10",
		"--customer", "customer@example.com",
		"--body", "Customer body"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Created create-customer thread on conversation 10.")
}

func TestThreadsCreatePhone(t *testing.T) {
	mock := &mockClient{
		CreatePhoneThreadFn: func(ctx context.Context, convID string, body any) error {
			assert.Equal(t, "10", convID)
			payload, ok := body.(types.ThreadCreateBody)
			require.True(t, ok)
			assert.Equal(t, "Phone body", payload.Text)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "create-phone", "10",
		"--body", "Phone body"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Created create-phone thread on conversation 10.")
}

func TestThreadsUpdate(t *testing.T) {
	mock := &mockClient{
		UpdateThreadFn: func(ctx context.Context, convID string, threadID string, body any) error {
			assert.Equal(t, "10", convID)
			assert.Equal(t, "20", threadID)
			ops, ok := body.([]jsonPatchOp)
			require.True(t, ok)
			require.Len(t, ops, 2)
			assert.Equal(t, "/body", ops[0].Path)
			assert.Equal(t, "Updated body", ops[0].Value)
			assert.Equal(t, "/status", ops[1].Path)
			assert.Equal(t, "closed", ops[1].Value)
			return nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "update", "10", "20",
		"--text", "Updated body",
		"--status", "closed"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated thread 20 on conversation 10.")
}

func TestThreadsSource(t *testing.T) {
	mock := &mockClient{
		GetThreadSourceFn: func(ctx context.Context, convID string, threadID string) (json.RawMessage, error) {
			assert.Equal(t, "10", convID)
			assert.Equal(t, "20", threadID)
			return json.RawMessage(`{"raw":"source payload"}`), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "source", "10", "20"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "source payload")
}

func TestThreadsSourceRFC822(t *testing.T) {
	mock := &mockClient{
		GetThreadSourceRFC822Fn: func(ctx context.Context, convID string, threadID string) ([]byte, error) {
			assert.Equal(t, "10", convID)
			assert.Equal(t, "20", threadID)
			return []byte("From: test@example.com"), nil
		},
	}

	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "source-rfc822", "10", "20"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "From: test@example.com")
}

func TestThreadsList_PIIRedactsWhenEnabled(t *testing.T) {
	mock := &mockClient{
		ListThreadsFn: func(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
			return halJSON("threads", `[{
				"id":1,"type":"customer","body":"Contact me at alice@test.com",
				"createdAt":"2025-01-01","createdBy":{"first":"Alice","last":"Smith","email":"alice@test.com","type":"customer"}
			}]`), nil
		},
	}
	buf := setupTest(mock)
	t.Setenv("HS_INBOX_PII_MODE", "all")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "0")
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "list", "10"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.NotContains(t, out, "alice@test.com")
	assert.NotContains(t, out, "Alice Smith")
}

func TestThreadsList_UnredactedDeniedWhenDisallowed(t *testing.T) {
	mock := &mockClient{
		ListThreadsFn: func(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
			return halJSON("threads", `[]`), nil
		},
	}
	setupTest(mock)
	t.Setenv("HS_INBOX_PII_MODE", "all")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "0")
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "--unredacted", "conversations", "threads", "list", "10"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--unredacted is disabled")
}

func TestThreadsSourceRFC822_RedactedWhenEnabled(t *testing.T) {
	mock := &mockClient{
		GetThreadSourceRFC822Fn: func(ctx context.Context, convID string, threadID string) ([]byte, error) {
			return []byte("From: test@example.com\nBody: John Smith\nPhone: 415-555-1212"), nil
		},
	}
	buf := setupTest(mock)
	t.Setenv("HS_INBOX_PII_MODE", "all")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "0")
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "threads", "source-rfc822", "10", "20"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.NotContains(t, out, "test@example.com")
	assert.NotContains(t, out, "John Smith")
	assert.NotContains(t, out, "415-555-1212")
}

func TestThreadsSourceRFC822_UnredactedAllowed(t *testing.T) {
	mock := &mockClient{
		GetThreadSourceRFC822Fn: func(ctx context.Context, convID string, threadID string) ([]byte, error) {
			return []byte("From: test@example.com"), nil
		},
	}
	buf := setupTest(mock)
	t.Setenv("HS_INBOX_PII_MODE", "all")
	t.Setenv("HS_INBOX_PII_ALLOW_UNREDACTED", "1")
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "--unredacted", "conversations", "threads", "source-rfc822", "10", "20"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "From: test@example.com")
}
