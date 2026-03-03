package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/operator-kit/hs-cli/internal/auth"
)

const baseURL = "https://api.helpscout.net/v2"

type Client struct {
	http    *http.Client
	limiter *rate.Limiter
	debug   bool
}

func New(ctx context.Context, clientID, clientSecret string, debug bool) *Client {
	httpClient := auth.HTTPClient(ctx, clientID, clientSecret)
	c := &Client{
		http:    httpClient,
		limiter: rate.NewLimiter(rate.Every(time.Minute/200), 10), // 200/min, burst 10
		debug:   debug,
	}
	if debug {
		setupDebugLog(httpClient)
	}
	return c
}

// NewForTest creates a Client with a custom http.Client and no rate limiter.
// Used by httptest-based tests only.
func NewForTest(httpClient *http.Client) *Client {
	return &Client{
		http:    httpClient,
		limiter: rate.NewLimiter(rate.Inf, 0),
	}
}

func (c *Client) get(ctx context.Context, path string, params url.Values) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, path, params, nil)
}

func (c *Client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.doRaw(ctx, http.MethodPost, path, nil, body)
}

func (c *Client) put(ctx context.Context, path string, body any) error {
	resp, err := c.doRaw(ctx, http.MethodPut, path, nil, body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) patch(ctx context.Context, path string, body any) error {
	resp, err := c.doRaw(ctx, http.MethodPatch, path, nil, body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) delete(ctx context.Context, path string) error {
	resp, err := c.doRaw(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body any) (json.RawMessage, error) {
	resp, err := c.doRaw(ctx, method, path, params, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	return json.RawMessage(data), nil
}

func (c *Client) doRaw(ctx context.Context, method, path string, params url.Values, body any) (*http.Response, error) {
	return c.doRawWithHeaders(ctx, method, path, params, body, nil)
}

func (c *Client) doRawWithHeaders(ctx context.Context, method, path string, params url.Values, body any, headers map[string]string) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}

	u := baseURL + "/" + strings.TrimPrefix(path, "/")
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
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
		return c.doRawWithHeaders(ctx, method, path, params, body, headers)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, formatAPIError(resp.StatusCode, data))
	}

	return resp, nil
}

// --- Public resource methods ---

// ListMailboxes returns the raw JSON for mailboxes list.
func (c *Client) ListMailboxes(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "mailboxes", params)
}

func (c *Client) GetMailbox(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "mailboxes/"+id, nil)
}

func (c *Client) ListMailboxFolders(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "mailboxes/"+mailboxID+"/folders", params)
}

func (c *Client) ListMailboxCustomFields(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "mailboxes/"+mailboxID+"/custom-fields", params)
}

func (c *Client) GetMailboxRouting(ctx context.Context, mailboxID string) (json.RawMessage, error) {
	return c.get(ctx, "mailboxes/"+mailboxID+"/routing", nil)
}

func (c *Client) UpdateMailboxRouting(ctx context.Context, mailboxID string, body any) error {
	return c.put(ctx, "mailboxes/"+mailboxID+"/routing", body)
}

func (c *Client) ListConversations(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "conversations", params)
}

func (c *Client) GetConversation(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "conversations/"+id, params)
}

