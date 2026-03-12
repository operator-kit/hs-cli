package cmd

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBriefingTeamOverview(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Empty(t, params.Get("embed")) // team overview must NOT embed threads
			switch params.Get("status") {
			case "active":
				return halJSON("conversations", `[
					{"id":1,"number":100,"subject":"A","status":"active",
					 "primaryCustomer":{"email":"c1@test.com"},"userUpdatedAt":"2025-01-01",
					 "assignee":{"id":42,"first":"Alex","last":"Morgan","email":"alex@test.com"}},
					{"id":4,"number":103,"subject":"D","status":"active",
					 "primaryCustomer":{"email":"c4@test.com"},"userUpdatedAt":"2025-01-04"}
				]`), nil
			case "pending":
				return halJSON("conversations", `[
					{"id":2,"number":101,"subject":"B","status":"pending",
					 "primaryCustomer":{"email":"c2@test.com"},"userUpdatedAt":"2025-01-02",
					 "assignee":{"id":42,"first":"Alex","last":"Morgan","email":"alex@test.com"}}
				]`), nil
			case "closed":
				assert.NotEmpty(t, params.Get("modifiedSince"))
				return halJSON("conversations", `[
					{"id":3,"number":102,"subject":"C","status":"closed",
					 "primaryCustomer":{"email":"c3@test.com"},"userUpdatedAt":"2025-01-03",
					 "assignee":{"id":55,"first":"Jane","last":"Doe","email":"jane@test.com"}}
				]`), nil
			default:
				t.Fatalf("unexpected status: %q", params.Get("status"))
				return nil, nil
			}
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tools", "briefing"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "42") // agent ID for Alex
	assert.Contains(t, out, "Alex Morgan")
	assert.Contains(t, out, "55") // agent ID for Jane
	assert.Contains(t, out, "Jane Doe")
	assert.Contains(t, out, "(unassigned)")
	assert.Contains(t, out, "Total: 2 active")
	assert.Contains(t, out, "1 pending")
	assert.Contains(t, out, "1 closed")
	assert.Contains(t, out, "Team Briefing")
}

func TestBriefingAgentSummary(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "99", params.Get("assigned_to"))
			assert.Equal(t, "threads", params.Get("embed"))
			switch params.Get("status") {
			case "active":
				return halJSON("conversations", `[
					{"id":1,"number":100,"subject":"Help me","status":"active",
					 "createdAt":"2025-01-01T10:00:00Z",
					 "primaryCustomer":{"email":"alice@test.com"},"userUpdatedAt":"2025-01-01",
					 "assignee":{"first":"Alex","last":"Morgan","email":"alex@test.com"},
					 "_embedded":{"threads":[
						{"id":10,"type":"customer","body":"Need help","createdAt":"2025-01-01T10:00:00Z","createdBy":{"email":"alice@test.com"}},
						{"id":11,"type":"reply","body":"On it","createdAt":"2025-01-01T11:30:00Z","createdBy":{"email":"agent@test.com"}}
					 ]}}
				]`), nil
			case "pending":
				return halJSON("conversations", `[
					{"id":2,"number":101,"subject":"Billing question","status":"pending",
					 "createdAt":"2025-01-02T08:00:00Z",
					 "primaryCustomer":{"email":"bob@test.com"},"userUpdatedAt":"2025-01-02",
					 "assignee":{"first":"Alex","last":"Morgan","email":"alex@test.com"},
					 "_embedded":{"threads":[
						{"id":20,"type":"customer","body":"Question","createdAt":"2025-01-02T08:00:00Z","createdBy":{"email":"bob@test.com"}}
					 ]}}
				]`), nil
			case "closed":
				return halJSON("conversations", `[
					{"id":3,"number":102,"subject":"Old issue","status":"closed",
					 "createdAt":"2025-01-03T08:00:00Z",
					 "primaryCustomer":{"email":"carol@test.com"},"userUpdatedAt":"2025-01-03",
					 "assignee":{"first":"Alex","last":"Morgan","email":"alex@test.com"},
					 "_embedded":{"threads":[]}}
				]`), nil
			default:
				t.Fatalf("unexpected status: %q", params.Get("status"))
				return nil, nil
			}
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--assigned-to", "99"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	// Summary line with agent name and counts
	assert.Contains(t, out, "Briefing")
	assert.Contains(t, out, "Alex Morgan")
	assert.Contains(t, out, "1 open")
	assert.Contains(t, out, "1 pending")
	assert.Contains(t, out, "1 closed (7d)")
	// Active and pending conversations shown
	assert.Contains(t, out, "Help me")
	assert.Contains(t, out, "alice@test.com")
	assert.Contains(t, out, "Billing question")
	assert.Contains(t, out, "bob@test.com")
	// Closed conversation filtered out
	assert.NotContains(t, out, "Old issue")
	assert.NotContains(t, out, "carol@test.com")
	// Thread summary columns present (headers are title case)
	assert.Contains(t, out, "Threads")
	assert.Contains(t, out, "Last Activity")
	assert.Contains(t, out, "ago") // last activity shown as relative time
}

