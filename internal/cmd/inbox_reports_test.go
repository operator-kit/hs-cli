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

func TestReportsCompany(t *testing.T) {
	mock := &mockClient{
		GetReportFn: func(ctx context.Context, family string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "company", family)
			assert.Equal(t, "2026-01-01", params.Get("start"))
			assert.Equal(t, "2026-01-31", params.Get("end"))
			assert.Equal(t, "12", params.Get("mailbox"))
			assert.Equal(t, "responses", params.Get("view"))
			assert.Equal(t, "day", params.Get("granularity"))
			return json.RawMessage(`{"totals":{"conversations":10}}`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{
		"inbox", "reports", "company",
		"--start", "2026-01-01",
		"--end", "2026-01-31",
		"--mailbox", "12",
		"--view", "responses",
		"--param", "granularity=day",
	})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "company")
}

func TestReportsEmailPathMapping(t *testing.T) {
	mock := &mockClient{
		GetReportFn: func(ctx context.Context, family string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "email", family)
			return json.RawMessage(`{"ok":true}`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "reports", "email"})
	require.NoError(t, rootCmd.Execute())
}

func TestReportsRatingsPathMapping(t *testing.T) {
	mock := &mockClient{
		GetReportFn: func(ctx context.Context, family string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "happiness/ratings", family)
			return json.RawMessage(`{"ok":true}`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "reports", "ratings"})
	require.NoError(t, rootCmd.Execute())
}

func TestReportsUsersPathMapping(t *testing.T) {
	mock := &mockClient{
		GetReportFn: func(ctx context.Context, family string, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "user", family)
			return json.RawMessage(`{"ok":true}`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "reports", "users"})
	require.NoError(t, rootCmd.Execute())
}

func TestReportsFamiliesExposed(t *testing.T) {
	reportsCmd := newReportsCmd()
	names := make(map[string]struct{})
	for _, c := range reportsCmd.Commands() {
		names[c.Name()] = struct{}{}
	}
	for _, expected := range []string{
		"chats", "company", "conversations", "customers", "docs",
		"email", "productivity", "ratings", "users",
	} {
		_, ok := names[expected]
		assert.Truef(t, ok, "expected reports subcommand %q", expected)
	}
}