func (c *Client) CreateConversation(ctx context.Context, body any) (string, error) {
	resp, err := c.post(ctx, "conversations", body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return extractIDFromLocation(resp), nil
}

func (c *Client) UpdateConversation(ctx context.Context, id string, body any) error {
	return c.patch(ctx, "conversations/"+id, body)
}

func (c *Client) UpdateConversationFields(ctx context.Context, id string, body any) error {
	return c.patch(ctx, "conversations/"+id+"/fields", body)
}

func (c *Client) UpdateConversationTags(ctx context.Context, id string, body any) error {
	resp, err := c.post(ctx, "conversations/"+id+"/tags", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) UpdateConversationSnooze(ctx context.Context, id string, body any) error {
	return c.patch(ctx, "conversations/"+id+"/snooze", body)
}

func (c *Client) DeleteConversationSnooze(ctx context.Context, id string) error {
	return c.delete(ctx, "conversations/"+id+"/snooze")
}

func (c *Client) DeleteConversation(ctx context.Context, id string) error {
	return c.delete(ctx, "conversations/"+id)
}

func (c *Client) CreateAttachment(ctx context.Context, convID string, threadID string, body any) error {
	resp, err := c.post(ctx, "conversations/"+convID+"/threads/"+threadID+"/attachments", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) GetAttachmentData(ctx context.Context, convID string, attachmentID string) (json.RawMessage, error) {
	return c.get(ctx, "conversations/"+convID+"/attachments/"+attachmentID+"/data", nil)
}

func (c *Client) DeleteAttachment(ctx context.Context, convID string, attachmentID string) error {
	return c.delete(ctx, "conversations/"+convID+"/attachments/"+attachmentID)
}

func (c *Client) ListThreads(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "conversations/"+convID+"/threads", params)
}

func (c *Client) CreateReply(ctx context.Context, convID string, body any) error {
	resp, err := c.post(ctx, "conversations/"+convID+"/reply", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) CreateNote(ctx context.Context, convID string, body any) error {
	resp, err := c.post(ctx, "conversations/"+convID+"/notes", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) CreateChatThread(ctx context.Context, convID string, body any) error {
	resp, err := c.post(ctx, "conversations/"+convID+"/chats", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) CreateCustomerThread(ctx context.Context, convID string, body any) error {
	resp, err := c.post(ctx, "conversations/"+convID+"/customer", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) CreatePhoneThread(ctx context.Context, convID string, body any) error {
	resp, err := c.post(ctx, "conversations/"+convID+"/phones", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) UpdateThread(ctx context.Context, convID string, threadID string, body any) error {
	return c.patch(ctx, "conversations/"+convID+"/threads/"+threadID, body)
}

func (c *Client) GetThreadSource(ctx context.Context, convID string, threadID string) (json.RawMessage, error) {
	return c.get(ctx, "conversations/"+convID+"/threads/"+threadID+"/source", nil)
}

func (c *Client) GetThreadSourceRFC822(ctx context.Context, convID string, threadID string) ([]byte, error) {
	resp, err := c.doRawWithHeaders(ctx, http.MethodGet, "conversations/"+convID+"/threads/"+threadID+"/source", nil, nil, map[string]string{
		"Accept": "message/rfc822",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) ListCustomers(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "customers", params)
}

func (c *Client) GetCustomer(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "customers/"+id, params)
}

func (c *Client) CreateCustomer(ctx context.Context, body any) (string, error) {
	resp, err := c.post(ctx, "customers", body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return extractIDFromLocation(resp), nil
}

func (c *Client) UpdateCustomer(ctx context.Context, id string, body any) error {
	return c.patch(ctx, "customers/"+id, body)
}

func (c *Client) OverwriteCustomer(ctx context.Context, id string, body any) error {
	return c.put(ctx, "customers/"+id, body)
}

func (c *Client) DeleteCustomer(ctx context.Context, id string, params url.Values) error {
	resp, err := c.doRaw(ctx, http.MethodDelete, "customers/"+id, params, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) ListTags(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "tags", params)
}

func (c *Client) GetTag(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "tags/"+id, nil)
}

func (c *Client) ListUsers(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "users", params)
}

func (c *Client) GetUser(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "users/"+id, nil)
}

func (c *Client) GetResourceOwner(ctx context.Context) (json.RawMessage, error) {
	return c.get(ctx, "users/me", nil)
}

func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.delete(ctx, "users/"+id)
}

func (c *Client) ListUserStatuses(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "users/status", params)
}

func (c *Client) GetUserStatus(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "users/"+id+"/status", nil)
}

func (c *Client) SetUserStatus(ctx context.Context, id string, body any) error {
	return c.put(ctx, "users/"+id+"/status", body)
}

func (c *Client) ListTeams(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "teams", params)
}

func (c *Client) ListTeamMembers(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "teams/"+id+"/members", params)
}

func (c *Client) ListWorkflows(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "workflows", params)
}

func (c *Client) UpdateWorkflowStatus(ctx context.Context, id string, body any) error {
	return c.patch(ctx, "workflows/"+id, body)
}

func (c *Client) RunWorkflow(ctx context.Context, id string, body any) error {
	resp, err := c.post(ctx, "workflows/"+id+"/run", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *Client) ListWebhooks(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "webhooks", params)
}

func (c *Client) GetWebhook(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "webhooks/"+id, nil)
}

