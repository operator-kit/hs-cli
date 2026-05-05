package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientEndpointMethods(t *testing.T) {
	ctx := context.Background()
	body := map[string]any{"name": "test"}
	params := url.Values{"page": []string{"2"}}

	tests := []struct {
		name   string
		method string
		path   string
		query  string
		call   func(*Client) error
	}{
		{name: "ListMailboxes", method: http.MethodGet, path: "/v2/mailboxes", query: "page=2", call: func(c *Client) error {
			_, err := c.ListMailboxes(ctx, params)
			return err
		}},
		{name: "GetMailbox", method: http.MethodGet, path: "/v2/mailboxes/123", call: func(c *Client) error {
			_, err := c.GetMailbox(ctx, "123")
			return err
		}},
		{name: "ListMailboxFolders", method: http.MethodGet, path: "/v2/mailboxes/123/folders", query: "page=2", call: func(c *Client) error {
			_, err := c.ListMailboxFolders(ctx, "123", params)
			return err
		}},
		{name: "ListMailboxCustomFields", method: http.MethodGet, path: "/v2/mailboxes/123/custom-fields", query: "page=2", call: func(c *Client) error {
			_, err := c.ListMailboxCustomFields(ctx, "123", params)
			return err
		}},
		{name: "GetMailboxRouting", method: http.MethodGet, path: "/v2/mailboxes/123/routing", call: func(c *Client) error {
			_, err := c.GetMailboxRouting(ctx, "123")
			return err
		}},
		{name: "UpdateMailboxRouting", method: http.MethodPut, path: "/v2/mailboxes/123/routing", call: func(c *Client) error {
			return c.UpdateMailboxRouting(ctx, "123", body)
		}},
		{name: "ListConversations", method: http.MethodGet, path: "/v2/conversations", query: "page=2", call: func(c *Client) error {
			_, err := c.ListConversations(ctx, params)
			return err
		}},
		{name: "GetConversation", method: http.MethodGet, path: "/v2/conversations/456", query: "page=2", call: func(c *Client) error {
			_, err := c.GetConversation(ctx, "456", params)
			return err
		}},
		{name: "CreateConversation", method: http.MethodPost, path: "/v2/conversations", call: func(c *Client) error {
			_, err := c.CreateConversation(ctx, body)
			return err
		}},
		{name: "UpdateConversation", method: http.MethodPatch, path: "/v2/conversations/456", call: func(c *Client) error {
			return c.UpdateConversation(ctx, "456", body)
		}},
		{name: "UpdateConversationFields", method: http.MethodPatch, path: "/v2/conversations/456/fields", call: func(c *Client) error {
			return c.UpdateConversationFields(ctx, "456", body)
		}},
		{name: "UpdateConversationTags", method: http.MethodPut, path: "/v2/conversations/456/tags", call: func(c *Client) error {
			return c.UpdateConversationTags(ctx, "456", body)
		}},
		{name: "UpdateConversationSnooze", method: http.MethodPatch, path: "/v2/conversations/456/snooze", call: func(c *Client) error {
			return c.UpdateConversationSnooze(ctx, "456", body)
		}},
		{name: "DeleteConversationSnooze", method: http.MethodDelete, path: "/v2/conversations/456/snooze", call: func(c *Client) error {
			return c.DeleteConversationSnooze(ctx, "456")
		}},
		{name: "DeleteConversation", method: http.MethodDelete, path: "/v2/conversations/456", call: func(c *Client) error {
			return c.DeleteConversation(ctx, "456")
		}},
		{name: "CreateAttachment", method: http.MethodPost, path: "/v2/conversations/456/threads/789/attachments", call: func(c *Client) error {
			return c.CreateAttachment(ctx, "456", "789", body)
		}},
		{name: "GetAttachmentData", method: http.MethodGet, path: "/v2/conversations/456/attachments/101/data", call: func(c *Client) error {
			_, err := c.GetAttachmentData(ctx, "456", "101")
			return err
		}},
		{name: "DeleteAttachment", method: http.MethodDelete, path: "/v2/conversations/456/attachments/101", call: func(c *Client) error {
			return c.DeleteAttachment(ctx, "456", "101")
		}},
		{name: "ListThreads", method: http.MethodGet, path: "/v2/conversations/456/threads", query: "page=2", call: func(c *Client) error {
			_, err := c.ListThreads(ctx, "456", params)
			return err
		}},
		{name: "CreateReply", method: http.MethodPost, path: "/v2/conversations/456/reply", call: func(c *Client) error {
			return c.CreateReply(ctx, "456", body)
		}},
		{name: "CreateNote", method: http.MethodPost, path: "/v2/conversations/456/notes", call: func(c *Client) error {
			return c.CreateNote(ctx, "456", body)
		}},
		{name: "CreateChatThread", method: http.MethodPost, path: "/v2/conversations/456/chats", call: func(c *Client) error {
			return c.CreateChatThread(ctx, "456", body)
		}},
		{name: "CreateCustomerThread", method: http.MethodPost, path: "/v2/conversations/456/customer", call: func(c *Client) error {
			return c.CreateCustomerThread(ctx, "456", body)
		}},
		{name: "CreatePhoneThread", method: http.MethodPost, path: "/v2/conversations/456/phones", call: func(c *Client) error {
			return c.CreatePhoneThread(ctx, "456", body)
		}},
		{name: "UpdateThread", method: http.MethodPatch, path: "/v2/conversations/456/threads/789", call: func(c *Client) error {
			return c.UpdateThread(ctx, "456", "789", body)
		}},
		{name: "GetThreadSource", method: http.MethodGet, path: "/v2/conversations/456/threads/789/source", call: func(c *Client) error {
			_, err := c.GetThreadSource(ctx, "456", "789")
			return err
		}},
		{name: "GetThreadSourceRFC822", method: http.MethodGet, path: "/v2/conversations/456/threads/789/source", call: func(c *Client) error {
			_, err := c.GetThreadSourceRFC822(ctx, "456", "789")
			return err
		}},
		{name: "ListCustomers", method: http.MethodGet, path: "/v2/customers", query: "page=2", call: func(c *Client) error {
			_, err := c.ListCustomers(ctx, params)
			return err
		}},
		{name: "GetCustomer", method: http.MethodGet, path: "/v2/customers/321", query: "page=2", call: func(c *Client) error {
			_, err := c.GetCustomer(ctx, "321", params)
			return err
		}},
		{name: "CreateCustomer", method: http.MethodPost, path: "/v2/customers", call: func(c *Client) error {
			_, err := c.CreateCustomer(ctx, body)
			return err
		}},
		{name: "UpdateCustomer", method: http.MethodPatch, path: "/v2/customers/321", call: func(c *Client) error {
			return c.UpdateCustomer(ctx, "321", body)
		}},
		{name: "OverwriteCustomer", method: http.MethodPut, path: "/v2/customers/321", call: func(c *Client) error {
			return c.OverwriteCustomer(ctx, "321", body)
		}},
		{name: "DeleteCustomer", method: http.MethodDelete, path: "/v2/customers/321", query: "page=2", call: func(c *Client) error {
			return c.DeleteCustomer(ctx, "321", params)
		}},
		{name: "ListTags", method: http.MethodGet, path: "/v2/tags", query: "page=2", call: func(c *Client) error {
			_, err := c.ListTags(ctx, params)
			return err
		}},
		{name: "GetTag", method: http.MethodGet, path: "/v2/tags/11", call: func(c *Client) error {
			_, err := c.GetTag(ctx, "11")
			return err
		}},
		{name: "ListUsers", method: http.MethodGet, path: "/v2/users", query: "page=2", call: func(c *Client) error {
			_, err := c.ListUsers(ctx, params)
			return err
		}},
		{name: "GetUser", method: http.MethodGet, path: "/v2/users/12", call: func(c *Client) error {
			_, err := c.GetUser(ctx, "12")
			return err
		}},
		{name: "GetResourceOwner", method: http.MethodGet, path: "/v2/users/me", call: func(c *Client) error {
			_, err := c.GetResourceOwner(ctx)
			return err
		}},
		{name: "DeleteUser", method: http.MethodDelete, path: "/v2/users/12", call: func(c *Client) error {
			return c.DeleteUser(ctx, "12")
		}},
		{name: "ListUserStatuses", method: http.MethodGet, path: "/v2/users/status", query: "page=2", call: func(c *Client) error {
			_, err := c.ListUserStatuses(ctx, params)
			return err
		}},
		{name: "GetUserStatus", method: http.MethodGet, path: "/v2/users/12/status", call: func(c *Client) error {
			_, err := c.GetUserStatus(ctx, "12")
			return err
		}},
		{name: "SetUserStatus", method: http.MethodPut, path: "/v2/users/12/status", call: func(c *Client) error {
			return c.SetUserStatus(ctx, "12", body)
		}},
		{name: "ListTeams", method: http.MethodGet, path: "/v2/teams", query: "page=2", call: func(c *Client) error {
			_, err := c.ListTeams(ctx, params)
			return err
		}},
		{name: "ListTeamMembers", method: http.MethodGet, path: "/v2/teams/13/members", query: "page=2", call: func(c *Client) error {
			_, err := c.ListTeamMembers(ctx, "13", params)
			return err
		}},
		{name: "ListWorkflows", method: http.MethodGet, path: "/v2/workflows", query: "page=2", call: func(c *Client) error {
			_, err := c.ListWorkflows(ctx, params)
			return err
		}},
		{name: "UpdateWorkflowStatus", method: http.MethodPatch, path: "/v2/workflows/14", call: func(c *Client) error {
			return c.UpdateWorkflowStatus(ctx, "14", body)
		}},
		{name: "RunWorkflow", method: http.MethodPost, path: "/v2/workflows/14/run", call: func(c *Client) error {
			return c.RunWorkflow(ctx, "14", body)
		}},
		{name: "ListWebhooks", method: http.MethodGet, path: "/v2/webhooks", query: "page=2", call: func(c *Client) error {
			_, err := c.ListWebhooks(ctx, params)
			return err
		}},
		{name: "GetWebhook", method: http.MethodGet, path: "/v2/webhooks/15", call: func(c *Client) error {
			_, err := c.GetWebhook(ctx, "15")
			return err
		}},
		{name: "CreateWebhook", method: http.MethodPost, path: "/v2/webhooks", call: func(c *Client) error {
			_, err := c.CreateWebhook(ctx, body)
			return err
		}},
		{name: "UpdateWebhook", method: http.MethodPut, path: "/v2/webhooks/15", call: func(c *Client) error {
			return c.UpdateWebhook(ctx, "15", body)
		}},
		{name: "DeleteWebhook", method: http.MethodDelete, path: "/v2/webhooks/15", call: func(c *Client) error {
			return c.DeleteWebhook(ctx, "15")
		}},
		{name: "ListSavedReplies", method: http.MethodGet, path: "/v2/saved-replies", query: "page=2", call: func(c *Client) error {
			_, err := c.ListSavedReplies(ctx, params)
			return err
		}},
		{name: "GetSavedReply", method: http.MethodGet, path: "/v2/saved-replies/16", call: func(c *Client) error {
			_, err := c.GetSavedReply(ctx, "16")
			return err
		}},
		{name: "CreateSavedReply", method: http.MethodPost, path: "/v2/saved-replies", call: func(c *Client) error {
			_, err := c.CreateSavedReply(ctx, body)
			return err
		}},
		{name: "UpdateSavedReply", method: http.MethodPut, path: "/v2/saved-replies/16", call: func(c *Client) error {
			return c.UpdateSavedReply(ctx, "16", body)
		}},
		{name: "DeleteSavedReply", method: http.MethodDelete, path: "/v2/saved-replies/16", call: func(c *Client) error {
			return c.DeleteSavedReply(ctx, "16")
		}},
		{name: "ListOrganizations", method: http.MethodGet, path: "/v2/organizations", query: "page=2", call: func(c *Client) error {
			_, err := c.ListOrganizations(ctx, params)
			return err
		}},
		{name: "GetOrganization", method: http.MethodGet, path: "/v2/organizations/17", call: func(c *Client) error {
			_, err := c.GetOrganization(ctx, "17")
			return err
		}},
		{name: "CreateOrganization", method: http.MethodPost, path: "/v2/organizations", call: func(c *Client) error {
			_, err := c.CreateOrganization(ctx, body)
			return err
		}},
		{name: "UpdateOrganization", method: http.MethodPut, path: "/v2/organizations/17", call: func(c *Client) error {
			return c.UpdateOrganization(ctx, "17", body)
		}},
		{name: "DeleteOrganization", method: http.MethodDelete, path: "/v2/organizations/17", call: func(c *Client) error {
			return c.DeleteOrganization(ctx, "17")
		}},
		{name: "ListOrganizationConversations", method: http.MethodGet, path: "/v2/organizations/17/conversations", query: "page=2", call: func(c *Client) error {
			_, err := c.ListOrganizationConversations(ctx, "17", params)
			return err
		}},
		{name: "ListOrganizationCustomers", method: http.MethodGet, path: "/v2/organizations/17/customers", query: "page=2", call: func(c *Client) error {
			_, err := c.ListOrganizationCustomers(ctx, "17", params)
			return err
		}},
		{name: "ListOrganizationProperties", method: http.MethodGet, path: "/v2/organizations/properties", query: "page=2", call: func(c *Client) error {
			_, err := c.ListOrganizationProperties(ctx, params)
			return err
		}},
		{name: "GetOrganizationProperty", method: http.MethodGet, path: "/v2/organizations/properties/18", call: func(c *Client) error {
			_, err := c.GetOrganizationProperty(ctx, "18")
			return err
		}},
		{name: "CreateOrganizationProperty", method: http.MethodPost, path: "/v2/organizations/properties", call: func(c *Client) error {
			_, err := c.CreateOrganizationProperty(ctx, body)
			return err
		}},
		{name: "UpdateOrganizationProperty", method: http.MethodPut, path: "/v2/organizations/properties/18", call: func(c *Client) error {
			return c.UpdateOrganizationProperty(ctx, "18", body)
		}},
		{name: "DeleteOrganizationProperty", method: http.MethodDelete, path: "/v2/organizations/properties/18", call: func(c *Client) error {
			return c.DeleteOrganizationProperty(ctx, "18")
		}},
		{name: "ListCustomerProperties", method: http.MethodGet, path: "/v2/customer-properties", query: "page=2", call: func(c *Client) error {
			_, err := c.ListCustomerProperties(ctx, params)
			return err
		}},
		{name: "GetCustomerProperty", method: http.MethodGet, path: "/v2/customer-properties/19", call: func(c *Client) error {
			_, err := c.GetCustomerProperty(ctx, "19")
			return err
		}},
		{name: "ListConversationProperties", method: http.MethodGet, path: "/v2/conversation-properties", query: "page=2", call: func(c *Client) error {
			_, err := c.ListConversationProperties(ctx, params)
			return err
		}},
		{name: "GetConversationProperty", method: http.MethodGet, path: "/v2/conversation-properties/20", call: func(c *Client) error {
			_, err := c.GetConversationProperty(ctx, "20")
			return err
		}},
		{name: "GetRating", method: http.MethodGet, path: "/v2/ratings/21", call: func(c *Client) error {
			_, err := c.GetRating(ctx, "21")
			return err
		}},
		{name: "GetReport", method: http.MethodGet, path: "/v2/reports/conversations", query: "page=2", call: func(c *Client) error {
			_, err := c.GetReport(ctx, "/conversations", params)
			return err
		}},
	}

	names := make([]string, 0, len(tests))
	for _, tt := range tests {
		names = append(names, tt.name)
	}
	assertEndpointCasesCoverInterface(t, (*ClientAPI)(nil), names)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotQuery, gotContentType, gotAccept string
			c := clientWithServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				gotQuery = r.URL.RawQuery
				gotContentType = r.Header.Get("Content-Type")
				gotAccept = r.Header.Get("Accept")

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Location", "https://api.helpscout.net/v2/resources/42")
				if r.Method == http.MethodGet {
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id": 42}))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})

			require.NoError(t, tt.call(c))
			assert.Equal(t, tt.method, gotMethod)
			assert.Equal(t, tt.path, gotPath)
			assert.Equal(t, tt.query, gotQuery)
			if tt.method == http.MethodPost || tt.method == http.MethodPut || tt.method == http.MethodPatch {
				assert.Equal(t, "application/json", gotContentType)
			} else {
				assert.Empty(t, gotContentType)
			}
			if tt.name == "GetThreadSourceRFC822" {
				assert.Equal(t, "message/rfc822", gotAccept)
			}
		})
	}
}

func assertEndpointCasesCoverInterface(t *testing.T, iface any, got []string) {
	t.Helper()

	typ := reflect.TypeOf(iface).Elem()
	want := make([]string, 0, typ.NumMethod())
	for i := 0; i < typ.NumMethod(); i++ {
		want = append(want, typ.Method(i).Name)
	}

	assert.ElementsMatch(t, want, got)
}
