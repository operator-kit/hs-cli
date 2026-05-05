package types

import "encoding/json"

type Tag struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Color string `json:"color"`
}

func (t *Tag) UnmarshalJSON(data []byte) error {
	var payload struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Tag   string `json:"tag"`
		Slug  string `json:"slug"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	t.ID = payload.ID
	t.Name = payload.Name
	if t.Name == "" {
		t.Name = payload.Tag
	}
	t.Slug = payload.Slug
	t.Color = payload.Color
	return nil
}
