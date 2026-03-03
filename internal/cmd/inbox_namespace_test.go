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

func TestInboxNamespaceConversationsList(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("conversations", `[{
				"id":1,"number":100,"subject":"Inbox scoped","status":"active",
				"primaryCustomer":{"email":"inbox@test.com"},"userUpdatedAt":"2025-01-01"
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "conversations", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Inbox scoped")
	assert.Contains(t, out, "inbox@test.com")
}
