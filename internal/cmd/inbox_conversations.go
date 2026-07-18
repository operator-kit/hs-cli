package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/pii"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newConversationsCmd() *cobra.Command {
	convCmd := &cobra.Command{
		Use:     "conversations",
		Aliases: []string{"conv"},
		Short:   "Manage conversations",
	}

	listCmd := conversationsListCmd()
	permission.Annotate(listCmd, "conversations", permission.OpRead)
	listCmd.Flags().String("status", "active", "filter by status: active|closed|pending|spam|all")
	listCmd.Flags().String("mailbox", "", "filter by mailbox ID")
	listCmd.Flags().String("folder", "", "filter by folder ID")
	listCmd.Flags().String("tag", "", "filter by tag")
	listCmd.Flags().String("assigned-to", "", "filter by assigned user ID")
	listCmd.Flags().String("modified-since", "", "filter by modified since timestamp")
	listCmd.Flags().String("number", "", "filter by conversation number")
	listCmd.Flags().String("sort-field", "", "sort field")
	listCmd.Flags().String("sort-order", "", "sort order")
	listCmd.Flags().String("custom-fields-by-ids", "", "custom field filters")
	listCmd.Flags().String("query", "", "search query")
	listCmd.Flags().String("embed", "", "embed resources (e.g. threads)")
	markProtectedFlags(listCmd, "custom-fields-by-ids", "query")

	createCmd := conversationsCreateCmd()
	permission.Annotate(createCmd, "conversations", permission.OpWrite)
	createCmd.Flags().String("mailbox", "", "mailbox ID (required)")
	createCmd.Flags().String("subject", "", "conversation subject (required)")
	createCmd.Flags().String("customer", "", "customer email (required)")
	createCmd.Flags().String("body", "", "initial message body (required)")
	createCmd.Flags().String("type", "email", "conversation type")
	createCmd.Flags().String("status", "active", "initial status")
	createCmd.Flags().StringSlice("tags", nil, "tags to apply")
	createCmd.Flags().Int("assign-to", 0, "user ID to assign conversation to")
	createCmd.Flags().String("created-at", "", "creation timestamp")
	createCmd.Flags().Bool("imported", false, "mark conversation as imported")
	createCmd.Flags().Bool("auto-reply", false, "trigger auto-reply behavior")
	createCmd.Flags().StringSlice("field", nil, "custom field assignment in <id>=<value> format (repeatable)")
	markProtectedFlags(createCmd, "subject", "customer", "body", "field")
	createCmd.MarkFlagRequired("mailbox")
	createCmd.MarkFlagRequired("subject")
	createCmd.MarkFlagRequired("customer")
	createCmd.MarkFlagRequired("body")

	updateCmd := conversationsUpdateCmd()
	permission.Annotate(updateCmd, "conversations", permission.OpWrite)
	updateCmd.Flags().String("subject", "", "new subject")
	updateCmd.Flags().String("status", "", "new status")
	markProtectedFlags(updateCmd, "subject")

	getCmd := conversationsGetCmd()
	permission.Annotate(getCmd, "conversations", permission.OpRead)

	deleteCmd := conversationsDeleteCmd()
	permission.Annotate(deleteCmd, "conversations", permission.OpDelete)

	convCmd.AddCommand(
		listCmd,
		getCmd,
		createCmd,
		updateCmd,
		deleteCmd,
		newConversationAttachmentsCmd(),
		newThreadsCmd(),
		newConversationTagsCmd(),
		newConversationFieldsCmd(),
		newConversationSnoozeCmd(),
	)
	return convCmd
}

func conversationsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List conversations",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			if v, _ := cmd.Flags().GetString("status"); v != "" {
				params.Set("status", v)
			}
			if v, _ := cmd.Flags().GetString("mailbox"); v != "" {
				params.Set("mailbox", v)
			} else if cfg != nil && cfg.InboxDefaultMailbox > 0 {
				params.Set("mailbox", strconv.Itoa(cfg.InboxDefaultMailbox))
			}
			if v, _ := cmd.Flags().GetString("folder"); v != "" {
				params.Set("folder", v)
			}
			if v, _ := cmd.Flags().GetString("tag"); v != "" {
				params.Set("tag", v)
			}
			if v, _ := cmd.Flags().GetString("assigned-to"); v != "" {
				params.Set("assigned_to", v)
			}
			if v, _ := cmd.Flags().GetString("modified-since"); v != "" {
				params.Set("modifiedSince", v)
			}
			if v, _ := cmd.Flags().GetString("number"); v != "" {
				params.Set("number", v)
			}
			if v, _ := cmd.Flags().GetString("sort-field"); v != "" {
				params.Set("sortField", v)
			}
			if v, _ := cmd.Flags().GetString("sort-order"); v != "" {
				params.Set("sortOrder", v)
			}
			if v, _ := cmd.Flags().GetString("custom-fields-by-ids"); v != "" {
				params.Set("customFieldsByIds", v)
			}
			if v, _ := cmd.Flags().GetString("query"); v != "" {
				params.Set("query", v)
			}
			if v, _ := cmd.Flags().GetString("embed"); v != "" {
				params.Set("embed", v)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListConversations, params, "conversations", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items), conversationPIIContext)
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanConversation)), conversationPIIContext)
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListConversations, params, "conversations", noPaginate)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}

			var convs []types.Conversation
			for _, raw := range items {
				var c types.Conversation
				json.Unmarshal(raw, &c)
				if engine.Enabled() {
					known := []pii.KnownIdentity{knownFromPerson(c.PrimaryCustomer, "customer")}
					redactPersonForOutput(engine, &c.PrimaryCustomer, "customer")
					if c.Assignee != nil {
						known = append(known, knownFromPerson(*c.Assignee, "user"))
						redactPersonForOutput(engine, c.Assignee, "user")
					}
					c.Subject = redactTextWithPII(engine, c.Subject, known...)
				}
				convs = append(convs, c)
			}

			cols := []string{"id", "number", "subject", "status", "customer", "assigned", "updated"}
			rows := make([]map[string]string, len(convs))
			for i, c := range convs {
				customer := c.PrimaryCustomer.Email
				if customer == "" {
					customer = fmt.Sprintf("%s %s", c.PrimaryCustomer.First, c.PrimaryCustomer.Last)
				}
				assigned := "unassigned"
				if c.Assignee != nil {
					assigned = strings.TrimSpace(c.Assignee.First + " " + c.Assignee.Last)
					if assigned == "" {
						assigned = c.Assignee.Email
					}
				}
				rows[i] = map[string]string{
					"id":       strconv.Itoa(c.ID),
					"number":   strconv.Itoa(c.Number),
					"subject":  truncate(c.Subject, 50),
					"status":   c.Status,
					"customer": customer,
					"assigned": assigned,
					"updated":  output.RelativeTime(c.ModifiedAt),
				}
			}
			if err := output.Print(getFormat(), cols, rows); err != nil {
				return err
			}
			if pageInfo != nil && !noPaginate {
				fmt.Fprintf(output.Out, "%s\n\n", output.Dim(fmt.Sprintf("Page %d of %d (%d total)", pageInfo.Number, pageInfo.TotalPages, pageInfo.TotalElements)))
			}
			return nil
		},
	}
}

func conversationsGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get conversation details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if v, _ := cmd.Flags().GetString("embed"); v != "" {
				params.Set("embed", v)
			}

			data, err := apiClient.GetConversation(context.Background(), args[0], params)
			if err != nil {
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return printRawWithPII(data, conversationPIIContext)
				}
				return printRawWithPII(mustMarshal(cleanRawObject(data, cleanConversation)), conversationPIIContext)
			}

			var c types.Conversation
			json.Unmarshal(data, &c)
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}

			known := []pii.KnownIdentity{knownFromPerson(c.PrimaryCustomer, "customer")}
			if engine.Enabled() {
				redactPersonForOutput(engine, &c.PrimaryCustomer, "customer")
			}

			customer := formatPerson(c.PrimaryCustomer)
			assignee := ""
			if c.Assignee != nil {
				if engine.Enabled() {
					known = append(known, knownFromPerson(*c.Assignee, "user"))
					redactPersonForOutput(engine, c.Assignee, "user")
				}
				assignee = formatPerson(*c.Assignee)
			}
			if engine.Enabled() {
				c.Subject = redactTextWithPII(engine, c.Subject, known...)
				c.Preview = redactTextWithPII(engine, c.Preview, known...)
			}

			tagNames := make([]string, len(c.Tags))
			for i, t := range c.Tags {
				tagNames[i] = t.Name
			}

			source := strings.TrimSpace(c.Source.Type + " " + c.Source.Via)

			// CSV: single-row with expanded columns
			if getFormat() == "csv" {
				cols := []string{"id", "number", "subject", "status", "type", "customer", "assignee", "tags", "created", "updated"}
				rows := []map[string]string{{
					"id":       strconv.Itoa(c.ID),
					"number":   strconv.Itoa(c.Number),
					"subject":  c.Subject,
					"status":   c.Status,
					"type":     c.Type,
					"customer": customer,
					"assignee": assignee,
					"tags":     strings.Join(tagNames, ";"),
					"created":  c.CreatedAt,
					"updated":  c.ModifiedAt,
				}}
				return output.Print("csv", cols, rows)
			}

			// Default: detail view
			fields := []output.Field{
				{Label: "ID", Value: strconv.Itoa(c.ID)},
				{Label: "Number", Value: output.Blue(fmt.Sprintf("#%d", c.Number))},
				{Label: "Subject", Value: c.Subject},
				{Label: "Status", Value: c.Status},
				{Label: "Type", Value: c.Type},
				{Label: "Assignee", Value: assignee},
				{Label: "Customer", Value: customer},
				{Label: "Mailbox", Value: strconv.Itoa(c.MailboxID)},
				{Label: "Tags", Value: strings.Join(tagNames, ", ")},
				{Label: "Source", Value: source},
				{Label: "Created", Value: c.CreatedAt},
				{Label: "Updated", Value: c.ModifiedAt},
				{Label: "Closed", Value: c.ClosedAt},
				{Label: "Preview", Value: c.Preview},
			}
			output.PrintDetail(output.Out, fields)

			// Embedded threads
			threads := parseEmbeddedThreads(data)
			if len(threads) > 0 {
				fmt.Fprintf(output.Out, "\n%s\n", output.Dim(fmt.Sprintf("Threads (%d):", len(threads))))
				fmt.Fprintln(output.Out, output.Dim(strings.Repeat("─", 60)))
				for _, t := range threads {
					originalAuthor := t.CreatedBy
					authorType := threadAuthorType(t.Type)
					if engine.Enabled() {
						redactPersonForOutput(engine, &t.CreatedBy, authorType)
					}
					author := formatPerson(t.CreatedBy)
					fmt.Fprintf(output.Out, "\n[%s] %s — %s\n", t.Type, author, t.CreatedAt)
					body := t.Body
					if body == "" && t.Action.Text != "" {
						body = t.Action.Text
					}
					if body != "" {
						if engine.Enabled() {
							body = redactTextWithPII(engine, body, append(known, knownFromPerson(originalAuthor, authorType))...)
						}
						fmt.Fprintln(output.Out, stripHTMLTags(body))
					}
				}
			}

			return nil
		},
	}
	cmd.Flags().String("embed", "", "embed resources (e.g. threads)")
	return cmd
}

func formatPerson(p types.Person) string {
	if p.Email != "" {
		name := strings.TrimSpace(p.First + " " + p.Last)
		if name != "" {
			return name + " (" + p.Email + ")"
		}
		return p.Email
	}
	return strings.TrimSpace(p.First + " " + p.Last)
}

