package types

type Mailbox struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Email      string `json:"email"`
	CreatedAt  string `json:"createdAt"`
	ModifiedAt string `json:"updatedAt"`
}
