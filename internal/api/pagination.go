package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/operator-kit/hs-cli/internal/types"
)

// ListFunc is a function that fetches a page of results.
type ListFunc func(ctx context.Context, params url.Values) (json.RawMessage, error)

// PaginateAll fetches all pages using the given list function and embedded key.
// If noPaginate is false, only the specified page is returned.
func PaginateAll(ctx context.Context, fn ListFunc, params url.Values, embeddedKey string, noPaginate bool) ([]json.RawMessage, *types.PageInfo, error) {
	if params == nil {
		params = url.Values{}
	}

	data, err := fn(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	items, page, err := ExtractEmbedded(data, embeddedKey)
	if err != nil {
		return nil, nil, err
	}

	var all []json.RawMessage
	if err := json.Unmarshal(items, &all); err != nil {
		return nil, nil, fmt.Errorf("parsing items: %w", err)
	}

	if !noPaginate || page == nil || page.TotalPages <= 1 {
		return all, page, nil
	}

	// Fetch remaining pages
	for p := page.Number + 1; p <= page.TotalPages; p++ {
		params.Set("page", strconv.Itoa(p))
		data, err := fn(ctx, params)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching page %d: %w", p, err)
		}
		items, _, err := ExtractEmbedded(data, embeddedKey)
		if err != nil {
			return nil, nil, err
		}
		var pageItems []json.RawMessage
		if err := json.Unmarshal(items, &pageItems); err != nil {
			return nil, nil, err
		}
		all = append(all, pageItems...)
	}
	return all, page, nil
}
