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

func newDocsArticlesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "articles",
		Short: "Manage Docs articles",
	}

	listCmd := docsArticlesListCmd()
	permission.Annotate(listCmd, "articles", permission.OpRead)
	listCmd.Flags().String("collection", "", "collection ID (required if no --category)")
	listCmd.Flags().String("category", "", "category ID (required if no --collection)")
	listCmd.Flags().String("status", "", "filter by status (published|notpublished|all)")
	listCmd.Flags().String("sort", "", "sort field")
	listCmd.Flags().String("order", "", "sort order (asc|desc)")

	searchCmd := docsArticlesSearchCmd()
	permission.Annotate(searchCmd, "articles", permission.OpRead)
	searchCmd.Flags().String("query", "", "search query (required)")
	searchCmd.Flags().String("collection", "", "filter by collection ID")
	searchCmd.Flags().String("site", "", "filter by site ID")
	searchCmd.Flags().String("status", "", "filter by status")
	searchCmd.Flags().String("visibility", "", "filter by visibility")
	searchCmd.MarkFlagRequired("query")

	getCmd := docsArticlesGetCmd()
	permission.Annotate(getCmd, "articles", permission.OpRead)
	getCmd.Flags().Bool("draft", false, "retrieve draft version")

	relatedCmd := docsArticlesRelatedCmd()
	permission.Annotate(relatedCmd, "articles", permission.OpRead)

	createCmd := docsArticlesCreateCmd()
	permission.Annotate(createCmd, "articles", permission.OpWrite)
	createCmd.Flags().String("collection", "", "collection ID (required)")
	createCmd.Flags().String("name", "", "article title (required)")
	createCmd.Flags().String("text", "", "article body HTML (required)")
	createCmd.Flags().String("status", "", "status (published|notpublished)")
	createCmd.Flags().String("slug", "", "URL slug")
	createCmd.Flags().StringSlice("categories", nil, "category IDs")
	createCmd.Flags().StringSlice("related", nil, "related article IDs")
	createCmd.Flags().StringSlice("keywords", nil, "SEO keywords")
	createCmd.MarkFlagRequired("collection")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("text")

	updateCmd := docsArticlesUpdateCmd()
	permission.Annotate(updateCmd, "articles", permission.OpWrite)
	updateCmd.Flags().String("name", "", "article title")
	updateCmd.Flags().String("text", "", "article body HTML")
	updateCmd.Flags().String("status", "", "status (published|notpublished)")
	updateCmd.Flags().String("slug", "", "URL slug")
	updateCmd.Flags().StringSlice("categories", nil, "category IDs")
	updateCmd.Flags().StringSlice("related", nil, "related article IDs")
	updateCmd.Flags().StringSlice("keywords", nil, "SEO keywords")

	deleteCmd := docsArticlesDeleteCmd()
	permission.Annotate(deleteCmd, "articles", permission.OpDelete)

	uploadCmd := docsArticlesUploadCmd()
	permission.Annotate(uploadCmd, "articles", permission.OpWrite)
	uploadCmd.Flags().String("file", "", "file path to upload (required)")
	uploadCmd.MarkFlagRequired("file")

	viewsCmd := docsArticlesViewsUpdateCmd()
	permission.Annotate(viewsCmd, "articles", permission.OpWrite)
	viewsCmd.Flags().Int("count", 0, "view count to set (required)")
	viewsCmd.MarkFlagRequired("count")

	draftCmd := &cobra.Command{Use: "draft", Short: "Manage article drafts"}
	draftSaveCmd := docsArticlesDraftSaveCmd()
	permission.Annotate(draftSaveCmd, "articles", permission.OpWrite)
	draftSaveCmd.Flags().String("text", "", "draft body HTML (required)")
	draftSaveCmd.MarkFlagRequired("text")

	draftDeleteCmd := docsArticlesDraftDeleteCmd()
	permission.Annotate(draftDeleteCmd, "articles", permission.OpWrite)

	draftCmd.AddCommand(draftSaveCmd, draftDeleteCmd)

	revisionsCmd := &cobra.Command{Use: "revisions", Short: "Manage article revisions"}
	revListCmd := docsArticlesRevisionsListCmd()
	permission.Annotate(revListCmd, "articles", permission.OpRead)
	revGetCmd := docsArticlesRevisionsGetCmd()
	permission.Annotate(revGetCmd, "articles", permission.OpRead)
	revisionsCmd.AddCommand(revListCmd, revGetCmd)

	cmd.AddCommand(listCmd, searchCmd, getCmd, relatedCmd, createCmd, updateCmd, deleteCmd, uploadCmd, viewsCmd, draftCmd, revisionsCmd)
	return cmd
}

func docsArticlesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List articles by collection or category",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			collectionID, _ := cmd.Flags().GetString("collection")
			categoryID, _ := cmd.Flags().GetString("category")
			if collectionID == "" && categoryID == "" {
				return fmt.Errorf("--collection or --category is required")
			}

			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				params.Set("status", v)
			}
			if v, _ := cmd.Flags().GetString("sort"); v != "" {
				params.Set("sort", v)
			}
			if v, _ := cmd.Flags().GetString("order"); v != "" {
				params.Set("order", v)
			}

			var fn api.DocsListFunc
			if categoryID != "" {
				fn = func(ctx context.Context, p url.Values) (json.RawMessage, error) {
					return docsClient.ListArticlesByCategory(ctx, categoryID, p)
				}
			} else {
				fn = func(ctx context.Context, p url.Values) (json.RawMessage, error) {
					return docsClient.ListArticles(ctx, collectionID, p)
				}
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "articles", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "articles", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "name", "slug", "status", "collectionId"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":           jsonStr(m, "id"),
					"name":         jsonStr(m, "name"),
					"slug":         jsonStr(m, "slug"),
					"status":       jsonStr(m, "status"),
					"collectionId": jsonStr(m, "collectionId"),
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

func docsArticlesSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search",
		Short: "Search articles",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))
			query, _ := cmd.Flags().GetString("query")
			params.Set("query", query)
			if v, _ := cmd.Flags().GetString("collection"); v != "" {
				params.Set("collectionId", v)
			}
			if v, _ := cmd.Flags().GetString("site"); v != "" {
				params.Set("siteId", v)
			}
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				params.Set("status", v)
			}
			if v, _ := cmd.Flags().GetString("visibility"); v != "" {
				params.Set("visibility", v)
			}

			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.SearchArticles(ctx, p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "articles", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "articles", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "name", "slug", "status", "collectionId"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":           jsonStr(m, "id"),
					"name":         jsonStr(m, "name"),
					"slug":         jsonStr(m, "slug"),
					"status":       jsonStr(m, "status"),
					"collectionId": jsonStr(m, "collectionId"),
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

func docsArticlesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get article details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if v, _ := cmd.Flags().GetBool("draft"); v {
				params.Set("draft", "true")
			}

			data, err := docsClient.GetArticle(context.Background(), args[0], params)
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
			if inner, ok := m["article"].(map[string]any); ok {
				m = inner
			}

			cols := []string{"id", "name", "slug", "status", "collectionId", "updatedAt"}
			rows := []map[string]string{{
				"id":           jsonStr(m, "id"),
				"name":         jsonStr(m, "name"),
				"slug":         jsonStr(m, "slug"),
				"status":       jsonStr(m, "status"),
				"collectionId": jsonStr(m, "collectionId"),
				"updatedAt":    jsonStr(m, "updatedAt"),
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func docsArticlesRelatedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "related <id>",
		Short: "List related articles",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.GetRelatedArticles(ctx, args[0], p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "articles", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "articles", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "name", "slug", "status"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":     jsonStr(m, "id"),
					"name":   jsonStr(m, "name"),
					"slug":   jsonStr(m, "slug"),
					"status": jsonStr(m, "status"),
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

func docsArticlesCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create an article",
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			body["collectionId"], _ = cmd.Flags().GetString("collection")
			body["name"], _ = cmd.Flags().GetString("name")
			body["text"], _ = cmd.Flags().GetString("text")
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				body["status"] = v
			}
			if v, _ := cmd.Flags().GetString("slug"); v != "" {
				body["slug"] = v
			}
			if v, _ := cmd.Flags().GetStringSlice("categories"); len(v) > 0 {
				body["categories"] = v
			}
			if v, _ := cmd.Flags().GetStringSlice("related"); len(v) > 0 {
				body["related"] = v
			}
			if v, _ := cmd.Flags().GetStringSlice("keywords"); len(v) > 0 {
				body["keywords"] = v
			}

			data, err := docsClient.CreateArticle(context.Background(), body)
			if err != nil {
				return err
			}
			id := extractDocsID(data, "article")
			fmt.Fprintf(output.Out, "Created article %s\n", id)
			return nil
		},
	}
}

func docsArticlesUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <id>",
		Short: "Update an article",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			changed := false
			if v, _ := cmd.Flags().GetString("name"); v != "" {
				body["name"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("text"); v != "" {
				body["text"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("status"); v != "" {
				body["status"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetString("slug"); v != "" {
				body["slug"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetStringSlice("categories"); len(v) > 0 {
				body["categories"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetStringSlice("related"); len(v) > 0 {
				body["related"] = v
				changed = true
			}
			if v, _ := cmd.Flags().GetStringSlice("keywords"); len(v) > 0 {
				body["keywords"] = v
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields to update")
			}
			if err := docsClient.UpdateArticle(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated article %s\n", args[0])
			return nil
		},
	}
}

func docsArticlesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an article",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := docsClient.DeleteArticle(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted article %s\n", args[0])
			return nil
		},
	}
}

func docsArticlesUploadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upload <id>",
		Short: "Upload an asset to an article",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, _ := cmd.Flags().GetString("file")
			data, err := docsClient.UploadArticleAsset(context.Background(), args[0], filePath)
			if err != nil {
				return err
			}
			return output.PrintRaw(data)
		},
	}
}

func docsArticlesViewsUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "views <id>",
		Short: "Update article view count",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			count, _ := cmd.Flags().GetInt("count")
			body := map[string]any{"count": count}
			if err := docsClient.UpdateArticleViewCount(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Updated view count for article %s\n", args[0])
			return nil
		},
	}
}

func docsArticlesDraftSaveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "save <article-id>",
		Short: "Save article draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, _ := cmd.Flags().GetString("text")
			body := map[string]any{"text": text}
			if err := docsClient.SaveArticleDraft(context.Background(), args[0], body); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Saved draft for article %s\n", args[0])
			return nil
		},
	}
}

func docsArticlesDraftDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <article-id>",
		Short: "Delete article draft",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := docsClient.DeleteArticleDraft(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(output.Out, "Deleted draft for article %s\n", args[0])
			return nil
		},
	}
}

func docsArticlesRevisionsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <article-id>",
		Short: "List article revisions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("pageSize", strconv.Itoa(perPage))

			articleID := args[0]
			fn := func(ctx context.Context, p url.Values) (json.RawMessage, error) {
				return docsClient.ListRevisions(ctx, articleID, p)
			}

			if isJSON() {
				items, _, err := api.DocsPaginateAll(ctx, fn, params, "revisions", noPaginate)
				if err != nil {
					return err
				}
				if !isJSONClean() {
					return output.PrintRaw(mustMarshal(items))
				}
				return output.PrintRaw(mustMarshal(cleanRawItems(items, docsCleanMinimal)))
			}

			items, pageInfo, err := api.DocsPaginateAll(ctx, fn, params, "revisions", noPaginate)
			if err != nil {
				return err
			}

			cols := []string{"id", "articleId", "createdBy", "createdAt"}
			rows := make([]map[string]string, len(items))
			for i, raw := range items {
				var m map[string]any
				json.Unmarshal(raw, &m)
				rows[i] = map[string]string{
					"id":        jsonStr(m, "id"),
					"articleId": jsonStr(m, "articleId"),
					"createdBy": jsonStr(m, "createdBy"),
					"createdAt": jsonStr(m, "createdAt"),
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

func docsArticlesRevisionsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <article-id> <revision-id>",
		Short: "Get a specific article revision",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := docsClient.GetRevision(context.Background(), args[0], args[1])
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
			if inner, ok := m["revision"].(map[string]any); ok {
				m = inner
			}

			cols := []string{"id", "articleId", "createdBy", "createdAt"}
			rows := []map[string]string{{
				"id":        jsonStr(m, "id"),
				"articleId": jsonStr(m, "articleId"),
				"createdBy": jsonStr(m, "createdBy"),
				"createdAt": jsonStr(m, "createdAt"),
			}}
			return output.Print(getFormat(), cols, rows)
		},
	}
}
