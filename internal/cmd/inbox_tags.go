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

func newTagsCmd() *cobra.Command {
	tagsCmd := &cobra.Command{
		Use:   "tags",
		Short: "Manage tags",
	}
	listCmd := tagsListCmd()
	permission.Annotate(listCmd, "tags", permission.OpRead)

	getCmd := tagsGetCmd()
	permission.Annotate(getCmd, "tags", permission.OpRead)

	tagsCmd.AddCommand(listCmd, getCmd)
	return tagsCmd
}

func tagsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tags",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			if isJSON() {
				items, _, err := api.PaginateAll(ctx, apiClient.ListTags, params, "tags", noPaginate)
				if err != nil {
					return err
				}
				return output.PrintRaw(mustMarshal(items))
			}

			items, pageInfo, err := api.PaginateAll(ctx, apiClient.ListTags, params, "tags", noPaginate)
			if err != nil {
				return err
			}

			var tags []types.Tag
			for _, raw := range items {
				var t types.Tag
				json.Unmarshal(raw, &t)
				tags = append(tags, t)
			}

			cols := []string{"id", "name", "slug", "color"}
			rows := make([]map[string]string, len(tags))
			for i, t := range tags {
				rows[i] = map[string]string{
					"id":    strconv.Itoa(t.ID),
					"name":  t.Name,
					"slug":  t.Slug,
					"color": t.Color,
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

func tagsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get tag details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := apiClient.GetTag(context.Background(), args[0])
			if err != nil {
				return err
			}

			if isJSON() {
				return output.PrintRaw(data)
			}

			var t types.Tag
			json.Unmarshal(data, &t)

			cols := []string{"id", "name", "slug", "color"}
			rows := []map[string]string{{
				"id":    strconv.Itoa(t.ID),
				"name":  t.Name,
				"slug":  t.Slug,
				"color": t.Color,
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}
