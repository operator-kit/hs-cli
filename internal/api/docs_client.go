package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const docsBaseURL = "https://docsapi.helpscout.net/v1"

type DocsClient struct {
	http    *http.Client
	limiter *rate.Limiter
	apiKey  string
	debug   bool
}

func NewDocs(apiKey string, debug bool) *DocsClient {
	httpClient := &http.Client{}
	c := &DocsClient{
		http:    httpClient,
		limiter: rate.NewLimiter(rate.Every(10*time.Minute/2000), 20), // 2000/10min, burst 20
		apiKey:  apiKey,
		debug:   debug,
	}
	if debug {
		setupDebugLog(httpClient)
	}
	return c
}

// NewDocsForTest creates a DocsClient with a custom http.Client and no rate limiter.
func NewDocsForTest(httpClient *http.Client, apiKey string) *DocsClient {
	return &DocsClient{
		http:    httpClient,
		limiter: rate.NewLimiter(rate.Inf, 0),
		apiKey:  apiKey,
	}
}

func (c *DocsClient) get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, params, nil)
}

func (c *DocsClient) post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doRaw(ctx, http.MethodPost, path, nil, body)
}

func (c *DocsClient) put(ctx context.Context, path string, body any) error {
	resp, err := c.doRaw(ctx, http.MethodPut, path, nil, body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *DocsClient) delete(ctx context.Context, path string) error {
	resp, err := c.doRaw(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *DocsClient) do(ctx context.Context, method, path string, params url.Values, body any) (json.RawMessage, error) {
	resp, err := c.doRaw(ctx, method, path, params, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *DocsClient) doRaw(ctx context.Context, method, path string, params url.Values, body any) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	u := docsBaseURL + "/" + strings.TrimPrefix(path, "/")
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding body: %w", err)
		}
		bodyReader = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.apiKey, "X")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		retry := resp.Header.Get("Retry-After")
		secs, _ := strconv.Atoi(retry)
		if secs == 0 {
			secs = 10
		}
		select {
		case <-time.After(time.Duration(secs) * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return c.doRaw(ctx, method, path, params, body)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, formatDocsAPIError(resp.StatusCode, data))
	}

	return resp, nil
}

// doMultipart sends a multipart/form-data request for file uploads.
func (c *DocsClient) doMultipart(ctx context.Context, path, fieldName, filePath string) (json.RawMessage, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copying file: %w", err)
	}
	w.Close()

	u := docsBaseURL + "/" + strings.TrimPrefix(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &buf)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.apiKey, "X")
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, formatDocsAPIError(resp.StatusCode, data))
	}
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

