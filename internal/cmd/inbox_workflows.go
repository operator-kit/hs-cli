package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/types"
)

func newWorkflowsCmd() *cobra.Command {
	wfCmd := &cobra.Command{
		Use:     "workflows",
		Aliases: []string{"wf"},
		Short:   "Manage workflows",
	}

	listCmd := workflowsListCmd()
	permission.Annotate(listCmd, "workflows", permission.OpRead)
	listCmd.Flags().Int("mailbox-id", 0, "filter by mailbox ID")
	listCmd.Flags().String("type", "", "filter by workflow type")

	updateStatusCmd := workflowsUpdateStatusCmd()
	permission.Annotate(updateStatusCmd, "workflows", permission.OpWrite)
	updateStatusCmd.Flags().String("status", "", "workflow status (required: active or inactive)")
	updateStatusCmd.MarkFlagRequired("status")

	runCmd := workflowsRunCmd()
	permission.Annotate(runCmd, "workflows", permission.OpWrite)
	runCmd.Flags().StringSlice("conversation-ids", nil, "conversation IDs to run workflow on (required)")
	runCmd.MarkFlagRequired("conversation-ids")

	wfCmd.AddCommand(listCmd, updateStatusCmd, runCmd)
	return wfCmd
}

func workflowsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			if v, _ := cmd.Flags().GetInt("mailbox-id"); v > 0 {
				params.Set("mailboxId", strconv.Itoa(v))
			}
			if v, _ := cmd.Flags().GetString("type"); v != "" {
				params.Set("type", v)
			}

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListWorkflows, params, "workflows", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, cleanMinimal)))
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListWorkflows, params, "workflows", noPaginate)
			if err != nil {
				return err
			}

			var workflows []types.Workflow
			for _, raw := range items {
				var w types.Workflow
				json.Unmarshal(raw, &w)
				workflows = append(workflows, w)
			}

			cols := []string{"id", "name", "type", "status", "mailbox_id"}
			rows := make([]map[string]string, len(workflows))
			for i, w := range workflows {
				rows[i] = map[string]string{
					"id":         strconv.Itoa(w.ID),
					"name":       w.Name,
					"type":       w.Type,
					"status":     w.Status,
					"mailbox_id": strconv.Itoa(w.MailboxID),
				}
			}
			if err := output.Print(getFormat(), cols, rows); err != nil {
				return err
			}
			if pageInfo != nil && !noPaginate {
				fmt.Fprintf(output.Out, "\nPage %d of %d (%d total)\n", pageInfo.Number, pageInfo.TotalPages, pageInfo.TotalElements)
			}
			return nil
		},
	}
}

func workflowsUpdateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update-status <id>",
		Short: "Activate or deactivate a workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, _ := cmd.Flags().GetString("status")
			if status != "active" && status != "inactive" {
				return fmt.Errorf("--status must be \"active\" or \"inactive\"")
			}
			body := map[string]any{
				"op":    "replace",
				"path":  "/status",
				"value": status,
			}
			if err := apiClient.UpdateWorkflowStatus(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Workflow %s status set to %s\n", args[0], status)
			return nil
		},
	}
}

func workflowsRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <id>",
		Short: "Run a workflow on conversations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			convIDs, _ := cmd.Flags().GetStringSlice("conversation-ids")
			if len(convIDs) > 50 {
				return fmt.Errorf("conversation-ids supports at most 50 IDs")
			}

			parsedIDs := make([]int, 0, len(convIDs))
			for _, rawID := range convIDs {
				idStr := strings.TrimSpace(rawID)
				id, err := strconv.Atoi(idStr)
				if err != nil {
					return fmt.Errorf("invalid conversation ID %q: must be an integer", rawID)
				}
				parsedIDs = append(parsedIDs, id)
			}

			body := map[string]any{
				"conversationIds": parsedIDs,
			}
			if err := apiClient.RunWorkflow(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Workflow %s executed on %d conversations.\n", args[0], len(parsedIDs))
			return nil
		},
	}
}
