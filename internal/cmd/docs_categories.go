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

func newDocsCategoriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "Manage Docs categories",
	}

	listCmd := docsCategoriesListCmd()
	permission.Annotate(listCmd, "categories", permission.OpRead)

	getCmd := docsCategoriesGetCmd()
	permission.Annotate(getCmd, "categories", permission.OpRead)

	createCmd := docsCategoriesCreateCmd()
	permission.Annotate(createCmd, "categories", permission.OpWrite)
	createCmd.Flags().String("collection", "", "collection ID (required)")
	createCmd.Flags().String("name", "", "category name (required)")
	createCmd.Flags().String("slug", "", "URL slug")
	createCmd.Flags().String("visibility", "", "visibility (public|private)")
	createCmd.Flags().Int("order", 0, "display order")
	createCmd.Flags().String("default-sort", "", "default sort (name|number|popularity|manual)")
	createCmd.MarkFlagRequired("collection")
	createCmd.MarkFlagRequired("name")

	updateCmd := docsCategoriesUpdateCmd()
	permission.Annotate(updateCmd, "categories", permission.OpWrite)
	updateCmd.Flags().String("name", "", "category name")
	updateCmd.Flags().String("slug", "", "URL slug")
	updateCmd.Flags().String("visibility", "", "visibility (public|private)")
	updateCmd.Flags().Int("order", 0, "display order")
	updateCmd.Flags().String("default-sort", "", "default sort (name|number|popularity|manual)")

	reorderCmd := docsCategoriesReorderCmd()
	permission.Annotate(reorderCmd, "categories", permission.OpWrite)
	reorderCmd.Flags().StringSlice("categories", nil, "ordered category IDs (required)")
	reorderCmd.MarkFlagRequired("categories")

	deleteCmd := docsCategoriesDeleteCmd()
	permission.Annotate(deleteCmd, "categories", permission.OpDelete)

	cmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, reorderCmd, deleteCmd)
	return cmd
}

func docsCategoriesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <collection-id>",
		Short: "List categories in a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			collectionID := args[0]
			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.ListCategories(ctx, collectionID, p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "categories", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "categories", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "name", "slug", "visibility", "order"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":         jsonStr(m, "id"),
					"name":       jsonStr(m, "name"),
					"slug":       jsonStr(m, "slug"),
					"visibility": jsonStr(m, "visibility"),
					"order":      jsonStr(m, "order"),
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

func docsCategoriesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get category details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := docsClient.GetCategory(context.Background(), args[0])
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
			if inner, ok := m["category"].(map[string]any); ok {
				m = inner
			}

			cols := []string{"id", "name", "slug", "visibility", "order", "collectionId"}
			rows := []map[string]string{{
				"id":           jsonStr(m, "id"),
				"name":         jsonStr(m, "name"),
				"slug":         jsonStr(m, "slug"),
				"visibility":   jsonStr(m, "visibility"),
				"order":        jsonStr(m, "order"),
				"collectionId": jsonStr(m, "collectionId"),
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func docsCategoriesCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a category",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			body["collectionId"], _ = cmd.Flags().GetString("collection")
			body["name"], _ = cmd.Flags().GetString("name")
			if v, _ := cmd.Flags().GetString("slug"); v != "" {
				body["slug"] = v
			}
			if v, _ := cmd.Flags().GetString("visibility"); v != "" {
				body["visibility"] = v
			}
			if cmd.Flags().Changed("order") {
				v, _ := cmd.Flags().GetInt("order")
				body["order"] = v
			}
			if v, _ := cmd.Flags().GetString("default-sort"); v != "" {
				body["defaultSort"] = v
			}

			data, err := docsClient.CreateCategory(context.Background(), body)
			if err != nil {
				return err
			}
			id := extractDocsID(data, "category")
			fmt.Fprintf(output.Out, "Created category %s\n", id)
			return nil
		},
	}
}

func docsCategoriesUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update a category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			changed := false
			if v, _ := cmd.Flags().GetString("name"); v != "" {
				body["name"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("slug"); v != "" {
				body["slug"] = v
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
			if v, _ := cmd.Flags().GetString("default-sort"); v != "" {
				body["defaultSort"] = v
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := docsClient.UpdateCategory(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated category %s\n", args[0])
			return nil
		},
	}
}

func docsCategoriesReorderCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reorder <collection-id>",
		Short: "Reorder categories in a collection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			categories, _ := cmd.Flags().GetStringSlice("categories")
			body := map[string]any{
				"categories": categories,
			}
			if err := docsClient.ReorderCategory(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Reordered categories in collection %s\n", args[0])
			return nil
		},
	}
}

func docsCategoriesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := docsClient.DeleteCategory(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted category %s\n", args[0])
			return nil
		},
	}
}