// formatDocsAPIError extracts a human-readable message from a Docs API error.
// Docs errors use {"code":404,"error":"Not found"} format.
func formatDocsAPIError(statusCode int, data []byte) string {
	if len(data) == 0 {
		return http.StatusText(statusCode)
	}
	var body struct {
		Code    int    `json:"code"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return string(data)
	}
	if body.Error != "" {
		return body.Error
	}
	if body.Message != "" {
		return body.Message
	}
	return string(data)
}

// --- Public resource methods ---

// Collections

func (c *DocsClient) ListCollections(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "collections", params)
}

func (c *DocsClient) GetCollection(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "collections/"+id, nil)
}

func (c *DocsClient) CreateCollection(ctx context.Context, body any) (json.RawMessage, error) {
	resp, err := c.post(ctx, "collections", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *DocsClient) UpdateCollection(ctx context.Context, id string, body any) error {
	return c.put(ctx, "collections/"+id, body)
}

func (c *DocsClient) DeleteCollection(ctx context.Context, id string) error {
	return c.delete(ctx, "collections/"+id)
}

// Categories

func (c *DocsClient) ListCategories(ctx context.Context, collectionID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "collections/"+collectionID+"/categories", params)
}

func (c *DocsClient) GetCategory(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "categories/"+id, nil)
}

func (c *DocsClient) CreateCategory(ctx context.Context, body any) (json.RawMessage, error) {
	resp, err := c.post(ctx, "categories", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *DocsClient) UpdateCategory(ctx context.Context, id string, body any) error {
	return c.put(ctx, "categories/"+id, body)
}

func (c *DocsClient) ReorderCategory(ctx context.Context, collectionID string, body any) error {
	return c.put(ctx, "collections/"+collectionID+"/categories/order", body)
}

func (c *DocsClient) DeleteCategory(ctx context.Context, id string) error {
	return c.delete(ctx, "categories/"+id)
}

// Articles

func (c *DocsClient) ListArticles(ctx context.Context, collectionID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "collections/"+collectionID+"/articles", params)
}

func (c *DocsClient) ListArticlesByCategory(ctx context.Context, categoryID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "categories/"+categoryID+"/articles", params)
}

func (c *DocsClient) SearchArticles(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "search/articles", params)
}

func (c *DocsClient) GetArticle(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "articles/"+id, params)
}

func (c *DocsClient) GetRelatedArticles(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "articles/"+id+"/related", params)
}

func (c *DocsClient) ListRevisions(ctx context.Context, articleID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "articles/"+articleID+"/revisions", params)
}

func (c *DocsClient) GetRevision(ctx context.Context, articleID, revisionID string) (json.RawMessage, error) {
	return c.get(ctx, "articles/"+articleID+"/revisions/"+revisionID, nil)
}

func (c *DocsClient) CreateArticle(ctx context.Context, body any) (json.RawMessage, error) {
	resp, err := c.post(ctx, "articles", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *DocsClient) UpdateArticle(ctx context.Context, id string, body any) error {
	return c.put(ctx, "articles/"+id, body)
}

func (c *DocsClient) DeleteArticle(ctx context.Context, id string) error {
	return c.delete(ctx, "articles/"+id)
}

func (c *DocsClient) UploadArticleAsset(ctx context.Context, articleID, filePath string) (json.RawMessage, error) {
	return c.doMultipart(ctx, "articles/"+articleID+"/assets", "file", filePath)
}

func (c *DocsClient) UpdateArticleViewCount(ctx context.Context, id string, body any) error {
	return c.put(ctx, "articles/"+id+"/views", body)
}

func (c *DocsClient) SaveArticleDraft(ctx context.Context, articleID string, body any) error {
	resp, err := c.post(ctx, "articles/"+articleID+"/drafts", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *DocsClient) DeleteArticleDraft(ctx context.Context, articleID string) error {
	return c.delete(ctx, "articles/"+articleID+"/drafts")
}

// Sites

func (c *DocsClient) ListSites(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "sites", params)
}

func (c *DocsClient) GetSite(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "sites/"+id, nil)
}

func (c *DocsClient) CreateSite(ctx context.Context, body any) (json.RawMessage, error) {
	resp, err := c.post(ctx, "sites", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *DocsClient) UpdateSite(ctx context.Context, id string, body any) error {
	return c.put(ctx, "sites/"+id, body)
}

func (c *DocsClient) DeleteSite(ctx context.Context, id string) error {
	return c.delete(ctx, "sites/"+id)
}

func (c *DocsClient) GetSiteRestrictions(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "sites/"+id+"/restrictions", nil)
}

func (c *DocsClient) UpdateSiteRestrictions(ctx context.Context, id string, body any) error {
	return c.put(ctx, "sites/"+id+"/restrictions", body)
}

// Redirects

func (c *DocsClient) ListRedirects(ctx context.Context, siteID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "redirects/site/"+siteID, params)
}

func (c *DocsClient) FindRedirect(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "redirects", params)
}

func (c *DocsClient) GetRedirect(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "redirects/"+id, nil)
}

func (c *DocsClient) CreateRedirect(ctx context.Context, body any) (json.RawMessage, error) {
	resp, err := c.post(ctx, "redirects", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if len(data) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(data), nil
}

func (c *DocsClient) UpdateRedirect(ctx context.Context, id string, body any) error {
	return c.put(ctx, "redirects/"+id, body)
}

func (c *DocsClient) DeleteRedirect(ctx context.Context, id string) error {
	return c.delete(ctx, "redirects/"+id)
}

// Assets

func (c *DocsClient) UploadArticleSettingsAsset(ctx context.Context, filePath string) (json.RawMessage, error) {
	return c.doMultipart(ctx, "assets/article", "file", filePath)
}

func (c *DocsClient) UploadSettingsAsset(ctx context.Context, filePath string) (json.RawMessage, error) {
	return c.doMultipart(ctx, "assets/settings", "file", filePath)
}
