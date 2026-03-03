package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newSavedRepliesCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List saved replies",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			if v, _ := cmd.Flags().GetString("mailbox-id"); v != "" {
				params.Set("mailboxId", v)
			}
			if v, _ := cmd.Flags().GetString("query"); v != "" {
				params.Set("query", v)
			}

			data, err := apiClient.ListSavedReplies(context.Background(), params)
			if err != nil {
				return err
			}
			items, err := extractEmbeddedWithCandidates(data, "savedReplies", "saved-replies")
			if err != nil {
				if isJSON() {
					return output.PrintRaw(data)
				}
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, cleanSavedReply)))
			}

			replies := make([]types.SavedReply, 0, len(items))
			for _, raw := range items {
				var r types.SavedReply
				json.Unmarshal(raw, &r)
				replies = append(replies, r)
			}

			rows := make([]map[string]string, len(replies))
			for i, r := range replies {
				rows[i] = map[string]string{
					"id":      strconv.Itoa(r.ID),
					"name":    r.Name,
					"subject": r.Subject,
					"private": strconv.FormatBool(r.IsPrivate),
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "subject", "private"}, rows)
		},
	}
	listCmd.Flags().String("mailbox-id", "", "filter by mailbox ID")
	listCmd.Flags().String("query", "", "search query")

	getCmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a saved reply",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetSavedReply(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				if !isJSONClean() {
					return output.PrintRaw(data)
				}
				return output.PrintRaw(mustMarshal(cleanRawObject(data, cleanSavedReply)))
			}

			var r types.SavedReply
			json.Unmarshal(data, &r)
			return output.Print(getFormat(), []string{"id", "name", "subject", "text", "private"}, []map[string]string{{
				"id":      strconv.Itoa(r.ID),
				"name":    r.Name,
				"subject": r.Subject,
				"text":    truncate(r.Text, 80),
				"private": strconv.FormatBool(r.IsPrivate),
			}})
		},
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a saved reply",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := savedReplyBodyFromFlags(cmd, true)
			if err != nil {
				return err
			}

			id, err := apiClient.CreateSavedReply(context.Background(), body)
			if err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Created saved reply %s\n", id)
			return nil
		},
	}
	savedReplyCreateUpdateFlags(createCmd)
	createCmd.MarkFlagRequired("mailbox-id")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("body")

	updateCmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a saved reply",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := savedReplyBodyFromFlags(cmd, false)
			if err != nil {
				return err
			}
			if len(body) == 0 {
				return fmt.Errorf("no fields to update")
			}

			if err := apiClient.UpdateSavedReply(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated saved reply %s\n", args[0])
			return nil
		},
	}
	savedReplyCreateUpdateFlags(updateCmd)

	deleteCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a saved reply",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.DeleteSavedReply(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted saved reply %s\n", args[0])
			return nil
		},
	}

	permission.Annotate(listCmd, "saved-replies", permission.OpRead)
	permission.Annotate(getCmd, "saved-replies", permission.OpRead)
	permission.Annotate(createCmd, "saved-replies", permission.OpWrite)
	permission.Annotate(updateCmd, "saved-replies", permission.OpWrite)
	permission.Annotate(deleteCmd, "saved-replies", permission.OpDelete)

	cmd := &cobra.Command{
		Use:   "saved-replies",
		Short: "Manage saved replies",
	}
	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}

func savedReplyCreateUpdateFlags(cmd *cobra.Command) {
	cmd.Flags().String("mailbox-id", "", "mailbox ID")
	cmd.Flags().String("name", "", "saved reply name")
	cmd.Flags().String("subject", "", "saved reply subject")
	cmd.Flags().String("body", "", "saved reply body text")
	cmd.Flags().Bool("private", false, "mark saved reply as private")
	cmd.Flags().String("json", "", "full request body as JSON object")
}

func savedReplyBodyFromFlags(cmd *cobra.Command, isCreate bool) (map[string]any, error) {
	if raw, _ := cmd.Flags().GetString("json"); raw != "" {
		body, err := parseJSONBody(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid --json payload: %w", err)
		}
		return body, nil
	}

	body := map[string]any{}

	if v, _ := cmd.Flags().GetString("mailbox-id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid --mailbox-id value %q", v)
		}
		body["mailboxId"] = id
	}
	if v, _ := cmd.Flags().GetString("name"); v != "" {
		body["name"] = v
	}
	if v, _ := cmd.Flags().GetString("subject"); v != "" {
		body["subject"] = v
	}
	if v, _ := cmd.Flags().GetString("body"); v != "" {
		body["text"] = v
	}
	if cmd.Flags().Changed("private") {
		private, _ := cmd.Flags().GetBool("private")
		body["isPrivate"] = private
	}

	if isCreate {
		if _, ok := body["mailboxId"]; !ok {
			return nil, fmt.Errorf("--mailbox-id is required")
		}
		if _, ok := body["name"]; !ok {
			return nil, fmt.Errorf("--name is required")
		}
		if _, ok := body["text"]; !ok {
			return nil, fmt.Errorf("--body is required")
		}
	}
	return body, nil
}
