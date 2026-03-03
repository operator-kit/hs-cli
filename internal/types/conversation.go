package types

type Conversation struct {
	ID              int    `json:"id"`
	Number          int    `json:"number"`
	Subject         string `json:"subject"`
	Status          string `json:"status"`
	State           string `json:"state"`
	Type            string `json:"type"`
	MailboxID       int    `json:"mailboxId"`
	AssignTo        int    `json:"assignTo,omitempty"`
	CreatedAt       string `json:"createdAt"`
	ModifiedAt      string `json:"userUpdatedAt"`
	ClosedAt        string `json:"closedAt,omitempty"`
	PrimaryCustomer Person `json:"primaryCustomer"`
	Assignee        *Person `json:"assignee,omitempty"`
	Preview         string `json:"preview"`
	Tags            []Tag  `json:"tags,omitempty"`
	CC              []string `json:"cc,omitempty"`
	BCC             []string `json:"bcc,omitempty"`
	CustomFields    []ConversationField `json:"customFields,omitempty"`
	Source          struct {
		Type string `json:"type"`
		Via  string `json:"via"`
	} `json:"source,omitempty"`
	Threads []Thread `json:"-"`
}

type ConversationCreate struct {
	Subject    string              `json:"subject"`
	MailboxID  int                 `json:"mailboxId"`
	Type       string              `json:"type"`
	Status     string              `json:"status"`
	Customer   Person              `json:"customer"`
	Threads    []ThreadCreate      `json:"threads"`
	Tags       []string            `json:"tags,omitempty"`
	AssignTo   int                 `json:"assignTo,omitempty"`
	CreatedAt  string              `json:"createdAt,omitempty"`
	Imported   *bool               `json:"imported,omitempty"`
	AutoReply  *bool               `json:"autoReply,omitempty"`
	Fields     []ConversationField `json:"fields,omitempty"`
}

type ThreadCreate struct {
	Type     string `json:"type"`
	Customer Person `json:"customer,omitempty"`
	Text     string `json:"text"`
}

type ConversationField struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}
