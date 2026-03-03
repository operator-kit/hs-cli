//go:build integration

package api

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func integrationClient(t *testing.T) *Client {
	t.Helper()
	clientID := os.Getenv("HS_INBOX_APP_ID")
	clientSecret := os.Getenv("HS_INBOX_APP_SECRET")
	if clientID == "" || clientSecret == "" {
		t.Skip("HS_INBOX_APP_ID and HS_INBOX_APP_SECRET required")
	}
	return New(context.Background(), clientID, clientSecret, true)
}

func TestIntegration_ListMailboxes(t *testing.T) {
	c := integrationClient(t)
	data, err := c.ListMailboxes(context.Background(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	items, page, err := ExtractEmbedded(data, "mailboxes")
	require.NoError(t, err)
	require.NotNil(t, page)
	require.NotEmpty(t, items)
	t.Logf("Found %d mailboxes across %d pages", page.TotalElements, page.TotalPages)
}