func (c *Client) CreateWebhook(ctx context.Context, body any) (string, error) {
	resp, err := c.post(ctx, "webhooks", body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return extractIDFromLocation(resp), nil
}

func (c *Client) UpdateWebhook(ctx context.Context, id string, body any) error {
	return c.put(ctx, "webhooks/"+id, body)
}

func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	return c.delete(ctx, "webhooks/"+id)
}

func (c *Client) ListSavedReplies(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "saved-replies", params)
}

func (c *Client) GetSavedReply(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "saved-replies/"+id, nil)
}

func (c *Client) CreateSavedReply(ctx context.Context, body any) (string, error) {
	resp, err := c.post(ctx, "saved-replies", body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return extractIDFromLocation(resp), nil
}

func (c *Client) UpdateSavedReply(ctx context.Context, id string, body any) error {
	return c.put(ctx, "saved-replies/"+id, body)
}

func (c *Client) DeleteSavedReply(ctx context.Context, id string) error {
	return c.delete(ctx, "saved-replies/"+id)
}

func (c *Client) ListOrganizations(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "organizations", params)
}

func (c *Client) GetOrganization(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "organizations/"+id, nil)
}

func (c *Client) CreateOrganization(ctx context.Context, body any) (string, error) {
	resp, err := c.post(ctx, "organizations", body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return extractIDFromLocation(resp), nil
}

func (c *Client) UpdateOrganization(ctx context.Context, id string, body any) error {
	return c.put(ctx, "organizations/"+id, body)
}

func (c *Client) DeleteOrganization(ctx context.Context, id string) error {
	return c.delete(ctx, "organizations/"+id)
}

func (c *Client) ListOrganizationConversations(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "organizations/"+id+"/conversations", params)
}

func (c *Client) ListOrganizationCustomers(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "organizations/"+id+"/customers", params)
}

func (c *Client) ListOrganizationProperties(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "organizations/properties", params)
}

func (c *Client) GetOrganizationProperty(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "organizations/properties/"+id, nil)
}

func (c *Client) CreateOrganizationProperty(ctx context.Context, body any) (string, error) {
	resp, err := c.post(ctx, "organizations/properties", body)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	return extractIDFromLocation(resp), nil
}

func (c *Client) UpdateOrganizationProperty(ctx context.Context, id string, body any) error {
	return c.put(ctx, "organizations/properties/"+id, body)
}

func (c *Client) DeleteOrganizationProperty(ctx context.Context, id string) error {
	return c.delete(ctx, "organizations/properties/"+id)
}

func (c *Client) ListCustomerProperties(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "customer-properties", params)
}

func (c *Client) GetCustomerProperty(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "customer-properties/"+id, nil)
}

func (c *Client) ListConversationProperties(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "conversation-properties", params)
}

func (c *Client) GetConversationProperty(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "conversation-properties/"+id, nil)
}

func (c *Client) GetRating(ctx context.Context, id string) (json.RawMessage, error) {
	return c.get(ctx, "ratings/"+id, nil)
}

func (c *Client) GetReport(ctx context.Context, family string, params url.Values) (json.RawMessage, error) {
	return c.get(ctx, "reports/"+strings.TrimPrefix(family, "/"), params)
}

// formatAPIError extracts a human-readable message from a HelpScout error response.
func formatAPIError(statusCode int, data []byte) string {
	if len(data) == 0 {
		return http.StatusText(statusCode)
	}

	var body struct {
		Message  string `json:"message"`
		Embedded struct {
			Errors []struct {
				Path    string `json:"path"`
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"_embedded"`
	}
	if err := json.Unmarshal(data, &body); err != nil || body.Message == "" {
		return string(data)
	}

	msg := body.Message
	for _, e := range body.Embedded.Errors {
		if e.Path != "" && e.Message != "" {
			msg += fmt.Sprintf(" (%s: %s)", e.Path, e.Message)
		}
	}
	return msg
}

// extractIDFromLocation parses resource ID from Location header (e.g. ".../123").
func extractIDFromLocation(resp *http.Response) string {
	loc := resp.Header.Get("Location")
	if loc == "" {
		return ""
	}
	// resource ID is the last path segment
	parts := strings.Split(strings.TrimRight(loc, "/"), "/")
	return parts[len(parts)-1]
}
