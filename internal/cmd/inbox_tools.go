package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/pii"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newToolsCmd() *cobra.Command {
	toolsCmd := &cobra.Command{
		Use:   "tools",
		Short: "Workflow tools (non-API commands)",
	}

	briefingCmd := briefingCmd()
	permission.Annotate(briefingCmd, "conversations", permission.OpRead)
	briefingCmd.Flags().String("assigned-to", "", "filter by assigned user ID")
	briefingCmd.Flags().String("status", "active", "conversation status filter")
	briefingCmd.Flags().String("embed", "", "embed sub-resources (e.g. threads)")

	toolsCmd.AddCommand(briefingCmd)
	return toolsCmd
}

func briefingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "briefing",
		Short: "Conversation briefing with optional thread data",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			assignedTo, _ := cmd.Flags().GetString("assigned-to")
			status, _ := cmd.Flags().GetString("status")
			embed, _ := cmd.Flags().GetString("embed")

			embedThreads := strings.Contains(embed, "threads")
			if embedThreads && assignedTo == "" {
				return fmt.Errorf("--embed threads requires --assigned-to")
			}

			params := url.Values{}
			params.Set("status", status)
			if assignedTo != "" {
				params.Set("assigned_to", assignedTo)
				params.Set("embed", "threads") // free in single API call; team overview skips to avoid large payloads
			}

			items, _, err := api.PaginateAll(ctx, apiClient.ListConversations, params, "conversations", true)
			if err != nil {
				return err
			}

			if assignedTo == "" {
				return renderTeamOverview(items)
			}
			if embedThreads {
				return renderAgentWithThreads(items)
			}
			return renderAgentSummary(items)
		},
	}
}

// renderTeamOverview groups conversations by assignee and shows counts.
func renderTeamOverview(items []json.RawMessage) error {
	engine, err := newPIIEngine()
	if err != nil {
		return err
	}

	type agentCount struct {
		name  string
		email string
		count int
	}

	counts := map[string]*agentCount{}
	var unassigned int

	for _, raw := range items {
		var c types.Conversation
		json.Unmarshal(raw, &c)
		if c.Assignee == nil {
			unassigned++
			continue
		}
		redactPersonForOutput(engine, c.Assignee, "user")
		key := c.Assignee.Email
		if key == "" {
			key = strings.TrimSpace(c.Assignee.First + " " + c.Assignee.Last)
		}
		if ac, ok := counts[key]; ok {
			ac.count++
		} else {
			name := strings.TrimSpace(c.Assignee.First + " " + c.Assignee.Last)
			counts[key] = &agentCount{name: name, email: c.Assignee.Email, count: 1}
		}
	}

	// Sort agents by count descending
	agents := make([]*agentCount, 0, len(counts))
	for _, ac := range counts {
		agents = append(agents, ac)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].count > agents[j].count })

	if isJSON() {
		result := make([]map[string]any, 0, len(agents)+1)
		for _, ac := range agents {
			result = append(result, map[string]any{"agent": ac.name, "email": ac.email, "count": ac.count})
		}
		if unassigned > 0 {
			result = append(result, map[string]any{"agent": "(unassigned)", "email": "", "count": unassigned})
		}
		return printRawWithPII(mustMarshal(result))
	}

	rows := make([]map[string]string, 0, len(agents)+1)
	for _, ac := range agents {
		rows = append(rows, map[string]string{
			"agent": ac.name,
			"email": ac.email,
			"count": strconv.Itoa(ac.count),
		})
	}
	if unassigned > 0 {
		rows = append(rows, map[string]string{
			"agent": "(unassigned)",
			"email": "",
			"count": strconv.Itoa(unassigned),
		})
	}

	if len(rows) == 0 {
		fmt.Fprintln(output.Out, "No results.")
		return nil
	}

	cols := []string{"agent", "email", "count"}
	if err := output.Print(getFormat(), cols, rows); err != nil {
		return err
	}
	fmt.Fprintf(output.Out, "\n%d conversations\n", len(items))
	return nil
}

