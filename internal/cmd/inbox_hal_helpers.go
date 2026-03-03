package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/operator-kit/hs-cli/internal/api"
)

func extractEmbeddedWithCandidates(data json.RawMessage, keys ...string) ([]json.RawMessage, error) {
	for _, key := range keys {
		itemsRaw, _, err := api.ExtractEmbedded(data, key)
		if err == nil {
			var items []json.RawMessage
			if err := json.Unmarshal(itemsRaw, &items); err != nil {
				return nil, fmt.Errorf("parsing embedded %q array: %w", key, err)
			}
			return items, nil
		}
	}
	return nil, fmt.Errorf("embedded key not found in response (tried: %v)", keys)
}

func parseJSONBody(raw string) (map[string]any, error) {
	body := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return nil, err
	}
	return body, nil
}
