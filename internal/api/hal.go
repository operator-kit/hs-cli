package api

import (
	"encoding/json"
	"fmt"

	"github.com/operator-kit/hs-cli/internal/types"
)

// HALResponse represents a HAL+JSON list response from HelpScout.
type HALResponse struct {
	Embedded json.RawMessage `json:"_embedded"`
	Page     types.PageInfo  `json:"page"`
}

// ExtractEmbedded extracts the named array from _embedded.
func ExtractEmbedded(data json.RawMessage, key string) (json.RawMessage, *types.PageInfo, error) {
	var hal HALResponse
	if err := json.Unmarshal(data, &hal); err != nil {
		return nil, nil, fmt.Errorf("parsing HAL response: %w", err)
	}
	var embedded map[string]json.RawMessage
	if err := json.Unmarshal(hal.Embedded, &embedded); err != nil {
		return nil, nil, fmt.Errorf("parsing _embedded: %w", err)
	}
	items, ok := embedded[key]
	if !ok {
		return nil, nil, fmt.Errorf("key %q not found in _embedded", key)
	}
	return items, &hal.Page, nil
}
