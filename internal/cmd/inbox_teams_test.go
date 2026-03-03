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

func TestTeamsList(t *testing.T) {
	mock := &mockClient{
		ListTeamsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "1", params.Get("page"))
			assert.Empty(t, params.Get("pageSize"))
			return halJSON("teams", `[{"id":1,"name":"Support"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "teams", "list"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Support")
}

func TestTeamsMembers(t *testing.T) {
	mock := &mockClient{
		ListTeamMembersFn: func(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "1", id)
			assert.Equal(t, "1", params.Get("page"))
			assert.Empty(t, params.Get("pageSize"))
			return halJSON("users", `[{"id":2,"firstName":"Agent","lastName":"One","email":"agent@test.com","role":"user"}]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "teams", "members", "1"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "agent@test.com")
}