// renderAgentSummary shows a conversation list with thread summary data.
func renderAgentSummary(items []json.RawMessage) error {
	if len(items) == 0 {
		fmt.Fprintln(output.Out, "No results.")
		return nil
	}
	engine, err := newPIIEngine()
	if err != nil {
		return err
	}

	if isJSON() {
		cleaned := make([]map[string]any, len(items))
		for i, raw := range items {
			var c types.Conversation
			json.Unmarshal(raw, &c)
			threads := parseEmbeddedThreads(raw)
			summary := computeSummary(c, threads)
			if engine.Enabled() {
				known := []pii.KnownIdentity{knownFromPerson(c.PrimaryCustomer, "customer")}
				redactPersonForOutput(engine, &c.PrimaryCustomer, "customer")
				if c.Assignee != nil {
					known = append(known, knownFromPerson(*c.Assignee, "user"))
					redactPersonForOutput(engine, c.Assignee, "user")
				}
				c.Subject = redactTextWithPII(engine, c.Subject, known...)
			}

			cleaned[i] = map[string]any{
				"id":       c.ID,
				"number":   c.Number,
				"subject":  c.Subject,
				"status":   c.Status,
				"customer": formatPerson(c.PrimaryCustomer),
				"assignee": formatPersonPtr(c.Assignee),
				"tags":     c.Tags,
				"summary":  summary.toMap(),
			}
		}
		return printRawWithPII(mustMarshal(cleaned))
	}

	cols := []string{"id", "number", "subject", "status", "customer", "threads", "last_activity", "response_min", "age_days"}
	rows := make([]map[string]string, len(items))
	for i, raw := range items {
		var c types.Conversation
		json.Unmarshal(raw, &c)
		threads := parseEmbeddedThreads(raw)
		summary := computeSummary(c, threads)
		if engine.Enabled() {
			known := []pii.KnownIdentity{knownFromPerson(c.PrimaryCustomer, "customer")}
			redactPersonForOutput(engine, &c.PrimaryCustomer, "customer")
			if c.Assignee != nil {
				known = append(known, knownFromPerson(*c.Assignee, "user"))
			}
			c.Subject = redactTextWithPII(engine, c.Subject, known...)
		}

		customer := c.PrimaryCustomer.Email
		if customer == "" {
			customer = strings.TrimSpace(c.PrimaryCustomer.First + " " + c.PrimaryCustomer.Last)
		}
		responseMin := "-"
		if summary.FirstResponseMins != nil {
			responseMin = fmt.Sprintf("%.0f", *summary.FirstResponseMins)
		}
		ageDays := "-"
		if summary.AgeDays != nil {
			ageDays = fmt.Sprintf("%.1f", *summary.AgeDays)
		}

		rows[i] = map[string]string{
			"id":            strconv.Itoa(c.ID),
			"number":        strconv.Itoa(c.Number),
			"subject":       truncate(c.Subject, 50),
			"status":        c.Status,
			"customer":      customer,
			"threads":       strconv.Itoa(summary.ThreadCount),
			"last_activity": summary.LastActivity,
			"response_min":  responseMin,
			"age_days":      ageDays,
		}
	}
	fmt.Fprintf(os.Stderr, "%d conversations\n", len(items))
	return output.Print(getFormat(), cols, rows)
}

