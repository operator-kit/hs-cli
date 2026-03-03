package types

type Person struct {
	ID    int    `json:"id,omitempty"`
	Type  string `json:"type,omitempty"`
	Email string `json:"email,omitempty"`
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

type Email struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Address struct {
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Zip     string `json:"postalCode,omitempty"`
	Country string `json:"country,omitempty"`
	Lines   []string `json:"lines,omitempty"`
}
