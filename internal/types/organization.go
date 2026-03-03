package types

type Organization struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain,omitempty"`
}

type OrganizationProperty struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}
