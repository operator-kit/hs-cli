package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowsList(t *testing.T) {
	mock := &mockClient{
		ListWorkflowsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("workflows", `[{
				"id":1,"name":"Auto-close","type":"automatic","status":"active","mailboxId":10
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "workflows", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Auto-close")
	assert.Contains(t, out, "automatic")
}

func TestWorkflowsListFilters(t *testing.T) {
	mock := &mockClient{
		ListWorkflowsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "10", params.Get("mailboxId"))
			assert.Equal(t, "manual", params.Get("type"))
			return halJSON("workflows", `[]`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "workflows", "list", "--mailbox-id", "10", "--type", "manual"})
	require.NoError(t, rootCmd.Execute())
}

func TestWorkflowsUpdateStatus(t *testing.T) {
	mock := &mockClient{
		UpdateWorkflowStatusFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "5", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "replace", payload["op"])
			assert.Equal(t, "/status", payload["path"])
			assert.Equal(t, "inactive", payload["value"])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "workflows", "update-status", "5", "--status", "inactive"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Workflow 5 status set to inactive")
}

func TestWorkflowsUpdateStatusRejectsInvalid(t *testing.T) {
	mock := &mockClient{}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "workflows", "update-status", "5", "--status", "paused"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"active" or "inactive"`)
}

func TestWorkflowsRun(t *testing.T) {
	mock := &mockClient{
		RunWorkflowFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "1", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, []int{100, 200}, payload["conversationIds"])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "workflows", "run", "1", "--conversation-ids", "100,200"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Workflow 1 executed")
}

func TestWorkflowsRunRejectsNonIntegerIDs(t *testing.T) {
	mock := &mockClient{
		RunWorkflowFn: func(ctx context.Context, id string, body any) error {
			t.Fatalf("RunWorkflow should not be called for invalid IDs")
			return nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "workflows", "run", "1", "--conversation-ids", "100,abc"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid conversation ID "abc"`)
}

func TestWorkflowsRunRejectsMoreThan50IDs(t *testing.T) {
	mock := &mockClient{
		RunWorkflowFn: func(ctx context.Context, id string, body any) error {
			t.Fatalf("RunWorkflow should not be called when ID count exceeds limit")
			return nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	ids := make([]string, 51)
	for i := 0; i < 51; i++ {
		ids[i] = fmt.Sprintf("%d", i+1)
	}

	rootCmd.SetArgs([]string{"inbox", "workflows", "run", "1", "--conversation-ids", strings.Join(ids, ",")})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 50 IDs")
}
