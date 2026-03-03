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

func TestCustomerPropertiesList(t *testing.T) {
	mock := &mockClient{
		ListCustomerPropertiesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("customerProperties", `[{"id":1,"name":"Tier","type":"text"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "properties", "customers", "list"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Tier")
}

func TestCustomerPropertiesGet(t *testing.T) {
	mock := &mockClient{
		GetCustomerPropertyFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			return json.RawMessage(`{"id":1,"name":"Tier","type":"text"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "properties", "customers", "get", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "text")
}

func TestConversationPropertiesList(t *testing.T) {
	mock := &mockClient{
		ListConversationPropertiesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("conversationProperties", `[{"id":2,"name":"Priority","type":"select"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "properties", "conversations", "list"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Priority")
}

func TestConversationPropertiesGet(t *testing.T) {
	mock := &mockClient{
		GetConversationPropertyFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "2", id)
			return json.RawMessage(`{"id":2,"name":"Priority","type":"select"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "properties", "conversations", "get", "2"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "select")
}
