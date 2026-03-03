package types

type Rating struct {
	ID      int    `json:"id"`
	Rating  string `json:"rating,omitempty"`
	Comments string `json:"comments,omitempty"`
}
