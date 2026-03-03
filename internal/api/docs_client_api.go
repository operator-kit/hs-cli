package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// DocsClientAPI defines the public interface for the HelpScout Docs API client.
type DocsClientAPI interface {
	// Collections
	ListCollections(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetCollection(ctx context.Context, id string) (json.RawMessage, error)
	CreateCollection(ctx context.Context, body any) (json.RawMessage, error)
	UpdateCollection(ctx context.Context, id string, body any) error
	DeleteCollection(ctx context.Context, id string) error

	// Categories
	ListCategories(ctx context.Context, collectionID string, params url.Values) (json.RawMessage, error)
	GetCategory(ctx context.Context, id string) (json.RawMessage, error)
	CreateCategory(ctx context.Context, body any) (json.RawMessage, error)
	UpdateCategory(ctx context.Context, id string, body any) error
	ReorderCategory(ctx context.Context, collectionID string, body any) error
	DeleteCategory(ctx context.Context, id string) error

	// Articles
	ListArticles(ctx context.Context, collectionID string, params url.Values) (json.RawMessage, error)
	ListArticlesByCategory(ctx context.Context, categoryID string, params url.Values) (json.RawMessage, error)
	SearchArticles(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetArticle(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	GetRelatedArticles(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	ListRevisions(ctx context.Context, articleID string, params url.Values) (json.RawMessage, error)
	GetRevision(ctx context.Context, articleID, revisionID string) (json.RawMessage, error)
	CreateArticle(ctx context.Context, body any) (json.RawMessage, error)
	UpdateArticle(ctx context.Context, id string, body any) error
	DeleteArticle(ctx context.Context, id string) error
	UploadArticleAsset(ctx context.Context, articleID, filePath string) (json.RawMessage, error)
	UpdateArticleViewCount(ctx context.Context, id string, body any) error
	SaveArticleDraft(ctx context.Context, articleID string, body any) error
	DeleteArticleDraft(ctx context.Context, articleID string) error

	// Sites
	ListSites(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetSite(ctx context.Context, id string) (json.RawMessage, error)
	CreateSite(ctx context.Context, body any) (json.RawMessage, error)
	UpdateSite(ctx context.Context, id string, body any) error
	DeleteSite(ctx context.Context, id string) error
	GetSiteRestrictions(ctx context.Context, id string) (json.RawMessage, error)
	UpdateSiteRestrictions(ctx context.Context, id string, body any) error

	// Redirects
	ListRedirects(ctx context.Context, siteID string, params url.Values) (json.RawMessage, error)
	FindRedirect(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetRedirect(ctx context.Context, id string) (json.RawMessage, error)
	CreateRedirect(ctx context.Context, body any) (json.RawMessage, error)
	UpdateRedirect(ctx context.Context, id string, body any) error
	DeleteRedirect(ctx context.Context, id string) error

	// Assets
	UploadArticleSettingsAsset(ctx context.Context, filePath string) (json.RawMessage, error)
	UploadSettingsAsset(ctx context.Context, filePath string) (json.RawMessage, error)
}
