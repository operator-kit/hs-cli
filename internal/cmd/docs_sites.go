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

func newDocsSitesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites",
		Short: "Manage Docs sites",
	}

	listCmd := docsSitesListCmd()
	permission.Annotate(listCmd, "sites", permission.OpRead)

	getCmd := docsSitesGetCmd()
	permission.Annotate(getCmd, "sites", permission.OpRead)

	createCmd := docsSitesCreateCmd()
	permission.Annotate(createCmd, "sites", permission.OpWrite)
	createCmd.Flags().String("subdomain", "", "subdomain (required)")
	createCmd.Flags().String("title", "", "site title (required)")
	createCmd.Flags().String("status", "", "status")
	createCmd.Flags().String("cname", "", "custom CNAME")
	createCmd.Flags().Bool("has-public-site", false, "has public site")
	createCmd.Flags().String("logo-url", "", "logo URL")
	createCmd.Flags().String("favicon-url", "", "favicon URL")
	createCmd.Flags().String("color", "", "primary color")
	createCmd.Flags().String("contact-email", "", "contact email")
	createCmd.MarkFlagRequired("subdomain")
	createCmd.MarkFlagRequired("title")

	updateCmd := docsSitesUpdateCmd()
	permission.Annotate(updateCmd, "sites", permission.OpWrite)
	updateCmd.Flags().String("subdomain", "", "subdomain")
	updateCmd.Flags().String("title", "", "site title")
	updateCmd.Flags().String("status", "", "status")
	updateCmd.Flags().String("cname", "", "custom CNAME")
	updateCmd.Flags().Bool("has-public-site", false, "has public site")
	updateCmd.Flags().String("logo-url", "", "logo URL")
	updateCmd.Flags().String("favicon-url", "", "favicon URL")
	updateCmd.Flags().String("color", "", "primary color")
	updateCmd.Flags().String("contact-email", "", "contact email")

	deleteCmd := docsSitesDeleteCmd()
	permission.Annotate(deleteCmd, "sites", permission.OpDelete)

	restrictionsCmd := &cobra.Command{Use: "restrictions", Short: "Manage site restrictions"}
	restrictionsGetCmd := docsSitesRestrictionsGetCmd()
	permission.Annotate(restrictionsGetCmd, "sites", permission.OpRead)
	restrictionsUpdateCmd := docsSitesRestrictionsUpdateCmd()
	permission.Annotate(restrictionsUpdateCmd, "sites", permission.OpWrite)
	restrictionsUpdateCmd.Flags().StringSlice("emails", nil, "allowed email addresses")
	restrictionsUpdateCmd.Flags().StringSlice("domains", nil, "allowed domains")
	restrictionsCmd.AddCommand(restrictionsGetCmd, restrictionsUpdateCmd)

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd, restrictionsCmd)
	return cmd
}

func docsSitesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.ListSites(ctx, p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "sites", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "sites", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "subdomain", "title", "status"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":        jsonStr(m, "id"),
					"subdomain": jsonStr(m, "subDomain"),
					"title":     jsonStr(m, "title"),
					"status":    jsonStr(m, "status"),
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

func docsSitesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get site details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := docsClient.GetSite(context.Background(), args[0])
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
			if inner, ok := m["site"].(map[string]any); ok {
				m = inner
			}

			cols := []string{"id", "subdomain", "title", "status", "cname"}
			rows := []map[string]string{{
				"id":        jsonStr(m, "id"),
				"subdomain": jsonStr(m, "subDomain"),
				"title":     jsonStr(m, "title"),
				"status":    jsonStr(m, "status"),
				"cname":     jsonStr(m, "cname"),
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func docsSitesCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a site",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			body["subDomain"], _ = cmd.Flags().GetString("subdomain")
			body["title"], _ = cmd.Flags().GetString("title")
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				body["status"] = v
			}
			if v, _ := cmd.Flags().GetString("cname"); v != "" {
				body["cname"] = v
			}
			if cmd.Flags().Changed("has-public-site") {
				v, _ := cmd.Flags().GetBool("has-public-site")
				body["hasPublicSite"] = v
			}
			if v, _ := cmd.Flags().GetString("logo-url"); v != "" {
				body["logoUrl"] = v
			}
			if v, _ := cmd.Flags().GetString("favicon-url"); v != "" {
				body["favIconUrl"] = v
			}
			if v, _ := cmd.Flags().GetString("color"); v != "" {
				body["color"] = v
			}
			if v, _ := cmd.Flags().GetString("contact-email"); v != "" {
				body["contactEmail"] = v
			}

			data, err := docsClient.CreateSite(context.Background(), body)
			if err != nil {
				return err
			}
			id := extractDocsID(data, "site")
			fmt.Fprintf(output.Out, "Created site %s\n", id)
			return nil
		},
	}
}

func docsSitesUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update a site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			changed := false
			if v, _ := cmd.Flags().GetString("subdomain"); v != "" {
				body["subDomain"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("title"); v != "" {
				body["title"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				body["status"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("cname"); v != "" {
				body["cname"] = v
				changed = true
			}
			if cmd.Flags().Changed("has-public-site") {
				v, _ := cmd.Flags().GetBool("has-public-site")
				body["hasPublicSite"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("logo-url"); v != "" {
				body["logoUrl"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("favicon-url"); v != "" {
				body["favIconUrl"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("color"); v != "" {
				body["color"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("contact-email"); v != "" {
				body["contactEmail"] = v
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := docsClient.UpdateSite(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated site %s\n", args[0])
			return nil
		},
	}
}

func docsSitesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := docsClient.DeleteSite(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted site %s\n", args[0])
			return nil
		},
	}
}

func docsSitesRestrictionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <site-id>",
		Short: "Get site restrictions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := docsClient.GetSiteRestrictions(context.Background(), args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(data)
		},
	}
}

func docsSitesRestrictionsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <site-id>",
		Short: "Update site restrictions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if v, _ := cmd.Flags().GetStringSlice("emails"); len(v) > 0 {
				body["emails"] = v
			}
			if v, _ := cmd.Flags().GetStringSlice("domains"); len(v) > 0 {
				body["domains"] = v
			}
			if err := docsClient.UpdateSiteRestrictions(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated restrictions for site %s\n", args[0])
			return nil
		},
	}
}
