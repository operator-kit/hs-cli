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

func TestTagsList(t *testing.T) {
	mock := &mockClient{
		ListTagsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("tags", `[{"id":1,"name":"bug","slug":"bug","color":"red"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tags", "list"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "bug")
	assert.Contains(t, out, "red")
}

func TestTagsGet(t *testing.T) {
	mock := &mockClient{
		GetTagFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "2", id)
			return json.RawMessage(`{"id":2,"name":"vip","slug":"vip","color":"#00ff00"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tags", "get", "2"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "vip")
	assert.Contains(t, out, "#00ff00")
}
