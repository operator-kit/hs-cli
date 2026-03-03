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

func newMailboxesCmd() *cobra.Command {
	mailboxesCmd := &cobra.Command{
		Use:     "mailboxes",
		Aliases: []string{"mb"},
		Short:   "Manage mailboxes",
	}

	listCmd := mailboxesListCmd()
	permission.Annotate(listCmd, "mailboxes", permission.OpRead)

	getCmd := mailboxesGetCmd()
	permission.Annotate(getCmd, "mailboxes", permission.OpRead)

	mailboxesCmd.AddCommand(
		listCmd,
		getCmd,
		newMailboxFoldersCmd(),
		newMailboxCustomFieldsCmd(),
		newMailboxRoutingCmd(),
	)
	return mailboxesCmd
}

func mailboxesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List mailboxes",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListMailboxes, params, "mailboxes", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, cleanMinimal)))
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListMailboxes, params, "mailboxes", noPaginate)
			if err != nil {
				return err
			}

			var mailboxes []types.Mailbox
			for _, raw := range items {
				var m types.Mailbox
				json.Unmarshal(raw, &m)
				mailboxes = append(mailboxes, m)
			}

			cols := []string{"id", "name", "email", "slug"}
			rows := make([]map[string]string, len(mailboxes))
			for i, m := range mailboxes {
				rows[i] = map[string]string{
					"id":    strconv.Itoa(m.ID),
					"name":  m.Name,
					"email": m.Email,
					"slug":  m.Slug,
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

func mailboxesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get mailbox details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetMailbox(context.Background(), args[0])
			if err != nil {
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return output.PrintRaw(data)
				}
				return output.PrintRaw(mustMarshal(cleanRawObject(data, cleanMinimal)))
			}

			var m types.Mailbox
			json.Unmarshal(data, &m)

			cols := []string{"id", "name", "email", "slug"}
			rows := []map[string]string{{
				"id":    strconv.Itoa(m.ID),
				"name":  m.Name,
				"email": m.Email,
				"slug":  m.Slug,
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