func TestBriefingAgentSummaryJSON(t *testing.T) {
	activeConv := `[
		{"id":1,"number":100,"subject":"Help me","status":"active",
		 "createdAt":"2025-01-01T10:00:00Z",
		 "primaryCustomer":{"first":"Alice","last":"Smith","email":"alice@test.com"},
		 "assignee":{"first":"Alex","last":"M","email":"alex@test.com"},
		 "userUpdatedAt":"2025-01-01",
		 "_embedded":{"threads":[
			{"id":10,"type":"customer","body":"Need help","createdAt":"2025-01-01T10:00:00Z",
			 "createdBy":{"first":"Alice","last":"Smith","email":"alice@test.com"}},
			{"id":11,"type":"reply","body":"On it","createdAt":"2025-01-01T11:30:00Z",
			 "createdBy":{"first":"Alex","last":"M","email":"alex@test.com"}}
		 ]}}
	]`
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			if params.Get("status") == "active" {
				return halJSON("conversations", activeConv), nil
			}
			return halJSON("conversations", `[]`), nil
		},
	}
	buf := setupTest(mock)
	format = "json"
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--assigned-to", "99", "--format", "json"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result, 1)
	conv := result[0]

	// Conversation fields
	assert.Equal(t, float64(1), conv["id"])
	assert.Equal(t, "Help me", conv["subject"])
	assert.Equal(t, "Alice Smith (alice@test.com)", conv["customer"])
	assert.Equal(t, "Alex M (alex@test.com)", conv["assignee"])

	// Summary block present
	summary, ok := conv["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), summary["thread_count"])
	assert.Equal(t, float64(90), summary["first_response_minutes"]) // 90 min
	assert.Greater(t, summary["age_days"].(float64), float64(0))
	assert.Equal(t, "reply", summary["last_by"])
	assert.Equal(t, false, summary["awaiting_reply"]) // agent replied last

	// NO threads key in tier 2
	assert.Nil(t, conv["threads"])
}

