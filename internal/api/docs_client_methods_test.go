package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func docsClientWithServer(t *testing.T, handler http.HandlerFunc) *DocsClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	httpClient := &http.Client{Transport: &rewriteTransport{
		base:    http.DefaultTransport,
		baseURL: srv.URL,
	}}
	return NewDocsForTest(httpClient, "test-key")
}

func TestDocsClientEndpointMethods(t *testing.T) {
	ctx := context.Background()
	body := map[string]any{"name": "test"}
	params := url.Values{"page": []string{"2"}}
	assetPath := filepath.Join(t.TempDir(), "asset.txt")
	require.NoError(t, os.WriteFile(assetPath, []byte("asset"), 0o600))

	tests := []struct {
		name      string
		method    string
		path      string
		query     string
		multipart bool
		call      func(*DocsClient) error
	}{
		{name: "ListCollections", method: http.MethodGet, path: "/v1/collections", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListCollections(ctx, params)
			return err
		}},
		{name: "GetCollection", method: http.MethodGet, path: "/v1/collections/1", call: func(c *DocsClient) error {
			_, err := c.GetCollection(ctx, "1")
			return err
		}},
		{name: "CreateCollection", method: http.MethodPost, path: "/v1/collections", call: func(c *DocsClient) error {
			_, err := c.CreateCollection(ctx, body)
			return err
		}},
		{name: "UpdateCollection", method: http.MethodPut, path: "/v1/collections/1", call: func(c *DocsClient) error {
			return c.UpdateCollection(ctx, "1", body)
		}},
		{name: "DeleteCollection", method: http.MethodDelete, path: "/v1/collections/1", call: func(c *DocsClient) error {
			return c.DeleteCollection(ctx, "1")
		}},
		{name: "ListCategories", method: http.MethodGet, path: "/v1/collections/1/categories", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListCategories(ctx, "1", params)
			return err
		}},
		{name: "GetCategory", method: http.MethodGet, path: "/v1/categories/2", call: func(c *DocsClient) error {
			_, err := c.GetCategory(ctx, "2")
			return err
		}},
		{name: "CreateCategory", method: http.MethodPost, path: "/v1/categories", call: func(c *DocsClient) error {
			_, err := c.CreateCategory(ctx, body)
			return err
		}},
		{name: "UpdateCategory", method: http.MethodPut, path: "/v1/categories/2", call: func(c *DocsClient) error {
			return c.UpdateCategory(ctx, "2", body)
		}},
		{name: "ReorderCategory", method: http.MethodPut, path: "/v1/collections/1/categories/order", call: func(c *DocsClient) error {
			return c.ReorderCategory(ctx, "1", body)
		}},
		{name: "DeleteCategory", method: http.MethodDelete, path: "/v1/categories/2", call: func(c *DocsClient) error {
			return c.DeleteCategory(ctx, "2")
		}},
		{name: "ListArticles", method: http.MethodGet, path: "/v1/collections/1/articles", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListArticles(ctx, "1", params)
			return err
		}},
		{name: "ListArticlesByCategory", method: http.MethodGet, path: "/v1/categories/2/articles", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListArticlesByCategory(ctx, "2", params)
			return err
		}},
		{name: "SearchArticles", method: http.MethodGet, path: "/v1/search/articles", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.SearchArticles(ctx, params)
			return err
		}},
		{name: "GetArticle", method: http.MethodGet, path: "/v1/articles/3", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.GetArticle(ctx, "3", params)
			return err
		}},
		{name: "GetRelatedArticles", method: http.MethodGet, path: "/v1/articles/3/related", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.GetRelatedArticles(ctx, "3", params)
			return err
		}},
		{name: "ListRevisions", method: http.MethodGet, path: "/v1/articles/3/revisions", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListRevisions(ctx, "3", params)
			return err
		}},
		{name: "GetRevision", method: http.MethodGet, path: "/v1/articles/3/revisions/4", call: func(c *DocsClient) error {
			_, err := c.GetRevision(ctx, "3", "4")
			return err
		}},
		{name: "CreateArticle", method: http.MethodPost, path: "/v1/articles", call: func(c *DocsClient) error {
			_, err := c.CreateArticle(ctx, body)
			return err
		}},
		{name: "UpdateArticle", method: http.MethodPut, path: "/v1/articles/3", call: func(c *DocsClient) error {
			return c.UpdateArticle(ctx, "3", body)
		}},
		{name: "DeleteArticle", method: http.MethodDelete, path: "/v1/articles/3", call: func(c *DocsClient) error {
			return c.DeleteArticle(ctx, "3")
		}},
		{name: "UploadArticleAsset", method: http.MethodPost, path: "/v1/articles/3/assets", multipart: true, call: func(c *DocsClient) error {
			_, err := c.UploadArticleAsset(ctx, "3", assetPath)
			return err
		}},
		{name: "UpdateArticleViewCount", method: http.MethodPut, path: "/v1/articles/3/views", call: func(c *DocsClient) error {
			return c.UpdateArticleViewCount(ctx, "3", body)
		}},
		{name: "SaveArticleDraft", method: http.MethodPost, path: "/v1/articles/3/drafts", call: func(c *DocsClient) error {
			return c.SaveArticleDraft(ctx, "3", body)
		}},
		{name: "DeleteArticleDraft", method: http.MethodDelete, path: "/v1/articles/3/drafts", call: func(c *DocsClient) error {
			return c.DeleteArticleDraft(ctx, "3")
		}},
		{name: "ListSites", method: http.MethodGet, path: "/v1/sites", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListSites(ctx, params)
			return err
		}},
		{name: "GetSite", method: http.MethodGet, path: "/v1/sites/5", call: func(c *DocsClient) error {
			_, err := c.GetSite(ctx, "5")
			return err
		}},
		{name: "CreateSite", method: http.MethodPost, path: "/v1/sites", call: func(c *DocsClient) error {
			_, err := c.CreateSite(ctx, body)
			return err
		}},
		{name: "UpdateSite", method: http.MethodPut, path: "/v1/sites/5", call: func(c *DocsClient) error {
			return c.UpdateSite(ctx, "5", body)
		}},
		{name: "DeleteSite", method: http.MethodDelete, path: "/v1/sites/5", call: func(c *DocsClient) error {
			return c.DeleteSite(ctx, "5")
		}},
		{name: "GetSiteRestrictions", method: http.MethodGet, path: "/v1/sites/5/restrictions", call: func(c *DocsClient) error {
			_, err := c.GetSiteRestrictions(ctx, "5")
			return err
		}},
		{name: "UpdateSiteRestrictions", method: http.MethodPut, path: "/v1/sites/5/restrictions", call: func(c *DocsClient) error {
			return c.UpdateSiteRestrictions(ctx, "5", body)
		}},
		{name: "ListRedirects", method: http.MethodGet, path: "/v1/redirects/site/5", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.ListRedirects(ctx, "5", params)
			return err
		}},
		{name: "FindRedirect", method: http.MethodGet, path: "/v1/redirects", query: "page=2", call: func(c *DocsClient) error {
			_, err := c.FindRedirect(ctx, params)
			return err
		}},
		{name: "GetRedirect", method: http.MethodGet, path: "/v1/redirects/6", call: func(c *DocsClient) error {
			_, err := c.GetRedirect(ctx, "6")
			return err
		}},
		{name: "CreateRedirect", method: http.MethodPost, path: "/v1/redirects", call: func(c *DocsClient) error {
			_, err := c.CreateRedirect(ctx, body)
			return err
		}},
		{name: "UpdateRedirect", method: http.MethodPut, path: "/v1/redirects/6", call: func(c *DocsClient) error {
			return c.UpdateRedirect(ctx, "6", body)
		}},
		{name: "DeleteRedirect", method: http.MethodDelete, path: "/v1/redirects/6", call: func(c *DocsClient) error {
			return c.DeleteRedirect(ctx, "6")
		}},
		{name: "UploadArticleSettingsAsset", method: http.MethodPost, path: "/v1/assets/article", multipart: true, call: func(c *DocsClient) error {
			_, err := c.UploadArticleSettingsAsset(ctx, assetPath)
			return err
		}},
		{name: "UploadSettingsAsset", method: http.MethodPost, path: "/v1/assets/settings", multipart: true, call: func(c *DocsClient) error {
			_, err := c.UploadSettingsAsset(ctx, assetPath)
			return err
		}},
	}

	names := make([]string, 0, len(tests))
	for _, tt := range tests {
		names = append(names, tt.name)
	}
	assertEndpointCasesCoverInterface(t, (*DocsClientAPI)(nil), names)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotQuery, gotContentType string
			c := docsClientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				gotContentType = r.Header.Get("Content-Type")

				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet || r.Method == http.MethodPost {
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id": 42}))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})

			require.NoError(t, tt.call(c))
			assert.Equal(t, tt.method, gotMethod)
			assert.Equal(t, tt.path, gotPath)
			assert.Equal(t, tt.query, gotQuery)
			switch {
			case tt.multipart:
				assert.True(t, strings.HasPrefix(gotContentType, "multipart/form-data"))
			case tt.method == http.MethodPost || tt.method == http.MethodPut:
				assert.Equal(t, "application/json", gotContentType)
			default:
				assert.Empty(t, gotContentType)
			}
		})
	}
}
