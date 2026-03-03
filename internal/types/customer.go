package types

type Customer struct {
	ID         int     `json:"id"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	Email      string  `json:"email,omitempty"`
	Phone      string  `json:"phone,omitempty"`
	PhotoURL   string  `json:"photoUrl,omitempty"`
	PhotoType  string  `json:"photoType,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	ModifiedAt string  `json:"updatedAt"`
	Emails     []Email `json:"-"` // populated from _embedded
}

type CustomerCreate struct {
	FirstName string  `json:"firstName"`
	LastName  string  `json:"lastName,omitempty"`
	Emails    []Email `json:"emails,omitempty"`
	Phone     string  `json:"phone,omitempty"`
}

type CustomerUpdate struct {
	FirstName string `json:"firstName,omitempty"`
	LastName  string `json:"lastName,omitempty"`
	Phone     string `json:"phone,omitempty"`
}
