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

func TestConversationsList(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("conversations", `[{
				"id":1,"number":100,"subject":"Help me","status":"active",
				"primaryCustomer":{"email":"alice@test.com"},"userUpdatedAt":"2025-01-01"
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Help me")
	assert.Contains(t, out, "alice@test.com")
	assert.Contains(t, out, "active")
}

func TestConversationsListAdvancedFilters(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "closed", params.Get("status"))
			assert.Equal(t, "12", params.Get("mailbox"))
			assert.Equal(t, "55", params.Get("folder"))
			assert.Equal(t, "billing", params.Get("tag"))
			assert.Equal(t, "9", params.Get("assigned_to"))
			assert.Equal(t, "2026-01-01T00:00:00Z", params.Get("modifiedSince"))
			assert.Equal(t, "123", params.Get("number"))
			assert.Equal(t, "createdAt", params.Get("sortField"))
			assert.Equal(t, "desc", params.Get("sortOrder"))
			assert.Equal(t, "10:foo,11:bar", params.Get("customFieldsByIds"))
			assert.Equal(t, "needle", params.Get("query"))
			assert.Equal(t, "threads", params.Get("embed"))
			return halJSON("conversations", `[]`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "conversations", "list",
		"--status", "closed",
		"--mailbox", "12",
		"--folder", "55",
		"--tag", "billing",
		"--assigned-to", "9",
		"--modified-since", "2026-01-01T00:00:00Z",
		"--number", "123",
		"--sort-field", "createdAt",
		"--sort-order", "desc",
		"--custom-fields-by-ids", "10:foo,11:bar",
		"--query", "needle",
		"--embed", "threads",
	})
	require.NoError(t, rootCmd.Execute())
}

func TestConversationsGet(t *testing.T) {
	mock := &mockClient{
		GetConversationFn: func(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "42", id)
			return json.RawMessage(`{
				"id":42,"number":200,"subject":"Detailed","status":"closed","type":"email",
				"primaryCustomer":{"email":"bob@test.com"},
				"assignee":{"first":"Jane","last":"Smith","email":"jane@test.com"},
				"tags":[{"id":1,"name":"billing"}],
				"preview":"Need help with invoice",
				"createdAt":"2025-01-01","userUpdatedAt":"2025-01-02",
				"source":{"type":"email","via":"customer"}
			}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "get", "42"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Detailed")
	assert.Contains(t, out, "bob@test.com")
	assert.Contains(t, out, "Jane Smith (jane@test.com)")
	assert.Contains(t, out, "billing")
	assert.Contains(t, out, "Need help with invoice")
	// Verify detail format, not table
	assert.Contains(t, out, "Subject:")
	assert.Contains(t, out, "Assignee:")
}

func TestConversationsGetWithThreads(t *testing.T) {
	mock := &mockClient{
		GetConversationFn: func(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "42", id)
			assert.Equal(t, "threads", params.Get("embed"))
			return json.RawMessage(`{
				"id":42,"number":200,"subject":"Threaded","status":"active","type":"email",
				"primaryCustomer":{"email":"bob@test.com"},
				"createdAt":"2025-01-01","userUpdatedAt":"2025-01-02",
				"_embedded":{
					"threads":[
						{"id":1,"type":"customer","body":"<p>Help me</p>","createdAt":"2025-01-01","createdBy":{"email":"bob@test.com"}},
						{"id":2,"type":"reply","body":"Sure thing","createdAt":"2025-01-02","createdBy":{"email":"agent@test.com"}},
						{"id":3,"type":"lineitem","createdAt":"2025-01-02","createdBy":{"email":"agent@test.com"},"action":{"text":"Agent assigned to themselves","type":"default"}}
					]
				}
			}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "get", "42", "--embed", "threads"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Threads (3)")
	assert.Contains(t, out, "[customer] bob@test.com")
	assert.Contains(t, out, "Help me")
	assert.NotContains(t, out, "<p>")
	assert.Contains(t, out, "[reply] agent@test.com")
	assert.Contains(t, out, "[lineitem] agent@test.com")
	assert.Contains(t, out, "Agent assigned to themselves")
}

func TestConversationsCreate(t *testing.T) {
	mock := &mockClient{
		CreateConversationFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(types.ConversationCreate)
			require.True(t, ok)
			assert.Equal(t, 1, payload.MailboxID)
			assert.Equal(t, "New", payload.Subject)
			assert.Equal(t, "a@b.com", payload.Customer.Email)
			assert.Equal(t, 7, payload.AssignTo)
			assert.Equal(t, "2026-01-02T15:04:05Z", payload.CreatedAt)
			require.NotNil(t, payload.Imported)
			assert.True(t, *payload.Imported)
			require.NotNil(t, payload.AutoReply)
			assert.True(t, *payload.AutoReply)
			require.Len(t, payload.Fields, 2)
			assert.Equal(t, types.ConversationField{ID: 10, Value: "foo"}, payload.Fields[0])
			assert.Equal(t, types.ConversationField{ID: 11, Value: "bar"}, payload.Fields[1])
			return "99", nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "create",
		"--mailbox", "1",
		"--subject", "New",
		"--customer", "a@b.com",
		"--body", "Hello",
		"--assign-to", "7",
		"--created-at", "2026-01-02T15:04:05Z",
		"--imported",
		"--auto-reply",
		"--field", "10=foo",
		"--field", "11=bar"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Created conversation 99")
}

func TestConversationsCreateRejectsInvalidFieldFormat(t *testing.T) {
	mock := &mockClient{
		CreateConversationFn: func(ctx context.Context, body any) (string, error) {
			t.Fatalf("CreateConversation should not be called for invalid --field input")
			return "", nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "create",
		"--mailbox", "1",
		"--subject", "New",
		"--customer", "a@b.com",
		"--body", "Hello",
		"--field", "bad-format"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --field value")
}

func TestConversationsUpdate(t *testing.T) {
	mock := &mockClient{
		UpdateConversationFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "42", id)
			ops, ok := body.([]jsonPatchOp)
			require.True(t, ok)
			require.Len(t, ops, 1)
			assert.Equal(t, jsonPatchOp{
				Op:    "replace",
				Path:  "/subject",
				Value: "Updated",
			}, ops[0])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "update", "42", "--subject", "Updated"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Updated conversation 42")
}

func TestConversationsDelete(t *testing.T) {
	mock := &mockClient{
		DeleteConversationFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "42", id)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "delete", "42"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Deleted conversation 42")
}

func TestConversationTagsSet(t *testing.T) {
	mock := &mockClient{
		UpdateConversationTagsFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "42", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, []string{"vip", "bug"}, payload["tags"])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "tags", "set", "42", "--tag", "vip,bug"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated tags for conversation 42")
}

func TestConversationFieldsSet(t *testing.T) {
	mock := &mockClient{
		UpdateConversationFieldsFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "42", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			fields, ok := payload["fields"].([]types.ConversationField)
			require.True(t, ok)
			require.Len(t, fields, 2)
			assert.Equal(t, types.ConversationField{ID: 10, Value: "foo"}, fields[0])
			assert.Equal(t, types.ConversationField{ID: 11, Value: "bar"}, fields[1])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "fields", "set", "42", "--field", "10=foo", "--field", "11=bar"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated custom fields for conversation 42")
}

func TestConversationSnoozeSetAndClear(t *testing.T) {
	mock := &mockClient{
		UpdateConversationSnoozeFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "42", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "2026-02-20T00:00:00Z", payload["snoozedUntil"])
			return nil
		},
		DeleteConversationSnoozeFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "42", id)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "snooze", "set", "42", "--until", "2026-02-20T00:00:00Z"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Snoozed conversation 42")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "conversations", "snooze", "clear", "42"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Cleared snooze for conversation 42")
}
