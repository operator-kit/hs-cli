package cmd

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRatingsGet(t *testing.T) {
	mock := &mockClient{
		GetRatingFn: func(ctx context.Context, id string) (json.RawMessage, error) {
			assert.Equal(t, "7", id)
			return json.RawMessage(`{"id":7,"rating":"great","comments":"Very helpful"}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "ratings", "get", "7"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "great")
}
