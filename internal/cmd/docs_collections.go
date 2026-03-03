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

func newDocsCollectionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collections",
		Short: "Manage Docs collections",
	}

	listCmd := docsCollectionsListCmd()
	permission.Annotate(listCmd, "collections", permission.OpRead)
	listCmd.Flags().String("site", "", "filter by site ID")
	listCmd.Flags().String("visibility", "", "filter by visibility (public|private)")
	listCmd.Flags().String("sort", "", "sort field")
	listCmd.Flags().String("order", "", "sort order (asc|desc)")

	getCmd := docsCollectionsGetCmd()
	permission.Annotate(getCmd, "collections", permission.OpRead)

	createCmd := docsCollectionsCreateCmd()
	permission.Annotate(createCmd, "collections", permission.OpWrite)
	createCmd.Flags().String("site", "", "site ID (required)")
	createCmd.Flags().String("name", "", "collection name (required)")
	createCmd.Flags().String("visibility", "", "visibility (public|private)")
	createCmd.Flags().Int("order", 0, "display order")
	createCmd.Flags().String("description", "", "collection description")
	createCmd.MarkFlagRequired("site")
	createCmd.MarkFlagRequired("name")

	updateCmd := docsCollectionsUpdateCmd()
	permission.Annotate(updateCmd, "collections", permission.OpWrite)
	updateCmd.Flags().String("name", "", "collection name")
	updateCmd.Flags().String("visibility", "", "visibility (public|private)")
	updateCmd.Flags().Int("order", 0, "display order")
	updateCmd.Flags().String("description", "", "collection description")

	deleteCmd := docsCollectionsDeleteCmd()
	permission.Annotate(deleteCmd, "collections", permission.OpDelete)

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)
	return cmd
}

func docsCollectionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List collections",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			if v, _ := cmd.Flags().GetString("site"); v != "" {
				params.Set("siteId", v)
			}
			if v, _ := cmd.Flags().GetString("visibility"); v != "" {
				params.Set("visibility", v)
			}
			if v, _ := cmd.Flags().GetString("sort"); v != "" {
				params.Set("sort", v)
			}
			if v, _ := cmd.Flags().GetString("order"); v != "" {
				params.Set("order", v)
			}

			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.ListCollections(ctx, p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "collections", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "collections", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "name", "slug", "visibility", "siteId"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":         jsonStr(m, "id"),
					"name":       jsonStr(m, "name"),
					"slug":       jsonStr(m, "slug"),
					"visibility":  jsonStr(m, "visibility"),
					"siteId":     jsonStr(m, "siteId"),
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

func docsCollectionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get collection details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := docsClient.GetCollection(context.Background(), args[0])
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
			// Docs API wraps single objects: {"collection":{...}}
			if inner, ok := m["collection"].(map[string]any); ok {
				m = inner
			}

			cols := []string{"id", "name", "slug", "visibility", "siteId"}
			rows := []map[string]string{{
				"id":         jsonStr(m, "id"),
				"name":       jsonStr(m, "name"),
				"slug":       jsonStr(m, "slug"),
				"visibility":  jsonStr(m, "visibility"),
				"siteId":     jsonStr(m, "siteId"),
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func docsCollectionsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			body["siteId"], _ = cmd.Flags().GetString("site")
			body["name"], _ = cmd.Flags().GetString("name")
			if v, _ := cmd.Flags().GetString("visibility"); v != "" {
				body["visibility"] = v
			}
			if cmd.Flags().Changed("order") {
				v, _ := cmd.Flags().GetInt("order")
				body["order"] = v
			}
			if v, _ := cmd.Flags().GetString("description"); v != "" {
				body["description"] = v
			}

			data, err := docsClient.CreateCollection(context.Background(), body)
			if err != nil {
				return err
			}
			id := extractDocsID(data, "collection")
			fmt.Fprintf(output.Out, "Created collection %s\n", id)
			return nil
		},
	}
}

func docsCollectionsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			changed := false
			if v, _ := cmd.Flags().GetString("name"); v != "" {
				body["name"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("visibility"); v != "" {
				body["visibility"] = v
				changed = true
			}
			if cmd.Flags().Changed("order") {
				v, _ := cmd.Flags().GetInt("order")
				body["order"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("description"); v != "" {
				body["description"] = v
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := docsClient.UpdateCollection(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated collection %s\n", args[0])
			return nil
		},
	}
}

func docsCollectionsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := docsClient.DeleteCollection(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted collection %s\n", args[0])
			return nil
		},
	}
}
