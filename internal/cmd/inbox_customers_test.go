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

func TestCustomersList(t *testing.T) {
	mock := &mockClient{
		ListCustomersFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("customers", `[{
				"id":1,"firstName":"Alice","lastName":"Smith","email":"alice@test.com","createdAt":"2025-01-01"
			}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "Smith")
}

func TestCustomersListAdvancedFilters(t *testing.T) {
	mock := &mockClient{
		ListCustomersFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "12", params.Get("mailbox"))
			assert.Equal(t, "Alice", params.Get("firstName"))
			assert.Equal(t, "Smith", params.Get("lastName"))
			assert.Equal(t, "2026-01-01T00:00:00Z", params.Get("modifiedSince"))
			assert.Equal(t, "firstName", params.Get("sortField"))
			assert.Equal(t, "asc", params.Get("sortOrder"))
			assert.Equal(t, "alice@example.com", params.Get("query"))
			assert.Empty(t, params.Get("embed"))
			return halJSON("customers", `[]`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "customers", "list",
		"--mailbox", "12",
		"--first-name", "Alice",
		"--last-name", "Smith",
		"--modified-since", "2026-01-01T00:00:00Z",
		"--sort-field", "firstName",
		"--sort-order", "asc",
		"--query", "alice@example.com",
	})
	require.NoError(t, rootCmd.Execute())
}

func TestCustomersGet(t *testing.T) {
	mock := &mockClient{
		GetCustomerFn: func(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "5", id)
			return json.RawMessage(`{
				"id":5,"firstName":"Bob","lastName":"Jones","email":"bob@test.com","phone":"555-1234","createdAt":"2025-01-01"
			}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "get", "5"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "Bob")
	assert.Contains(t, out, "555-1234")
}

func TestCustomersCreate(t *testing.T) {
	mock := &mockClient{
		CreateCustomerFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Carol", payload["firstName"])
			return "77", nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "create", "--first-name", "Carol"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Created customer 77")
}

func TestCustomersCreateWithExtraFlags(t *testing.T) {
	mock := &mockClient{
		CreateCustomerFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Carol", payload["firstName"])
			assert.Equal(t, "Engineer", payload["jobTitle"])
			assert.Equal(t, "Paris", payload["location"])
			assert.Equal(t, "female", payload["gender"])
			org, ok := payload["organization"].(map[string]int)
			require.True(t, ok)
			assert.Equal(t, 42, org["id"])
			return "99", nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "customers", "create",
		"--first-name", "Carol",
		"--job-title", "Engineer",
		"--location", "Paris",
		"--gender", "female",
		"--organization-id", "42",
	})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Created customer 99")
}

// NOTE: --json test must run after scalar flag tests due to cobra singleton flag state.
func TestCustomersCreateWithJSON(t *testing.T) {
	mock := &mockClient{
		CreateCustomerFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Test", payload["firstName"])
			emails, ok := payload["emails"].([]any)
			require.True(t, ok)
			require.Len(t, emails, 1)
			return "88", nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "customers", "create",
		"--json", `{"firstName":"Test","emails":[{"type":"work","value":"test@example.com"}]}`,
	})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Created customer 88")
}

func TestCustomersUpdate(t *testing.T) {
	mock := &mockClient{
		UpdateCustomerFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "5", id)
			ops, ok := body.([]jsonPatchOp)
			require.True(t, ok)
			require.Len(t, ops, 1)
			assert.Equal(t, jsonPatchOp{
				Op:    "replace",
				Path:  "/firstName",
				Value: "Updated",
			}, ops[0])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "update", "5", "--first-name", "Updated"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Updated customer 5")
}

func TestCustomersOverwrite(t *testing.T) {
	mock := &mockClient{
		OverwriteCustomerFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "5", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Updated", payload["firstName"])
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "overwrite", "5", "--first-name", "Updated"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Overwrote customer 5")
}

func TestCustomersDelete(t *testing.T) {
	mock := &mockClient{
		DeleteCustomerFn: func(ctx context.Context, id string, params url.Values) error {
			assert.Equal(t, "5", id)
			assert.Empty(t, params.Get("async"))
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "delete", "5"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Deleted customer 5")
}

func TestCustomersDeleteAsync(t *testing.T) {
	mock := &mockClient{
		DeleteCustomerFn: func(ctx context.Context, id string, params url.Values) error {
			assert.Equal(t, "10", id)
			assert.Equal(t, "true", params.Get("async"))
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "customers", "delete", "10", "--async"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Deleted customer 10")
}