func parseEmbeddedThreads(data json.RawMessage) []types.Thread {
	var wrapper struct {
		Embedded struct {
			Threads []types.Thread `json:"threads"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil
	}
	return wrapper.Embedded.Threads
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTMLTags(s string) string {
	return strings.TrimSpace(htmlTagRe.ReplaceAllString(s, ""))
}

func conversationsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a conversation",
		RunE: func(cmd *cobra.Command, args []string) error {
			mailbox, _ := cmd.Flags().GetString("mailbox")
			subject, _ := cmd.Flags().GetString("subject")
			customer, _ := cmd.Flags().GetString("customer")
			body, _ := cmd.Flags().GetString("body")
			convType, _ := cmd.Flags().GetString("type")
			status, _ := cmd.Flags().GetString("status")
			tags, _ := cmd.Flags().GetStringSlice("tags")
			assignTo, _ := cmd.Flags().GetInt("assign-to")
			createdAt, _ := cmd.Flags().GetString("created-at")
			imported, _ := cmd.Flags().GetBool("imported")
			autoReply, _ := cmd.Flags().GetBool("auto-reply")
			fieldFlags, _ := cmd.Flags().GetStringSlice("field")

			mailboxID, err := strconv.Atoi(mailbox)
			if err != nil {
				return fmt.Errorf("invalid mailbox ID: %s", mailbox)
			}

			fields, err := parseConversationFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}

			conv := types.ConversationCreate{
				Subject:   subject,
				MailboxID: mailboxID,
				Type:      convType,
				Status:    status,
				Customer:  types.Person{Email: customer},
				Threads: []types.ThreadCreate{{
					Type:     "customer",
					Customer: types.Person{Email: customer},
					Text:     body,
				}},
				Tags: tags,
			}
			if assignTo > 0 {
				conv.AssignTo = assignTo
			}
			if createdAt != "" {
				conv.CreatedAt = createdAt
			}
			if cmd.Flags().Changed("imported") {
				conv.Imported = &imported
			}
			if cmd.Flags().Changed("auto-reply") {
				conv.AutoReply = &autoReply
			}
			if len(fields) > 0 {
				conv.Fields = fields
			}

			id, err := apiClient.CreateConversation(context.Background(), conv)
			if err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created conversation %s\n", id)
			return nil
		},
	}
}

func newConversationTagsCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list <conversation-id>",
		Short: "List conversation tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, err := currentConversationTags(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				return printRawWithPII(mustMarshal(map[string]any{"tags": tags}), conversationPIIContext)
			}
			for _, tag := range tags {
				fmt.Fprintln(output.Out, tag)
			}
			return nil
		},
	}
	permission.Annotate(listCmd, "conversations", permission.OpRead)

	setCmd := &cobra.Command{
		Use:   "set <conversation-id>",
		Short: "Replace all conversation tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, _ := cmd.Flags().GetStringSlice("tag")
			if !cmd.Flags().Changed("tag") {
				return fmt.Errorf("at least one --tag is required")
			}
			tags = normalizeTagNames(tags)

			if err := replaceConversationTags(context.Background(), args[0], tags); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Replaced tags for conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(setCmd, "conversations", permission.OpWrite)
	setCmd.Flags().StringSlice("tag", nil, "tag to apply; replaces all existing tags (repeatable or comma-separated)")
	setCmd.MarkFlagRequired("tag")

	addCmd := &cobra.Command{
		Use:   "add <conversation-id>",
		Short: "Add conversation tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, _ := cmd.Flags().GetStringSlice("tag")
			tags = normalizeTagNames(tags)
			if len(tags) == 0 {
				return fmt.Errorf("at least one --tag is required")
			}

			current, err := currentConversationTags(context.Background(), args[0])
			if err != nil {
				return err
			}
			next := mergeTagNames(current, tags)
			if err := replaceConversationTags(context.Background(), args[0], next); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Added tags for conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(addCmd, "conversations", permission.OpWrite)
	addCmd.Flags().StringSlice("tag", nil, "tag to add while preserving existing tags (repeatable or comma-separated)")
	addCmd.MarkFlagRequired("tag")

	removeCmd := &cobra.Command{
		Use:   "remove <conversation-id>",
		Short: "Remove conversation tags",
		Aliases: []string{
			"rm",
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, _ := cmd.Flags().GetStringSlice("tag")
			tags = normalizeTagNames(tags)
			if len(tags) == 0 {
				return fmt.Errorf("at least one --tag is required")
			}

			current, err := currentConversationTags(context.Background(), args[0])
			if err != nil {
				return err
			}
			next := removeTagNames(current, tags)
			if err := replaceConversationTags(context.Background(), args[0], next); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Removed tags for conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(removeCmd, "conversations", permission.OpWrite)
	removeCmd.Flags().StringSlice("tag", nil, "tag to remove while preserving other tags (repeatable or comma-separated)")
	removeCmd.MarkFlagRequired("tag")

	clearCmd := &cobra.Command{
		Use:   "clear <conversation-id>",
		Short: "Remove all conversation tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := replaceConversationTags(context.Background(), args[0], []string{}); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Cleared tags for conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(clearCmd, "conversations", permission.OpWrite)

	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage conversation tags",
	}
	cmd.AddCommand(listCmd, setCmd, addCmd, removeCmd, clearCmd)
	return cmd
}

func currentConversationTags(ctx context.Context, conversationID string) ([]string, error) {
	data, err := apiClient.GetConversation(ctx, conversationID, nil)
	if err != nil {
		return nil, err
	}

	var conversation types.Conversation
	if err := json.Unmarshal(data, &conversation); err != nil {
		return nil, fmt.Errorf("parsing conversation tags: %w", err)
	}

	tags := make([]string, 0, len(conversation.Tags))
	for _, tag := range conversation.Tags {
		name := strings.TrimSpace(tag.Name)
		if name != "" {
			tags = append(tags, name)
		}
	}
	return tags, nil
}

func replaceConversationTags(ctx context.Context, conversationID string, tags []string) error {
	return apiClient.UpdateConversationTags(ctx, conversationID, map[string]any{"tags": tags})
}

func normalizeTagNames(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized
}

func mergeTagNames(current, added []string) []string {
	next := append([]string{}, current...)
	seen := make(map[string]bool, len(next)+len(added))
	for _, tag := range next {
		seen[tag] = true
	}
	for _, tag := range added {
		if seen[tag] {
			continue
		}
		seen[tag] = true
		next = append(next, tag)
	}
	return next
}

func removeTagNames(current, removed []string) []string {
	remove := make(map[string]bool, len(removed))
	for _, tag := range removed {
		remove[tag] = true
	}

	next := make([]string, 0, len(current))
	for _, tag := range current {
		if !remove[tag] {
			next = append(next, tag)
		}
	}
	return next
}

func newConversationFieldsCmd() *cobra.Command {
	setCmd := &cobra.Command{
		Use:   "set <conversation-id>",
		Short: "Set conversation custom fields",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fieldFlags, _ := cmd.Flags().GetStringSlice("field")
			fields, err := parseConversationFieldAssignments(fieldFlags)
			if err != nil {
				return err
			}
			if len(fields) == 0 {
				return fmt.Errorf("at least one --field is required")
			}

			body := map[string]any{"fields": fields}
			if err := apiClient.UpdateConversationFields(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated custom fields for conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(setCmd, "conversations", permission.OpWrite)
	setCmd.Flags().StringSlice("field", nil, "custom field assignment in <id>=<value> format (repeatable)")
	markProtectedFlags(setCmd, "field")
	setCmd.MarkFlagRequired("field")

	cmd := &cobra.Command{
		Use:   "fields",
		Short: "Manage conversation custom fields",
	}
	cmd.AddCommand(setCmd)
	return cmd
}

func newConversationSnoozeCmd() *cobra.Command {
	setCmd := &cobra.Command{
		Use:   "set <conversation-id>",
		Short: "Snooze a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			until, _ := cmd.Flags().GetString("until")
			body := map[string]any{"snoozedUntil": until}
			if err := apiClient.UpdateConversationSnooze(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Snoozed conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(setCmd, "conversations", permission.OpWrite)
	setCmd.Flags().String("until", "", "snooze-until timestamp (required)")
	setCmd.MarkFlagRequired("until")

	clearCmd := &cobra.Command{
		Use:   "clear <conversation-id>",
		Short: "Clear conversation snooze",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteConversationSnooze(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Cleared snooze for conversation %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(clearCmd, "conversations", permission.OpWrite)

	cmd := &cobra.Command{
		Use:   "snooze",
		Short: "Manage conversation snooze",
	}
	cmd.AddCommand(setCmd, clearCmd)
	return cmd
}

func parseConversationFieldAssignments(entries []string) ([]types.ConversationField, error) {
	fields := make([]types.ConversationField, 0, len(entries))
	for _, entry := range entries {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid --field value: expected <id>=<value>")
		}
		fieldID, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid --field id: must be an integer")
		}
		fields = append(fields, types.ConversationField{
			ID:    fieldID,
			Value: parts[1],
		})
	}
	return fields, nil
}

func conversationsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := make([]jsonPatchOp, 0, 2)
			if v, _ := cmd.Flags().GetString("subject"); v != "" {
				body = append(body, jsonPatchOp{
					Op:    "replace",
					Path:  "/subject",
					Value: v,
				})
			}
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				body = append(body, jsonPatchOp{
					Op:    "replace",
					Path:  "/status",
					Value: v,
				})
			}
			if len(body) == 0 {
				return fmt.Errorf("no fields to update")
			}
			if err := apiClient.UpdateConversation(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated conversation %s\n", args[0])
			return nil
		},
	}
}

func conversationsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteConversation(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted conversation %s\n", args[0])
			return nil
		},
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
