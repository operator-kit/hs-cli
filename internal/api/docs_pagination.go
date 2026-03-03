package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/operator-kit/hs-cli/internal/types"
)

// DocsListFunc is a function that fetches a page of Docs API results.
type DocsListFunc func(ctx context.Context, params url.Values) (json.RawMessage, error)

// DocsPaginateAll fetches all pages using the given Docs list function.
// Docs API wraps results as {"<key>":{"page":1,"pages":5,"count":100,"items":[...]}}.
// If noPaginate is false, only the specified page is returned.
func DocsPaginateAll(ctx context.Context, fn DocsListFunc, params url.Values, wrapperKey string, noPaginate bool) ([]json.RawMessage, *types.DocsPageInfo, error) {
	if params == nil {
		params = url.Values{}
	}

	data, err := fn(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	items, pageInfo, err := ExtractDocsItems(data, wrapperKey)
	if err != nil {
		return nil, nil, err
	}

	var all []json.RawMessage
	if err := json.Unmarshal(items, &all); err != nil {
		return nil, nil, fmt.Errorf("parsing items: %w", err)
	}

	if !noPaginate || pageInfo == nil || pageInfo.Pages <= 1 {
		return all, pageInfo, nil
	}

	for p := pageInfo.Page + 1; p <= pageInfo.Pages; p++ {
		params.Set("page", strconv.Itoa(p))
		data, err := fn(ctx, params)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching page %d: %w", p, err)
		}
		pageItems, _, err := ExtractDocsItems(data, wrapperKey)
		if err != nil {
			return nil, nil, err
		}
		var pi []json.RawMessage
		if err := json.Unmarshal(pageItems, &pi); err != nil {
			return nil, nil, err
		}
		all = append(all, pi...)
	}
	return all, pageInfo, nil
}

// ExtractDocsItems extracts the items array and page info from a Docs API response.
// Format: {"<key>":{"page":1,"pages":5,"count":100,"items":[...]}}
func ExtractDocsItems(data json.RawMessage, key string) (json.RawMessage, *types.DocsPageInfo, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(data, &outer); err != nil {
		return nil, nil, fmt.Errorf("parsing docs response: %w", err)
	}

	wrapper, ok := outer[key]
	if !ok {
		// Single-object response (no wrapper) — return data as-is
		return data, nil, nil
	}

	var inner struct {
		Page  int               `json:"page"`
		Pages int               `json:"pages"`
		Count int               `json:"count"`
		Items json.RawMessage   `json:"items"`
	}
	if err := json.Unmarshal(wrapper, &inner); err != nil {
		return nil, nil, fmt.Errorf("parsing docs wrapper %q: %w", key, err)
	}

	pageInfo := &types.DocsPageInfo{
		Page:  inner.Page,
		Pages: inner.Pages,
		Count: inner.Count,
	}

	if inner.Items == nil {
		return json.RawMessage("[]"), pageInfo, nil
	}

	return inner.Items, pageInfo, nil
}
