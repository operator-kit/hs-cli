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

func TestOrganizationsList(t *testing.T) {
	mock := &mockClient{
		ListOrganizationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "acme", params.Get("query"))
			return halJSON("organizations", `[{"id":1,"name":"Acme","domain":"acme.com"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "organizations", "list", "--query", "acme"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Acme")
}

func TestOrganizationsGet(t *testing.T) {
	mock := &mockClient{
		GetOrganizationFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{"id":1,"name":"Acme","domain":"acme.com"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "organizations", "get", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "acme.com")
}

func TestOrganizationsCreateUpdateDelete(t *testing.T) {
	mock := &mockClient{
		CreateOrganizationFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Acme", payload["name"])
			assert.Equal(t, "acme.com", payload["domain"])
			return "1", nil
		},
		UpdateOrganizationFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "1", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Acme 2", payload["name"])
			return nil
		},
		DeleteOrganizationFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "1", id)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "organizations", "create", "--name", "Acme", "--domain", "acme.com"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Created organization 1")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "update", "1", "--name", "Acme 2"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated organization 1")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "delete", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Deleted organization 1")
}

func TestOrganizationRelatedLists(t *testing.T) {
	mock := &mockClient{
		ListOrganizationConversationsFn: func(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return halJSON("conversations", `[{"id":9,"number":99,"subject":"Billing","status":"active"}]`), nil
		},
		ListOrganizationCustomersFn: func(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return halJSON("customers", `[{"id":8,"firstName":"Alice","lastName":"A","email":"alice@example.com"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "organizations", "conversations", "list", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Billing")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "customers", "list", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "alice@example.com")
}

func TestOrganizationPropertiesCRUD(t *testing.T) {
	mock := &mockClient{
		ListOrganizationPropertiesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("properties", `[{"id":1,"name":"Tier","type":"text"}]`), nil
		},
		GetOrganizationPropertyFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{"id":1,"name":"Tier","type":"text"}`), nil
		},
		CreateOrganizationPropertyFn: func(ctx context.Context, body any) (string, error) {
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Tier", payload["name"])
			return "1", nil
		},
		UpdateOrganizationPropertyFn: func(ctx context.Context, id string, body any) error {
			assert.Equal(t, "1", id)
			payload, ok := body.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "Tier2", payload["name"])
			return nil
		},
		DeleteOrganizationPropertyFn: func(ctx context.Context, id string) error {
			assert.Equal(t, "1", id)
			return nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "organizations", "properties", "list"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Tier")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "properties", "get", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "text")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "properties", "create", "--name", "Tier", "--type", "text"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Created organization property 1")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "properties", "update", "1", "--name", "Tier2"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Updated organization property 1")

	buf.Reset()
	rootCmd.SetArgs([]string{"inbox", "organizations", "properties", "delete", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Deleted organization property 1")
}
