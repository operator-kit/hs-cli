package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"

	"github.com/operator-kit/hs-cli/internal/config"
	"github.com/operator-kit/hs-cli/internal/output"
)

// mockClient implements api.ClientAPI with function fields.
// Unset methods panic to surface missing test setup.
type mockClient struct {
	ListMailboxesFn                 func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetMailboxFn                    func(ctx context.Context, id string) (json.RawMessage, error)
	ListMailboxFoldersFn            func(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error)
	ListMailboxCustomFieldsFn       func(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error)
	GetMailboxRoutingFn             func(ctx context.Context, mailboxID string) (json.RawMessage, error)
	UpdateMailboxRoutingFn          func(ctx context.Context, mailboxID string, body any) error
	ListConversationsFn             func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetConversationFn               func(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	CreateConversationFn            func(ctx context.Context, body any) (string, error)
	UpdateConversationFn            func(ctx context.Context, id string, body any) error
	UpdateConversationFieldsFn      func(ctx context.Context, id string, body any) error
	UpdateConversationTagsFn        func(ctx context.Context, id string, body any) error
	UpdateConversationSnoozeFn      func(ctx context.Context, id string, body any) error
	DeleteConversationSnoozeFn      func(ctx context.Context, id string) error
	DeleteConversationFn            func(ctx context.Context, id string) error
	CreateAttachmentFn              func(ctx context.Context, convID string, threadID string, body any) error
	GetAttachmentDataFn             func(ctx context.Context, convID string, attachmentID string) (json.RawMessage, error)
	DeleteAttachmentFn              func(ctx context.Context, convID string, attachmentID string) error
	ListThreadsFn                   func(ctx context.Context, convID string, params url.Values) (json.RawMessage, error)
	CreateReplyFn                   func(ctx context.Context, convID string, body any) error
	CreateNoteFn                    func(ctx context.Context, convID string, body any) error
	CreateChatThreadFn              func(ctx context.Context, convID string, body any) error
	CreateCustomerThreadFn          func(ctx context.Context, convID string, body any) error
	CreatePhoneThreadFn             func(ctx context.Context, convID string, body any) error
	UpdateThreadFn                  func(ctx context.Context, convID string, threadID string, body any) error
	GetThreadSourceFn               func(ctx context.Context, convID string, threadID string) (json.RawMessage, error)
	GetThreadSourceRFC822Fn         func(ctx context.Context, convID string, threadID string) ([]byte, error)
	ListCustomersFn                 func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetCustomerFn                   func(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	CreateCustomerFn                func(ctx context.Context, body any) (string, error)
	UpdateCustomerFn                func(ctx context.Context, id string, body any) error
	OverwriteCustomerFn             func(ctx context.Context, id string, body any) error
	DeleteCustomerFn                func(ctx context.Context, id string, params url.Values) error
	ListTagsFn                      func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetTagFn                        func(ctx context.Context, id string) (json.RawMessage, error)
	ListUsersFn                     func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetUserFn                       func(ctx context.Context, id string) (json.RawMessage, error)
	GetResourceOwnerFn              func(ctx context.Context) (json.RawMessage, error)
	DeleteUserFn                    func(ctx context.Context, id string) error
	ListUserStatusesFn              func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetUserStatusFn                 func(ctx context.Context, id string) (json.RawMessage, error)
	SetUserStatusFn                 func(ctx context.Context, id string, body any) error
	ListTeamsFn                     func(ctx context.Context, params url.Values) (json.RawMessage, error)
	ListTeamMembersFn               func(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	ListWorkflowsFn                 func(ctx context.Context, params url.Values) (json.RawMessage, error)
	UpdateWorkflowStatusFn          func(ctx context.Context, id string, body any) error
	RunWorkflowFn                   func(ctx context.Context, id string, body any) error
	ListWebhooksFn                  func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetWebhookFn                    func(ctx context.Context, id string) (json.RawMessage, error)
	CreateWebhookFn                 func(ctx context.Context, body any) (string, error)
	UpdateWebhookFn                 func(ctx context.Context, id string, body any) error
	DeleteWebhookFn                 func(ctx context.Context, id string) error
	ListSavedRepliesFn              func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetSavedReplyFn                 func(ctx context.Context, id string) (json.RawMessage, error)
	CreateSavedReplyFn              func(ctx context.Context, body any) (string, error)
	UpdateSavedReplyFn              func(ctx context.Context, id string, body any) error
	DeleteSavedReplyFn              func(ctx context.Context, id string) error
	ListOrganizationsFn             func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetOrganizationFn               func(ctx context.Context, id string) (json.RawMessage, error)
	CreateOrganizationFn            func(ctx context.Context, body any) (string, error)
	UpdateOrganizationFn            func(ctx context.Context, id string, body any) error
	DeleteOrganizationFn            func(ctx context.Context, id string) error
	ListOrganizationConversationsFn func(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	ListOrganizationCustomersFn     func(ctx context.Context, id string, params url.Values) (json.RawMessage, error)
	ListOrganizationPropertiesFn    func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetOrganizationPropertyFn       func(ctx context.Context, id string) (json.RawMessage, error)
	CreateOrganizationPropertyFn    func(ctx context.Context, body any) (string, error)
	UpdateOrganizationPropertyFn    func(ctx context.Context, id string, body any) error
	DeleteOrganizationPropertyFn    func(ctx context.Context, id string) error
	ListCustomerPropertiesFn        func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetCustomerPropertyFn           func(ctx context.Context, id string) (json.RawMessage, error)
	ListConversationPropertiesFn    func(ctx context.Context, params url.Values) (json.RawMessage, error)
	GetConversationPropertyFn       func(ctx context.Context, id string) (json.RawMessage, error)
	GetRatingFn                     func(ctx context.Context, id string) (json.RawMessage, error)
	GetReportFn                     func(ctx context.Context, family string, params url.Values) (json.RawMessage, error)
}

func (m *mockClient) ListMailboxes(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListMailboxesFn(ctx, params)
}
func (m *mockClient) GetMailbox(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetMailboxFn(ctx, id)
}
func (m *mockClient) ListMailboxFolders(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error) {
	return m.ListMailboxFoldersFn(ctx, mailboxID, params)
}
func (m *mockClient) ListMailboxCustomFields(ctx context.Context, mailboxID string, params url.Values) (json.RawMessage, error) {
	return m.ListMailboxCustomFieldsFn(ctx, mailboxID, params)
}
func (m *mockClient) GetMailboxRouting(ctx context.Context, mailboxID string) (json.RawMessage, error) {
	return m.GetMailboxRoutingFn(ctx, mailboxID)
}
func (m *mockClient) UpdateMailboxRouting(ctx context.Context, mailboxID string, body any) error {
	return m.UpdateMailboxRoutingFn(ctx, mailboxID, body)
}
func (m *mockClient) ListConversations(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListConversationsFn(ctx, params)
}
func (m *mockClient) GetConversation(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return m.GetConversationFn(ctx, id, params)
}
func (m *mockClient) CreateConversation(ctx context.Context, body any) (string, error) {
	return m.CreateConversationFn(ctx, body)
}
func (m *mockClient) UpdateConversation(ctx context.Context, id string, body any) error {
	return m.UpdateConversationFn(ctx, id, body)
}
func (m *mockClient) UpdateConversationFields(ctx context.Context, id string, body any) error {
	return m.UpdateConversationFieldsFn(ctx, id, body)
}
func (m *mockClient) UpdateConversationTags(ctx context.Context, id string, body any) error {
	return m.UpdateConversationTagsFn(ctx, id, body)
}
func (m *mockClient) UpdateConversationSnooze(ctx context.Context, id string, body any) error {
	return m.UpdateConversationSnoozeFn(ctx, id, body)
}
func (m *mockClient) DeleteConversationSnooze(ctx context.Context, id string) error {
	return m.DeleteConversationSnoozeFn(ctx, id)
}
func (m *mockClient) DeleteConversation(ctx context.Context, id string) error {
	return m.DeleteConversationFn(ctx, id)
}
func (m *mockClient) CreateAttachment(ctx context.Context, convID string, threadID string, body any) error {
	return m.CreateAttachmentFn(ctx, convID, threadID, body)
}
func (m *mockClient) GetAttachmentData(ctx context.Context, convID string, attachmentID string) (json.RawMessage, error) {
	return m.GetAttachmentDataFn(ctx, convID, attachmentID)
}
func (m *mockClient) DeleteAttachment(ctx context.Context, convID string, attachmentID string) error {
	return m.DeleteAttachmentFn(ctx, convID, attachmentID)
}
func (m *mockClient) ListThreads(ctx context.Context, convID string, params url.Values) (json.RawMessage, error) {
	return m.ListThreadsFn(ctx, convID, params)
}
func (m *mockClient) CreateReply(ctx context.Context, convID string, body any) error {
	return m.CreateReplyFn(ctx, convID, body)
}
func (m *mockClient) CreateNote(ctx context.Context, convID string, body any) error {
	return m.CreateNoteFn(ctx, convID, body)
}
func (m *mockClient) CreateChatThread(ctx context.Context, convID string, body any) error {
	return m.CreateChatThreadFn(ctx, convID, body)
}
func (m *mockClient) CreateCustomerThread(ctx context.Context, convID string, body any) error {
	return m.CreateCustomerThreadFn(ctx, convID, body)
}
func (m *mockClient) CreatePhoneThread(ctx context.Context, convID string, body any) error {
	return m.CreatePhoneThreadFn(ctx, convID, body)
}
func (m *mockClient) UpdateThread(ctx context.Context, convID string, threadID string, body any) error {
	return m.UpdateThreadFn(ctx, convID, threadID, body)
}
func (m *mockClient) GetThreadSource(ctx context.Context, convID string, threadID string) (json.RawMessage, error) {
	return m.GetThreadSourceFn(ctx, convID, threadID)
}
func (m *mockClient) GetThreadSourceRFC822(ctx context.Context, convID string, threadID string) ([]byte, error) {
	return m.GetThreadSourceRFC822Fn(ctx, convID, threadID)
}
func (m *mockClient) ListCustomers(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListCustomersFn(ctx, params)
}
func (m *mockClient) GetCustomer(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return m.GetCustomerFn(ctx, id, params)
}
func (m *mockClient) CreateCustomer(ctx context.Context, body any) (string, error) {
	return m.CreateCustomerFn(ctx, body)
}
func (m *mockClient) UpdateCustomer(ctx context.Context, id string, body any) error {
	return m.UpdateCustomerFn(ctx, id, body)
}
func (m *mockClient) OverwriteCustomer(ctx context.Context, id string, body any) error {
	return m.OverwriteCustomerFn(ctx, id, body)
}
func (m *mockClient) DeleteCustomer(ctx context.Context, id string, params url.Values) error {
	return m.DeleteCustomerFn(ctx, id, params)
}
func (m *mockClient) ListTags(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListTagsFn(ctx, params)
}
func (m *mockClient) GetTag(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetTagFn(ctx, id)
}
func (m *mockClient) ListUsers(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListUsersFn(ctx, params)
}
func (m *mockClient) GetUser(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetUserFn(ctx, id)
}
func (m *mockClient) GetResourceOwner(ctx context.Context) (json.RawMessage, error) {
	return m.GetResourceOwnerFn(ctx)
}
func (m *mockClient) DeleteUser(ctx context.Context, id string) error {
	return m.DeleteUserFn(ctx, id)
}
func (m *mockClient) ListUserStatuses(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListUserStatusesFn(ctx, params)
}
func (m *mockClient) GetUserStatus(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetUserStatusFn(ctx, id)
}
func (m *mockClient) SetUserStatus(ctx context.Context, id string, body any) error {
	return m.SetUserStatusFn(ctx, id, body)
}
func (m *mockClient) ListTeams(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListTeamsFn(ctx, params)
}
func (m *mockClient) ListTeamMembers(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return m.ListTeamMembersFn(ctx, id, params)
}
func (m *mockClient) ListWorkflows(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListWorkflowsFn(ctx, params)
}
func (m *mockClient) UpdateWorkflowStatus(ctx context.Context, id string, body any) error {
	return m.UpdateWorkflowStatusFn(ctx, id, body)
}
func (m *mockClient) RunWorkflow(ctx context.Context, id string, body any) error {
	return m.RunWorkflowFn(ctx, id, body)
}
func (m *mockClient) ListWebhooks(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListWebhooksFn(ctx, params)
}
func (m *mockClient) GetWebhook(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetWebhookFn(ctx, id)
}
func (m *mockClient) CreateWebhook(ctx context.Context, body any) (string, error) {
	return m.CreateWebhookFn(ctx, body)
}
func (m *mockClient) UpdateWebhook(ctx context.Context, id string, body any) error {
	return m.UpdateWebhookFn(ctx, id, body)
}
func (m *mockClient) DeleteWebhook(ctx context.Context, id string) error {
	return m.DeleteWebhookFn(ctx, id)
}
func (m *mockClient) ListSavedReplies(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListSavedRepliesFn(ctx, params)
}
func (m *mockClient) GetSavedReply(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetSavedReplyFn(ctx, id)
}
func (m *mockClient) CreateSavedReply(ctx context.Context, body any) (string, error) {
	return m.CreateSavedReplyFn(ctx, body)
}
func (m *mockClient) UpdateSavedReply(ctx context.Context, id string, body any) error {
	return m.UpdateSavedReplyFn(ctx, id, body)
}
func (m *mockClient) DeleteSavedReply(ctx context.Context, id string) error {
	return m.DeleteSavedReplyFn(ctx, id)
}
func (m *mockClient) ListOrganizations(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListOrganizationsFn(ctx, params)
}
func (m *mockClient) GetOrganization(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetOrganizationFn(ctx, id)
}
func (m *mockClient) CreateOrganization(ctx context.Context, body any) (string, error) {
	return m.CreateOrganizationFn(ctx, body)
}
func (m *mockClient) UpdateOrganization(ctx context.Context, id string, body any) error {
	return m.UpdateOrganizationFn(ctx, id, body)
}
func (m *mockClient) DeleteOrganization(ctx context.Context, id string) error {
	return m.DeleteOrganizationFn(ctx, id)
}
func (m *mockClient) ListOrganizationConversations(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return m.ListOrganizationConversationsFn(ctx, id, params)
}
func (m *mockClient) ListOrganizationCustomers(ctx context.Context, id string, params url.Values) (json.RawMessage, error) {
	return m.ListOrganizationCustomersFn(ctx, id, params)
}
func (m *mockClient) ListOrganizationProperties(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListOrganizationPropertiesFn(ctx, params)
}
func (m *mockClient) GetOrganizationProperty(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetOrganizationPropertyFn(ctx, id)
}
func (m *mockClient) CreateOrganizationProperty(ctx context.Context, body any) (string, error) {
	return m.CreateOrganizationPropertyFn(ctx, body)
}
func (m *mockClient) UpdateOrganizationProperty(ctx context.Context, id string, body any) error {
	return m.UpdateOrganizationPropertyFn(ctx, id, body)
}
func (m *mockClient) DeleteOrganizationProperty(ctx context.Context, id string) error {
	return m.DeleteOrganizationPropertyFn(ctx, id)
}
func (m *mockClient) ListCustomerProperties(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListCustomerPropertiesFn(ctx, params)
}
func (m *mockClient) GetCustomerProperty(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetCustomerPropertyFn(ctx, id)
}
func (m *mockClient) ListConversationProperties(ctx context.Context, params url.Values) (json.RawMessage, error) {
	return m.ListConversationPropertiesFn(ctx, params)
}
func (m *mockClient) GetConversationProperty(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetConversationPropertyFn(ctx, id)
}
func (m *mockClient) GetRating(ctx context.Context, id string) (json.RawMessage, error) {
	return m.GetRatingFn(ctx, id)
}
func (m *mockClient) GetReport(ctx context.Context, family string, params url.Values) (json.RawMessage, error) {
	return m.GetReportFn(ctx, family, params)
}

// setupTest sets globals for testing and returns a buffer capturing output.
func setupTest(mock *mockClient) *bytes.Buffer {
	buf := &bytes.Buffer{}
	output.Out = buf
	apiClient = mock
	cfg = &config.Config{Format: "table", InboxPIIMode: "off", InboxPIIAllowUnredacted: false}
	format = "table"
	unredacted = false
	noPaginate = false
	page = 1
	perPage = 25
	debug = false
	return buf
}

// halJSON wraps items in a HAL response envelope.
func halJSON(key string, items string) json.RawMessage {
	return json.RawMessage(`{
		"_embedded": {"` + key + `": ` + items + `},
		"page": {"number": 1, "size": 25, "totalElements": 1, "totalPages": 1}
	}`)
}
