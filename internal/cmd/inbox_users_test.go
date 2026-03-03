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

func TestUsersList(t *testing.T) {
	mock := &mockClient{
		ListUsersFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("users", `[{
				"id":1,"firstName":"Admin","lastName":"User","email":"admin@test.com","role":"owner"
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Admin")
	assert.Contains(t, out, "owner")
}

func TestUsersListFilters(t *testing.T) {
	mock := &mockClient{
		ListUsersFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "agent@example.com", params.Get("email"))
			assert.Equal(t, "88", params.Get("mailbox"))
			return halJSON("users", `[]`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "list", "--email", "agent@example.com", "--mailbox", "88"})
	require.NoError(t, rootCmd.Execute())
}

func TestUsersGet(t *testing.T) {
	mock := &mockClient{
		GetUserFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{
				"id":1,"firstName":"Admin","lastName":"User","email":"admin@test.com","role":"owner","type":"user"
			}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "get", "1"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Admin")
	assert.Contains(t, out, "user")
}

func TestUsersMe(t *testing.T) {
	mock := &mockClient{
		GetResourceOwnerFn: func(ctx context.Context) (json.RawMessage, error) {
			return json.RawMessage(`{
				"id":9,"firstName":"Owner","lastName":"User","email":"owner@test.com","role":"owner","type":"user"
			}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "me"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Owner")
}

func TestUsersDelete(t *testing.T) {
	mock := &mockClient{
		DeleteUserFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "9", id)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "delete", "9"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Deleted user 9")
}

func TestUsersStatusList(t *testing.T) {
	mock := &mockClient{
		ListUserStatusesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("statuses", `[{"id":"active","name":"Active","color":"green"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "status", "list"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Active")
}

func TestUsersStatusGet(t *testing.T) {
	mock := &mockClient{
		GetUserStatusFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "9", id)
			return json.RawMessage(`{"id":"away","name":"Away"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "status", "get", "9"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "away")
}

func TestUsersStatusSet(t *testing.T) {
	mock := &mockClient{
		SetUserStatusFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "9", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "away", payload["status"])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "users", "status", "set", "9", "--status", "away"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated status for user 9")
}