func TestBriefingAgentWithThreads(t *testing.T) {
	activeConv := `[
		{"id":1,"number":100,"subject":"Help me","status":"active",
		 "createdAt":"2025-01-01T10:00:00Z",
		 "primaryCustomer":{"email":"alice@test.com"},"userUpdatedAt":"2025-01-01",
		 "assignee":{"first":"Alex","last":"Morgan","email":"alex@test.com"},
		 "_embedded":{"threads":[
			{"id":10,"type":"customer","body":"Need help","createdAt":"2025-01-01T10:00:00Z","createdBy":{"email":"alice@test.com"}},
			{"id":11,"type":"reply","body":"On it","createdAt":"2025-01-01T11:30:00Z","createdBy":{"email":"agent@test.com"}}
		 ]}}
	]`
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "threads", params.Get("embed"))
			if params.Get("status") == "active" {
				return halJSON("conversations", activeConv), nil
			}
			return halJSON("conversations", `[]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--assigned-to", "99", "--embed", "threads"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	// Summary line
	assert.Contains(t, out, "Briefing")
	assert.Contains(t, out, "Alex Morgan")
	assert.Contains(t, out, "1 open")
	// Detail view: conversation header
	assert.Contains(t, out, "#100")
	assert.Contains(t, out, "Help me")
	assert.Contains(t, out, "[active]")
	assert.Contains(t, out, "threads:2")
	// Human-readable threads
	assert.Contains(t, out, "[customer]")
	assert.Contains(t, out, "[reply]")
	assert.Contains(t, out, "Need help")
	assert.Contains(t, out, "On it")
	assert.Contains(t, out, "alice@test.com")
	assert.Contains(t, out, "agent@test.com")
}

func TestBriefingAgentThreadsJSON(t *testing.T) {
	activeConv := `[
		{"id":1,"number":100,"subject":"Help me","status":"active",
		 "createdAt":"2025-01-01T10:00:00Z",
		 "primaryCustomer":{"first":"Alice","last":"Smith","email":"alice@test.com"},
		 "assignee":{"first":"Alex","last":"M","email":"alex@test.com"},
		 "tags":[{"id":1,"name":"billing","slug":"billing","color":"#f00"}],
				 "userUpdatedAt":"2025-01-01",
		 "_embedded":{"threads":[
			{"id":10,"type":"customer",
			 "body":"<p>I need help with <strong>billing</strong>. See <a href=\"https://example.com/inv\">invoice</a>.</p>",
			 "createdAt":"2025-01-01T10:00:00Z",
			 "createdBy":{"first":"Alice","last":"Smith","email":"alice@test.com"},
			 "attachments":[{"id":55,"filename":"invoice.pdf","mimeType":"application/pdf","size":43008}],
			 "_embedded":{"attachments":[]},
			 "_links":{"createdByCustomer":{"href":"/customers/1"}}},
			{"id":11,"type":"reply","body":"<p>On it!</p>","createdAt":"2025-01-01T11:30:00Z",
			 "createdBy":{"first":"Alex","last":"M","email":"alex@test.com"}}
		 ]},
		 "_links":{"self":{"href":"/conversations/1"}}}
	]`
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			if params.Get("status") == "active" {
				return halJSON("conversations", activeConv), nil
			}
			return halJSON("conversations", `[]`), nil
		},
	}
	buf := setupTest(mock)
	format = "json"
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--assigned-to", "99", "--embed", "threads", "--format", "json"})
	require.NoError(t, rootCmd.Execute())

	out := buf.String()
	// No HAL noise
	assert.NotContains(t, out, `_embedded`)
	assert.NotContains(t, out, `_links`)

	// Parse and validate structure
	var result []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &result))
	require.Len(t, result, 1)
	conv := result[0]

	// Conversation fields collapsed
	assert.Equal(t, float64(1), conv["id"])
	assert.Equal(t, "Help me", conv["subject"])
	assert.Equal(t, "Alice Smith (alice@test.com)", conv["customer"])
	assert.Equal(t, "Alex M (alex@test.com)", conv["assignee"])

	// Summary block with computed metrics
	summary, ok := conv["summary"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(2), summary["thread_count"])
	assert.Equal(t, true, summary["has_attachments"])
	assert.Equal(t, float64(1), summary["attachment_count"])
	assert.Equal(t, "2025-01-01T11:30:00Z", summary["last_activity"])
	assert.Equal(t, float64(90), summary["first_response_minutes"]) // 90 min gap
	assert.Greater(t, summary["age_days"].(float64), float64(0))
	assert.Equal(t, "reply", summary["last_by"])
	assert.Equal(t, false, summary["awaiting_reply"]) // agent replied last
	byType, ok := summary["threads_by_type"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), byType["customer"])
	assert.Equal(t, float64(1), byType["reply"])

	// Threads collapsed
	threads, ok := conv["threads"].([]any)
	require.True(t, ok)
	require.Len(t, threads, 2)

	t0 := threads[0].(map[string]any)
	assert.Equal(t, "customer", t0["type"])
	assert.Equal(t, "Alice Smith (alice@test.com)", t0["from"])
	// HTML converted to markdown — bold preserved, link preserved
	assert.Contains(t, t0["body"], "**billing**")
	assert.Contains(t, t0["body"], "[invoice](https://example.com/inv)")
	// No raw HTML
	assert.NotContains(t, t0["body"], "<p>")
	assert.NotContains(t, t0["body"], "<strong>")
	// Attachment collapsed with ID kept
	atts, ok := t0["attachments"].([]any)
	require.True(t, ok)
	require.Len(t, atts, 1)
	att := atts[0].(map[string]any)
	assert.Equal(t, float64(55), att["id"])
	assert.Equal(t, "invoice.pdf (42KB)", att["file"])

	// Thread without attachments — no attachments key
	t1 := threads[1].(map[string]any)
	assert.Equal(t, "reply", t1["type"])
	assert.Nil(t, t1["attachments"])
}

func TestBriefingNoResults(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("conversations", `[]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--assigned-to", "99"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "No results.")
}

func TestBriefingEmbedWithoutAssignedTo(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("conversations", `[]`), nil
		},
	}
	setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	// Explicit --assigned-to "" to override cobra flag state from prior tests
	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--embed", "threads", "--assigned-to", ""})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--embed threads requires --assigned-to")
}

