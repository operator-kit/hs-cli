package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func halPage(key string, items string, pageNum, totalPages, totalElements int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"_embedded": {%q: %s},
		"page": {"number": %d, "size": 2, "totalElements": %d, "totalPages": %d}
	}`, key, items, pageNum, totalElements, totalPages))
}

func TestPaginateAll_SinglePage(t *testing.T) {
	fn := func(ctx context.Context, params url.Values) (json.RawMessage, error) {
		return halPage("items", `[{"id":1},{"id":2}]`, 1, 1, 2), nil
	}

	items, page, err := PaginateAll(context.Background(), fn, nil, "items", false)
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, 1, page.Number)
	assert.Equal(t, 1, page.TotalPages)
}

func TestPaginateAll_MultiPage_NoPaginate(t *testing.T) {
	calls := 0
	fn := func(ctx context.Context, params url.Values) (json.RawMessage, error) {
		calls++
		switch calls {
		case 1:
			return halPage("items", `[{"id":1},{"id":2}]`, 1, 3, 6), nil
		case 2:
			return halPage("items", `[{"id":3},{"id":4}]`, 2, 3, 6), nil
		case 3:
			return halPage("items", `[{"id":5},{"id":6}]`, 3, 3, 6), nil
		default:
			return nil, fmt.Errorf("unexpected call %d", calls)
		}
	}

	items, page, err := PaginateAll(context.Background(), fn, nil, "items", true)
	require.NoError(t, err)
	assert.Len(t, items, 6)
	assert.Equal(t, 3, calls)
	assert.Equal(t, 1, page.Number)
}

func TestPaginateAll_SinglePage_NoPaginateFlag(t *testing.T) {
	fn := func(ctx context.Context, params url.Values) (json.RawMessage, error) {
		return halPage("items", `[{"id":1}]`, 1, 1, 1), nil
	}

	items, _, err := PaginateAll(context.Background(), fn, nil, "items", false)
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestPaginateAll_ErrorPropagation(t *testing.T) {
	fn := func(ctx context.Context, params url.Values) (json.RawMessage, error) {
		return nil, fmt.Errorf("api down")
	}

	_, _, err := PaginateAll(context.Background(), fn, nil, "items", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api down")
}

func TestPaginateAll_ErrorOnSecondPage(t *testing.T) {
	calls := 0
	fn := func(ctx context.Context, params url.Values) (json.RawMessage, error) {
		calls++
		if calls == 1 {
			return halPage("items", `[{"id":1}]`, 1, 2, 2), nil
		}
		return nil, fmt.Errorf("page 2 failed")
	}

	_, _, err := PaginateAll(context.Background(), fn, nil, "items", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "page 2 failed")
}