// renderAgentWithThreads shows conversations with full thread content.
func renderAgentWithThreads(items []json.RawMessage) error {
	if len(items) == 0 {
		fmt.Fprintln(output.Out, "No results.")
		return nil
	}
	engine, err := newPIIEngine()
	if err != nil {
		return err
	}

	if isJSON() {
		cleaned := make([]map[string]any, len(items))
		for i, raw := range items {
			var c types.Conversation
			json.Unmarshal(raw, &c)
			threads := parseEmbeddedThreads(raw)
			summary := computeSummary(c, threads)
			baseKnown := []pii.KnownIdentity{knownFromPerson(c.PrimaryCustomer, "customer")}
			if engine.Enabled() {
				redactPersonForOutput(engine, &c.PrimaryCustomer, "customer")
				if c.Assignee != nil {
					baseKnown = append(baseKnown, knownFromPerson(*c.Assignee, "user"))
					redactPersonForOutput(engine, c.Assignee, "user")
				}
				c.Subject = redactTextWithPII(engine, c.Subject, baseKnown...)
			}

			// Collapse threads: HTML→markdown, flatten createdBy, compact attachments
			collapsedThreads := make([]map[string]any, len(threads))
			for j, t := range threads {
				originalAuthor := t.CreatedBy
				authorType := threadAuthorType(t.Type)
				body, err := htmltomarkdown.ConvertString(t.Body)
				if err != nil {
					body = stripHTMLTags(t.Body)
				}
				if engine.Enabled() {
					redactPersonForOutput(engine, &t.CreatedBy, authorType)
					body = redactTextWithPII(engine, strings.TrimSpace(body), append(baseKnown, knownFromPerson(originalAuthor, authorType))...)
				}

				ct := map[string]any{
					"id":        t.ID,
					"type":      t.Type,
					"from":      formatPerson(t.CreatedBy),
					"body":      strings.TrimSpace(body),
					"createdAt": t.CreatedAt,
				}
				if len(t.Attachments) > 0 {
					atts := make([]map[string]any, len(t.Attachments))
					for k, a := range t.Attachments {
						atts[k] = map[string]any{
							"id":   a.ID,
							"file": fmt.Sprintf("%s (%s)", a.FileName, formatBytes(a.Size)),
						}
					}
					ct["attachments"] = atts
				}
				collapsedThreads[j] = ct
			}

			cleaned[i] = map[string]any{
				"id":       c.ID,
				"number":   c.Number,
				"subject":  c.Subject,
				"status":   c.Status,
				"customer": formatPerson(c.PrimaryCustomer),
				"assignee": formatPersonPtr(c.Assignee),
				"tags":     c.Tags,
				"summary":  summary.toMap(),
				"threads":  collapsedThreads,
			}
		}
		return printRawWithPII(mustMarshal(cleaned))
	}

	// Detail view: conversation header + human-readable threads
	fmt.Fprintf(os.Stderr, "%d conversations\n", len(items))
	for _, raw := range items {
		var c types.Conversation
		json.Unmarshal(raw, &c)
		threads := parseEmbeddedThreads(raw)
		summary := computeSummary(c, threads)
		baseKnown := []pii.KnownIdentity{knownFromPerson(c.PrimaryCustomer, "customer")}
		if engine.Enabled() {
			redactPersonForOutput(engine, &c.PrimaryCustomer, "customer")
			if c.Assignee != nil {
				baseKnown = append(baseKnown, knownFromPerson(*c.Assignee, "user"))
				redactPersonForOutput(engine, c.Assignee, "user")
			}
			c.Subject = redactTextWithPII(engine, c.Subject, baseKnown...)
		}

		customer := c.PrimaryCustomer.Email
		if customer == "" {
			customer = strings.TrimSpace(c.PrimaryCustomer.First + " " + c.PrimaryCustomer.Last)
		}
		responseMin := "-"
		if summary.FirstResponseMins != nil {
			responseMin = fmt.Sprintf("%.0fm", *summary.FirstResponseMins)
		}
		ageDays := "-"
		if summary.AgeDays != nil {
			ageDays = fmt.Sprintf("%.1fd", *summary.AgeDays)
		}

		fmt.Fprintf(output.Out, "\n#%d %s [%s] — %s | threads:%d response:%s age:%s\n",
			c.Number, truncate(c.Subject, 50), c.Status, customer,
			summary.ThreadCount, responseMin, ageDays)
		fmt.Fprintln(output.Out, strings.Repeat("─", 60))

		for _, t := range threads {
			originalAuthor := t.CreatedBy
			authorType := threadAuthorType(t.Type)
			if engine.Enabled() {
				redactPersonForOutput(engine, &t.CreatedBy, authorType)
			}
			author := formatPerson(t.CreatedBy)
			fmt.Fprintf(output.Out, "\n  [%s] %s — %s\n", t.Type, author, t.CreatedAt)
			body := t.Body
			if body == "" && t.Action.Text != "" {
				body = t.Action.Text
			}
			if body != "" {
				if engine.Enabled() {
					body = redactTextWithPII(engine, body, append(baseKnown, knownFromPerson(originalAuthor, authorType))...)
				}
				fmt.Fprintf(output.Out, "  %s\n", stripHTMLTags(body))
			}
		}
	}
	return nil
}