// Regression: team overview must not embed threads (large payload for no benefit).
func TestBriefingTeamOverviewNoThreadEmbed(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			// This is the key assertion: embed must be empty for team overview
			assert.Empty(t, params.Get("embed"), "team overview should not request embed=threads")
			assert.Empty(t, params.Get("assigned_to"))
			if params.Get("status") == "active" {
				return halJSON("conversations", `[
					{"id":1,"number":100,"subject":"A","status":"active",
					 "primaryCustomer":{"email":"c1@test.com"},
					 "assignee":{"first":"Alex","last":"M","email":"alex@test.com"}}
				]`), nil
			}
			return halJSON("conversations", `[]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	// Explicit --embed "" to reset cobra flag state from prior tests
	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--embed", ""})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Alex M")
}

// Regression: agent views must always embed threads (summary data depends on it).
func TestBriefingAgentViewsAlwaysEmbedThreads(t *testing.T) {
	mock := &mockClient{
		ListConversationsFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			assert.Equal(t, "threads", params.Get("embed"), "agent views must embed threads")
			if params.Get("status") == "active" {
				return halJSON("conversations", `[
					{"id":1,"number":100,"subject":"A","status":"active",
					 "createdAt":"2025-01-01T10:00:00Z",
					 "primaryCustomer":{"email":"c1@test.com"},
					 "_embedded":{"threads":[
						{"id":10,"type":"customer","body":"Help","createdAt":"2025-01-01T10:00:00Z","createdBy":{"email":"c1@test.com"}}
					 ]}}
				]`), nil
			}
			return halJSON("conversations", `[]`), nil
		},
	}
	buf := setupTest(mock)
	defer func() { output.Out = os.Stdout }()

	// Tier 2: no --embed threads flag, but API should still embed
	rootCmd.SetArgs([]string{"inbox", "tools", "briefing", "--assigned-to", "99"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "A")
}

func TestComputeSummary(t *testing.T) {
	t.Run("empty threads", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		s := computeSummary(c, nil)
		assert.Equal(t, 0, s.ThreadCount)
		assert.Empty(t, s.LastActivity)
		assert.Empty(t, s.LastBy)
		assert.False(t, s.AwaitingReply)
		assert.False(t, s.HasAttachments)
		assert.Nil(t, s.FirstResponseMins)
		assert.NotNil(t, s.AgeDays)
	})

	t.Run("customer only — no first response, awaiting reply", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "customer", CreatedAt: "2025-01-01T11:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, 2, s.ThreadCount)
		assert.Nil(t, s.FirstResponseMins)
		assert.Equal(t, "customer", s.LastBy)
		assert.True(t, s.AwaitingReply)
		assert.Equal(t, 2, s.ThreadsByType["customer"])
	})

	t.Run("mixed threads — first response computed, customer last", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "reply", CreatedAt: "2025-01-01T11:30:00Z"},
			{Type: "customer", CreatedAt: "2025-01-01T12:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, 3, s.ThreadCount)
		require.NotNil(t, s.FirstResponseMins)
		assert.Equal(t, 90.0, *s.FirstResponseMins) // 90 minutes
		assert.Equal(t, "2025-01-01T12:00:00Z", s.LastActivity)
		assert.Equal(t, "customer", s.LastBy)
		assert.True(t, s.AwaitingReply) // customer sent last
		assert.Equal(t, 2, s.ThreadsByType["customer"])
		assert.Equal(t, 1, s.ThreadsByType["reply"])
	})

	t.Run("unparseable createdAt", func(t *testing.T) {
		c := types.Conversation{CreatedAt: ""}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Nil(t, s.FirstResponseMins)
		assert.Nil(t, s.AgeDays)
		assert.Equal(t, 1, s.ThreadCount) // still counts threads
	})

	t.Run("note after reply — still shows reply", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "reply", CreatedAt: "2025-01-01T11:00:00Z"},
			{Type: "note", CreatedAt: "2025-01-01T12:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, "reply", s.LastBy)
		assert.False(t, s.AwaitingReply)
	})

	t.Run("note after customer — still awaiting", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "note", CreatedAt: "2025-01-01T11:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, "customer", s.LastBy)
		assert.True(t, s.AwaitingReply)
	})

	t.Run("lineitem after customer — still awaiting", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "reply", CreatedAt: "2025-01-01T11:00:00Z"},
			{Type: "customer", CreatedAt: "2025-01-01T12:00:00Z"},
			{Type: "lineitem", CreatedAt: "2025-01-01T13:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, "customer", s.LastBy)
		assert.True(t, s.AwaitingReply)
	})

	t.Run("forward types ignored", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "forwardparent", CreatedAt: "2025-01-01T11:00:00Z"},
			{Type: "forwardchild", CreatedAt: "2025-01-01T12:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, "customer", s.LastBy)
		assert.True(t, s.AwaitingReply)
	})

	t.Run("message type counts as agent reply", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z"},
			{Type: "message", CreatedAt: "2025-01-01T11:00:00Z"},
		}
		s := computeSummary(c, threads)
		assert.Equal(t, "message", s.LastBy)
		assert.False(t, s.AwaitingReply)
	})

	t.Run("attachments counted", func(t *testing.T) {
		c := types.Conversation{CreatedAt: "2025-01-01T10:00:00Z"}
		threads := []types.Thread{
			{Type: "customer", CreatedAt: "2025-01-01T10:00:00Z",
				Attachments: []types.Attachment{{ID: 1}, {ID: 2}}},
			{Type: "reply", CreatedAt: "2025-01-01T11:00:00Z",
				Attachments: []types.Attachment{{ID: 3}}},
		}
		s := computeSummary(c, threads)
		assert.True(t, s.HasAttachments)
		assert.Equal(t, 3, s.AttachmentCount)
	})
}
