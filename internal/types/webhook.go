package types

type Webhook struct {
	ID             int      `json:"id"`
	URL            string   `json:"url"`
	State          string   `json:"state"`
	Events         []string `json:"events"`
	Secret         string   `json:"secret,omitempty"`
	CreatedAt      string   `json:"createdAt"`
	PayloadVersion string   `json:"payloadVersion,omitempty"`
	MailboxIDs     []int    `json:"mailboxIds,omitempty"`
	Notification   bool     `json:"notification,omitempty"`
	Label          string   `json:"label,omitempty"`
}

type WebhookCreate struct {
	URL            string `json:"url"`
	Events         []string `json:"events"`
	Secret         string `json:"secret"`
	PayloadVersion string `json:"payloadVersion,omitempty"`
	MailboxIDs     []int  `json:"mailboxIds,omitempty"`
	Notification   *bool  `json:"notification,omitempty"`
	Label          string `json:"label,omitempty"`
}

type WebhookUpdate struct {
	URL            string   `json:"url,omitempty"`
	Events         []string `json:"events,omitempty"`
	Secret         string   `json:"secret,omitempty"`
	PayloadVersion string   `json:"payloadVersion,omitempty"`
	MailboxIDs     []int    `json:"mailboxIds,omitempty"`
	Notification   *bool    `json:"notification,omitempty"`
	Label          string   `json:"label,omitempty"`
}
