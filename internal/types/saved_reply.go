package types

type SavedReply struct {
	ID        int    `json:"id"`
	MailboxID int    `json:"mailboxId,omitempty"`
	Name      string `json:"name"`
	Subject   string `json:"subject,omitempty"`
	Text      string `json:"text,omitempty"`
	IsPrivate bool   `json:"isPrivate,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}
