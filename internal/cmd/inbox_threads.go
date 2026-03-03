package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newThreadsCmd() *cobra.Command {
	// "conversations threads" subcommand group
	threadsCmd := &cobra.Command{
		Use:   "threads",
		Short: "Manage conversation threads",
	}

	replyCmd := threadsReplyCmd()
	permission.Annotate(replyCmd, "conversations", permission.OpWrite)
	replyCmd.Flags().String("customer", "", "customer email (required)")
	replyCmd.Flags().String("body", "", "reply body (required)")
	replyCmd.Flags().String("status", "", "set conversation status after reply")
	replyCmd.Flags().Int("user-id", 0, "user ID for the reply author")
	replyCmd.Flags().StringSlice("to", nil, "recipient emails")
	replyCmd.Flags().StringSlice("cc", nil, "cc recipient emails")
	replyCmd.Flags().StringSlice("bcc", nil, "bcc recipient emails")
	replyCmd.Flags().Bool("draft", false, "create as draft")
	replyCmd.Flags().Bool("imported", false, "mark as imported")
	replyCmd.Flags().String("created-at", "", "thread creation timestamp")
	replyCmd.Flags().String("type", "", "reply thread type")
	replyCmd.Flags().IntSlice("attachment-id", nil, "attachment IDs")
	replyCmd.MarkFlagRequired("customer")
	replyCmd.MarkFlagRequired("body")

	noteCmd := threadsNoteCmd()
	permission.Annotate(noteCmd, "conversations", permission.OpWrite)
	noteCmd.Flags().String("body", "", "note body (required)")
	noteCmd.Flags().Int("user-id", 0, "user ID for the note author")
	noteCmd.Flags().String("status", "", "set conversation status after note")
	noteCmd.Flags().IntSlice("attachment-id", nil, "attachment IDs")
	noteCmd.MarkFlagRequired("body")

	chatCmd := threadsCreateVariantCmd(
		"create-chat",
		"Create a chat thread on a conversation",
		func(ctx context.Context, convID string, body any) error {
			return apiClient.CreateChatThread(ctx, convID, body)
		},
	)
	permission.Annotate(chatCmd, "conversations", permission.OpWrite)
	customerCmd := threadsCreateVariantCmd(
		"create-customer",
		"Create a customer thread on a conversation",
		func(ctx context.Context, convID string, body any) error {
			return apiClient.CreateCustomerThread(ctx, convID, body)
		},
	)
	permission.Annotate(customerCmd, "conversations", permission.OpWrite)
	phoneCmd := threadsCreateVariantCmd(
		"create-phone",
		"Create a phone thread on a conversation",
		func(ctx context.Context, convID string, body any) error {
			return apiClient.CreatePhoneThread(ctx, convID, body)
		},
	)
	permission.Annotate(phoneCmd, "conversations", permission.OpWrite)

	updateCmd := threadsUpdateCmd()
	permission.Annotate(updateCmd, "conversations", permission.OpWrite)
	updateCmd.Flags().String("text", "", "updated thread body text")
	updateCmd.Flags().String("status", "", "updated thread status")

	listCmd := threadsListCmd()
	permission.Annotate(listCmd, "conversations", permission.OpRead)

	sourceCmd := threadsSourceCmd()
	permission.Annotate(sourceCmd, "conversations", permission.OpRead)

	sourceRFC822Cmd := threadsSourceRFC822Cmd()
	permission.Annotate(sourceRFC822Cmd, "conversations", permission.OpRead)

	threadsCmd.AddCommand(
		listCmd,
		replyCmd,
		noteCmd,
		chatCmd,
		customerCmd,
		phoneCmd,
		updateCmd,
		sourceCmd,
		sourceRFC822Cmd,
	)
	return threadsCmd
}

func threadsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <conversation-id>",
		Short: "List threads for a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, func(ctx context.Context, p url.Values) (json.RawMessage, error) {
					return apiClient.ListThreads(ctx, args[0], p)
				}, params, "threads", true)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return printRawWithPII(mustMarshal(items))
				}
				return printRawWithPII(mustMarshal(cleanRawItems(items, cleanThread)))
			}

			items, _, err := api.PaginateAll(ctx, func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return apiClient.ListThreads(ctx, args[0], p)
			}, params, "threads", true)
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}

			var threads []types.Thread
			for _, raw := range items {
				var t types.Thread
				json.Unmarshal(raw, &t)
				if engine.Enabled() {
					author := t.CreatedBy
					authorType := threadAuthorType(t.Type)
					redactPersonForOutput(engine, &t.CreatedBy, authorType)
					body := t.Body
					if body == "" && t.Action.Text != "" {
						body = t.Action.Text
					}
					body = redactTextWithPII(engine, body, knownFromPerson(author, authorType))
					if t.Body != "" {
						t.Body = body
					} else if t.Action.Text != "" {
						t.Action.Text = body
					}
				}
				threads = append(threads, t)
			}

			cols := []string{"id", "type", "created_by", "created_at", "body"}
			rows := make([]map[string]string, len(threads))
			for i, t := range threads {
				createdBy := t.CreatedBy.Email
				if createdBy == "" {
					createdBy = fmt.Sprintf("%s %s", t.CreatedBy.First, t.CreatedBy.Last)
				}
				body := t.Body
				if body == "" && t.Action.Text != "" {
					body = t.Action.Text
				}
				rows[i] = map[string]string{
					"id":         strconv.Itoa(t.ID),
					"type":       t.Type,
					"created_by": createdBy,
					"created_at": t.CreatedAt,
					"body":       truncate(body, 80),
				}
			}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func threadsReplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reply <conversation-id>",
		Short: "Reply to a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			customer, _ := cmd.Flags().GetString("customer")
			body, _ := cmd.Flags().GetString("body")
			status, _ := cmd.Flags().GetString("status")
			userID, _ := cmd.Flags().GetInt("user-id")
			toEmails, _ := cmd.Flags().GetStringSlice("to")
			ccEmails, _ := cmd.Flags().GetStringSlice("cc")
			bccEmails, _ := cmd.Flags().GetStringSlice("bcc")
			draft, _ := cmd.Flags().GetBool("draft")
			imported, _ := cmd.Flags().GetBool("imported")
			createdAt, _ := cmd.Flags().GetString("created-at")
			threadType, _ := cmd.Flags().GetString("type")
			attachmentIDs, _ := cmd.Flags().GetIntSlice("attachment-id")

			reply := types.ReplyBody{
				Customer: types.Person{Email: customer},
				Text:     body,
				Status:   status,
			}
			if userID > 0 {
				reply.User = userID
			}
			if len(toEmails) > 0 {
				reply.To = emailsToPeople(toEmails)
			}
			if len(ccEmails) > 0 {
				reply.CC = emailsToPeople(ccEmails)
			}
			if len(bccEmails) > 0 {
				reply.BCC = emailsToPeople(bccEmails)
			}
			if cmd.Flags().Changed("draft") {
				reply.Draft = &draft
			}
			if cmd.Flags().Changed("imported") {
				reply.Imported = &imported
			}
			if createdAt != "" {
				reply.CreatedAt = createdAt
			}
			if threadType != "" {
				reply.Type = threadType
			}
			if len(attachmentIDs) > 0 {
				reply.Attachments = attachmentIDs
			}

			if err := apiClient.CreateReply(context.Background(), args[0], reply); err != nil {
				return err
			}
			fmt.Fprintln(output.Out, "Reply sent.")
			return nil
		},
	}
}

func threadsNoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "note <conversation-id>",
		Short: "Add a note to a conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, _ := cmd.Flags().GetString("body")
			userID, _ := cmd.Flags().GetInt("user-id")
			status, _ := cmd.Flags().GetString("status")
			attachmentIDs, _ := cmd.Flags().GetIntSlice("attachment-id")

			note := types.NoteBody{
				Text:   body,
				Status: status,
			}
			if userID > 0 {
				note.User = userID
			}
			if len(attachmentIDs) > 0 {
				note.Attachments = attachmentIDs
			}

			if err := apiClient.CreateNote(context.Background(), args[0], note); err != nil {
				return err
			}
			fmt.Fprintln(output.Out, "Note added.")
			return nil
		},
	}
}

func threadsCreateVariantCmd(use string, short string, createFn func(ctx context.Context, convID string, body any) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   use + " <conversation-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := threadCreateBodyFromFlags(cmd)
			if err != nil {
				return err
			}
			if err := createFn(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created %s thread on conversation %s.\n", use, args[0])
			return nil
		},
	}
	cmd.Flags().String("customer", "", "customer email")
	cmd.Flags().String("body", "", "thread body (required)")
	cmd.Flags().Bool("imported", false, "mark as imported")
	cmd.Flags().String("created-at", "", "thread creation timestamp")
	cmd.Flags().IntSlice("attachment-id", nil, "attachment IDs")
	cmd.MarkFlagRequired("body")
	return cmd
}

func threadCreateBodyFromFlags(cmd *cobra.Command) (types.ThreadCreateBody, error) {
	customer, _ := cmd.Flags().GetString("customer")
	body, _ := cmd.Flags().GetString("body")
	imported, _ := cmd.Flags().GetBool("imported")
	createdAt, _ := cmd.Flags().GetString("created-at")
	attachmentIDs, _ := cmd.Flags().GetIntSlice("attachment-id")

	req := types.ThreadCreateBody{
		Text: body,
	}
	if customer != "" {
		req.Customer = &types.Person{Email: customer}
	}
	if cmd.Flags().Changed("imported") {
		req.Imported = &imported
	}
	if createdAt != "" {
		req.CreatedAt = createdAt
	}
	if len(attachmentIDs) > 0 {
		req.Attachments = attachmentIDs
	}
	return req, nil
}

func threadsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <conversation-id> <thread-id>",
		Short: "Update a thread",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			patches := make([]jsonPatchOp, 0, 2)
			if v, _ := cmd.Flags().GetString("text"); v != "" {
				patches = append(patches, jsonPatchOp{
					Op:    "replace",
					Path:  "/body",
					Value: v,
				})
			}
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				patches = append(patches, jsonPatchOp{
					Op:    "replace",
					Path:  "/status",
					Value: v,
				})
			}
			if len(patches) == 0 {
				return fmt.Errorf("no fields to update")
			}
			if err := apiClient.UpdateThread(context.Background(), args[0], args[1], patches); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated thread %s on conversation %s.\n", args[1], args[0])
			return nil
		},
	}
}

func threadsSourceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "source <conversation-id> <thread-id>",
		Short: "Get original thread source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetThreadSource(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}
			if isJSON() {
				return printRawWithPII(data)
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			source := string(data)
			if engine.Enabled() {
				source = engine.RedactText(source, nil)
			}
			return output.Print("table", []string{"conversation_id", "thread_id", "source"}, []map[string]string{{
				"conversation_id": args[0],
				"thread_id":       args[1],
				"source":          truncate(source, 120),
			}})
		},
	}
}

func threadsSourceRFC822Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "source-rfc822 <conversation-id> <thread-id>",
		Short: "Get original thread source in RFC822 format",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetThreadSourceRFC822(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}
			engine, err := newPIIEngine()
			if err != nil {
				return err
			}
			out := string(data)
			if engine.Enabled() {
				out = engine.RedactText(out, nil)
			}
			fmt.Fprintln(output.Out, out)
			return nil
		},
	}
}

func emailsToPeople(emails []string) []types.Person {
	people := make([]types.Person, 0, len(emails))
	for _, email := range emails {
		people = append(people, types.Person{Email: email})
	}
	return people
}