// Customer-facing thread types (visible to customer).
// Full HelpScout type list: beaconchat, chat, customer, forwardchild, forwardparent, lineitem, message, note, phone
// Excluded: note (internal), lineitem (system event), forwardparent/forwardchild (routing)
var customerFacingTypes = map[string]bool{
	"customer":   true,
	"reply":      true,
	"message":    true, // agent-initiated (not a reply)
	"chat":       true,
	"phone":      true,
	"beaconchat": true, // Beacon widget chat
}

// conversationSummary holds pre-computed metrics from embedded thread data.
type conversationSummary struct {
	ThreadCount       int
	ThreadsByType     map[string]int
	LastActivity      string
	LastBy            string // type of last thread (e.g. "customer", "reply", "note")
	AwaitingReply     bool   // true when last thread is from customer
	HasAttachments    bool
	AttachmentCount   int
	FirstResponseMins *float64 // nil if no non-customer thread
	AgeDays           *float64 // nil if createdAt unparseable
}

func computeSummary(c types.Conversation, threads []types.Thread) conversationSummary {
	s := conversationSummary{
		ThreadCount:   len(threads),
		ThreadsByType: map[string]int{},
	}
	for _, t := range threads {
		s.ThreadsByType[t.Type]++
		s.LastActivity = t.CreatedAt
		if customerFacingTypes[t.Type] {
			s.LastBy = t.Type
		}
		if len(t.Attachments) > 0 {
			s.HasAttachments = true
			s.AttachmentCount += len(t.Attachments)
		}
	}
	s.AwaitingReply = s.LastBy == "customer"

	convCreated := parseISO(c.CreatedAt)
	if convCreated != nil {
		// first_response_minutes: time to first non-customer thread
		for _, t := range threads {
			if t.Type != "customer" {
				if tc := parseISO(t.CreatedAt); tc != nil {
					mins := tc.Sub(*convCreated).Minutes()
					s.FirstResponseMins = &mins
				}
				break
			}
		}
		// age_days
		days := time.Since(*convCreated).Hours() / 24
		s.AgeDays = &days
	}
	return s
}

func (s conversationSummary) toMap() map[string]any {
	m := map[string]any{
		"thread_count":     s.ThreadCount,
		"threads_by_type":  s.ThreadsByType,
		"last_activity":    s.LastActivity,
		"last_by":          s.LastBy,
		"awaiting_reply":   s.AwaitingReply,
		"has_attachments":  s.HasAttachments,
		"attachment_count": s.AttachmentCount,
	}
	if s.FirstResponseMins != nil {
		m["first_response_minutes"] = math.Round(*s.FirstResponseMins*100) / 100
	}
	if s.AgeDays != nil {
		m["age_days"] = math.Round(*s.AgeDays*100) / 100
	}
	return m
}

func parseISO(s string) *time.Time {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func formatPersonPtr(p *types.Person) string {
	if p == nil {
		return ""
	}
	return formatPerson(*p)
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%dKB", b>>10)
	default:
		return fmt.Sprintf("%dB", b)
	}
}
