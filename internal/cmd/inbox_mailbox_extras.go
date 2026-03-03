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
)

func newMailboxFoldersCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list <mailbox-id>",
		Short: "List folders for a mailbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			data, err := apiClient.ListMailboxFolders(context.Background(), args[0], params)
			if err != nil {
				return err
			}

			items, err := extractEmbeddedWithCandidates(data, "folders")
			if err != nil {
				if isJSON() {
					return output.PrintRaw(data)
				}
				return err
			}
			if isJSON() {
				return output.PrintRaw(mustMarshal(items))
			}

			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var row map[string]any
				json.Unmarshal(raw, &row)
				rows[i] = map[string]string{
					"id":   fmt.Sprintf("%v", row["id"]),
					"name": fmt.Sprintf("%v", row["name"]),
					"type": fmt.Sprintf("%v", row["type"]),
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "type"}, rows)
		},
	}

	permission.Annotate(listCmd, "mailboxes", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage mailbox folders",
	}
	cmd.AddCommand(listCmd)
	return cmd
}

func newMailboxCustomFieldsCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list <mailbox-id>",
		Short: "List custom fields for a mailbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			data, err := apiClient.ListMailboxCustomFields(context.Background(), args[0], params)
			if err != nil {
				return err
			}

			items, err := extractEmbeddedWithCandidates(data, "customFields", "fields")
			if err != nil {
				if isJSON() {
					return output.PrintRaw(data)
				}
				return err
			}
			if isJSON() {
				return output.PrintRaw(mustMarshal(items))
			}

			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var row map[string]any
				json.Unmarshal(raw, &row)
				rows[i] = map[string]string{
					"id":   fmt.Sprintf("%v", row["id"]),
					"name": fmt.Sprintf("%v", row["name"]),
					"type": fmt.Sprintf("%v", row["type"]),
				}
			}
			return output.Print(getFormat(), []string{"id", "name", "type"}, rows)
		},
	}

	permission.Annotate(listCmd, "mailboxes", permission.OpRead)

	cmd := &cobra.Command{
		Use:   "custom-fields",
		Short: "Manage mailbox custom fields",
	}
	cmd.AddCommand(listCmd)
	return cmd
}

func newMailboxRoutingCmd() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get <mailbox-id>",
		Short: "Get mailbox routing settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetMailboxRouting(context.Background(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				return output.PrintRaw(data)
			}
			return output.Print("table", []string{"mailbox_id", "routing"}, []map[string]string{{
				"mailbox_id": args[0],
				"routing":    truncate(string(data), 120),
			}})
		},
	}

	updateCmd := &cobra.Command{
		Use:   "update <mailbox-id>",
		Short: "Update mailbox routing settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawBody, _ := cmd.Flags().GetString("json")
			body, err := parseJSONBody(rawBody)
			if err != nil {
				return fmt.Errorf("invalid --json payload: %w", err)
			}
			if err := apiClient.UpdateMailboxRouting(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated routing for mailbox %s\n", args[0])
			return nil
		},
	}
	permission.Annotate(getCmd, "mailboxes", permission.OpRead)
	permission.Annotate(updateCmd, "mailboxes", permission.OpWrite)
	updateCmd.Flags().String("json", "", "routing update payload as JSON object (required)")
	updateCmd.MarkFlagRequired("json")

	cmd := &cobra.Command{
		Use:   "routing",
		Short: "Manage mailbox routing",
	}
	cmd.AddCommand(getCmd, updateCmd)
	return cmd
}
