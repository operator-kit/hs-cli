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
)

func newDocsRedirectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirects",
		Short: "Manage Docs redirects",
	}

	listCmd := docsRedirectsListCmd()
	permission.Annotate(listCmd, "redirects", permission.OpRead)

	getCmd := docsRedirectsGetCmd()
	permission.Annotate(getCmd, "redirects", permission.OpRead)

	findCmd := docsRedirectsFindCmd()
	permission.Annotate(findCmd, "redirects", permission.OpRead)
	findCmd.Flags().String("site", "", "site ID (required)")
	findCmd.Flags().String("url", "", "URL to find redirect for (required)")
	findCmd.MarkFlagRequired("site")
	findCmd.MarkFlagRequired("url")

	createCmd := docsRedirectsCreateCmd()
	permission.Annotate(createCmd, "redirects", permission.OpWrite)
	createCmd.Flags().String("site", "", "site ID (required)")
	createCmd.Flags().String("url-mapping", "", "source URL path (required)")
	createCmd.Flags().String("redirect", "", "destination URL path (required)")
	createCmd.MarkFlagRequired("site")
	createCmd.MarkFlagRequired("url-mapping")
	createCmd.MarkFlagRequired("redirect")

	updateCmd := docsRedirectsUpdateCmd()
	permission.Annotate(updateCmd, "redirects", permission.OpWrite)
	updateCmd.Flags().String("url-mapping", "", "source URL path")
	updateCmd.Flags().String("redirect", "", "destination URL path")

	deleteCmd := docsRedirectsDeleteCmd()
	permission.Annotate(deleteCmd, "redirects", permission.OpDelete)

	cmd.AddCommand(listCmd, getCmd, findCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}

func docsRedirectsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <site-id>",
		Short: "List redirects for a site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			siteID := args[0]
			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.ListRedirects(ctx, siteID, p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "redirects", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "redirects", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "siteId", "urlMapping", "redirect"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":         jsonStr(m, "id"),
					"siteId":     jsonStr(m, "siteId"),
					"urlMapping": jsonStr(m, "urlMapping"),
					"redirect":   jsonStr(m, "redirect"),
				}
			}
			if err := output.Print(getFormat(), cols, rows); err != nil {
				return err
			}
			if pageInfo != nil && !noPaginate {
				fmt.Fprintf(output.Out, "\nPage %d of %d (%d total)\n", pageInfo.Page, pageInfo.Pages, pageInfo.Count)
			}
			return nil
		},
	}
}

func docsRedirectsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get redirect details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := docsClient.GetRedirect(context.Background(), args[0])
			if err != nil {
				return err
			}

			if isJSON() {
				if !isJSONClean() {
					return output.PrintRaw(data)
				}
				return output.PrintRaw(mustMarshal(cleanRawObject(data, docsCleanMinimal)))
			}

			var m map[string]any
			json.Unmarshal(data, &m)
			if inner, ok := m["redirect"].(map[string]any); ok {
				m = inner
			}

			cols := []string{"id", "siteId", "urlMapping", "redirect"}
			rows := []map[string]string{{
				"id":         jsonStr(m, "id"),
				"siteId":     jsonStr(m, "siteId"),
				"urlMapping": jsonStr(m, "urlMapping"),
				"redirect":   jsonStr(m, "redirect"),
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func docsRedirectsFindCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "find",
		Short: "Find a redirect by site and URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			site, _ := cmd.Flags().GetString("site")
			u, _ := cmd.Flags().GetString("url")
			params.Set("siteId", site)
			params.Set("url", u)

			data, err := docsClient.FindRedirect(context.Background(), params)
			if err != nil {
				return err
			}
			return output.PrintRaw(data)
		},
	}
}

func docsRedirectsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a redirect",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			body["siteId"], _ = cmd.Flags().GetString("site")
			body["urlMapping"], _ = cmd.Flags().GetString("url-mapping")
			body["redirect"], _ = cmd.Flags().GetString("redirect")

			data, err := docsClient.CreateRedirect(context.Background(), body)
			if err != nil {
				return err
			}
			id := extractDocsID(data, "redirect")
			fmt.Fprintf(output.Out, "Created redirect %s\n", id)
			return nil
		},
	}
}

func docsRedirectsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update a redirect",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			changed := false
			if v, _ := cmd.Flags().GetString("url-mapping"); v != "" {
				body["urlMapping"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("redirect"); v != "" {
				body["redirect"] = v
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := docsClient.UpdateRedirect(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated redirect %s\n", args[0])
			return nil
		},
	}
}

func docsRedirectsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a redirect",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := docsClient.DeleteRedirect(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted redirect %s\n", args[0])
			return nil
		},
	}
}
