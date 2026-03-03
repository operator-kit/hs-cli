package types

type Thread struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	State     string `json:"state"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	CreatedBy Person `json:"createdBy"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Source struct {
		Type string `json:"type"`
		Via  string `json:"via"`
	} `json:"source"`
	Action struct {
		Text string `json:"text"`
		Type string `json:"type"`
	} `json:"action"`
}

type ReplyBody struct {
	Customer    Person   `json:"customer"`
	Text        string   `json:"text"`
	Status      string   `json:"status,omitempty"`
	Draft       *bool    `json:"draft,omitempty"`
	Imported    *bool    `json:"imported,omitempty"`
	User        int      `json:"user,omitempty"`
	To          []Person `json:"to,omitempty"`
	CC          []Person `json:"cc,omitempty"`
	BCC         []Person `json:"bcc,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	Type        string   `json:"type,omitempty"`
	Attachments []int    `json:"attachments,omitempty"`
}

type NoteBody struct {
	Text        string `json:"text"`
	User        int    `json:"user,omitempty"`
	Status      string `json:"status,omitempty"`
	Attachments []int  `json:"attachments,omitempty"`
}

type ThreadCreateBody struct {
	Customer    *Person `json:"customer,omitempty"`
	Text        string  `json:"text"`
	Imported    *bool   `json:"imported,omitempty"`
	CreatedAt   string  `json:"createdAt,omitempty"`
	Attachments []int   `json:"attachments,omitempty"`
}
