package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// ClientAPI defines the public interface for the HelpScout API client.
type ClientAPI interface {
	ListMailboxes(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetMailbox(ctx context.Context, id string) (json.RawMessage, error)
	ListMailboxFolders(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error)
	ListMailboxCustomFields(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error)
	GetMailboxRouting(ctx context.Context, mailboxID string) (json.RawMessage, error)
	UpdateMailboxRouting(ctx context.Context, mailboxID string, body any) error

	ListConversations(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetConversation(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	CreateConversation(ctx context.Context, body any) (string, error)
	UpdateConversation(ctx context.Context, id string, body any) error
	UpdateConversationFields(ctx context.Context, id string, body any) error
	UpdateConversationTags(ctx context.Context, id string, body any) error
	UpdateConversationSnooze(ctx context.Context, id string, body any) error
	DeleteConversationSnooze(ctx context.Context, id string) error
	DeleteConversation(ctx context.Context, id string) error
	CreateAttachment(ctx context.Context, convID string, threadID string, body any) error
	GetAttachmentData(ctx context.Context, convID string, attachmentID string) (json.RawMessage, error)
	DeleteAttachment(ctx context.Context, convID string, attachmentID string) error

	ListThreads(ctx context.Context, convID string, params url.Values) (json.RawMessage, error)
	CreateReply(ctx context.Context, convID string, body any) error
	CreateNote(ctx context.Context, convID string, body any) error
	CreateChatThread(ctx context.Context, convID string, body any) error
	CreateCustomerThread(ctx context.Context, convID string, body any) error
	CreatePhoneThread(ctx context.Context, convID string, body any) error
	UpdateThread(ctx context.Context, convID string, threadID string, body any) error
	GetThreadSource(ctx context.Context, convID string, threadID string) (json.RawMessage, error)
	GetThreadSourceRFC822(ctx context.Context, convID string, threadID string) ([]byte, error)

	ListCustomers(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetCustomer(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	CreateCustomer(ctx context.Context, body any) (string, error)
	UpdateCustomer(ctx context.Context, id string, body any) error
	OverwriteCustomer(ctx context.Context, id string, body any) error
	DeleteCustomer(ctx context.Context, id string, params url.Values) error

	ListTags(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetTag(ctx context.Context, id string) (json.RawMessage, error)

	ListUsers(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetUser(ctx context.Context, id string) (json.RawMessage, error)
	GetResourceOwner(ctx context.Context) (json.RawMessage, error)
	DeleteUser(ctx context.Context, id string) error
	ListUserStatuses(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetUserStatus(ctx context.Context, id string) (json.RawMessage, error)
	SetUserStatus(ctx context.Context, id string, body any) error

	ListTeams(ctx context.Context, params url.Values) (json.RawMessage, error)
	ListTeamMembers(ctx context.Context, id string, params url.Values) (json.RawMessage, error)

	ListWorkflows(ctx context.Context, params url.Values) (json.RawMessage, error)
	UpdateWorkflowStatus(ctx context.Context, id string, body any) error
	RunWorkflow(ctx context.Context, id string, body any) error

	ListWebhooks(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetWebhook(ctx context.Context, id string) (json.RawMessage, error)
	CreateWebhook(ctx context.Context, body any) (string, error)
	UpdateWebhook(ctx context.Context, id string, body any) error
	DeleteWebhook(ctx context.Context, id string) error

	ListSavedReplies(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetSavedReply(ctx context.Context, id string) (json.RawMessage, error)
	CreateSavedReply(ctx context.Context, body any) (string, error)
	UpdateSavedReply(ctx context.Context, id string, body any) error
	DeleteSavedReply(ctx context.Context, id string) error

	ListOrganizations(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetOrganization(ctx context.Context, id string) (json.RawMessage, error)
	CreateOrganization(ctx context.Context, body any) (string, error)
	UpdateOrganization(ctx context.Context, id string, body any) error
	DeleteOrganization(ctx context.Context, id string) error
	ListOrganizationConversations(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	ListOrganizationCustomers(ctx context.Context, id string, params url.Values) (json.RawMessage, error)

	ListOrganizationProperties(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetOrganizationProperty(ctx context.Context, id string) (json.RawMessage, error)
	CreateOrganizationProperty(ctx context.Context, body any) (string, error)
	UpdateOrganizationProperty(ctx context.Context, id string, body any) error
	DeleteOrganizationProperty(ctx context.Context, id string) error

	ListCustomerProperties(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetCustomerProperty(ctx context.Context, id string) (json.RawMessage, error)
	ListConversationProperties(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetConversationProperty(ctx context.Context, id string) (json.RawMessage, error)

	GetRating(ctx context.Context, id string) (json.RawMessage, error)

	GetReport(ctx context.Context, family string, params url.Values) (json.RawMessage, error)
}
