package types

type User struct {
	ID         int    `json:"id"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	Type       string `json:"type"`
	CreatedAt  string `json:"createdAt"`
	ModifiedAt string `json:"updatedAt"`
	PhotoURL   string `json:"photoUrl,omitempty"`
}
