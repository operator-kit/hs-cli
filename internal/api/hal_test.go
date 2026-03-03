package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractEmbedded_Valid(t *testing.T) {
	raw := json.RawMessage(`{
		"_embedded": {
			"mailboxes": [{"id": 1, "name": "Support"}]
		},
		"page": {"number": 1, "size": 25, "totalElements": 1, "totalPages": 1}
	}`)

	items, page, err := ExtractEmbedded(raw, "mailboxes")
	require.NoError(t, err)
	assert.NotNil(t, page)
	assert.Equal(t, 1, page.Number)
	assert.Equal(t, 1, page.TotalPages)
	assert.Equal(t, 1, page.TotalElements)

	var arr []json.RawMessage
	require.NoError(t, json.Unmarshal(items, &arr))
	assert.Len(t, arr, 1)
}

func TestExtractEmbedded_MissingKey(t *testing.T) {
	raw := json.RawMessage(`{
		"_embedded": {"mailboxes": []},
		"page": {"number": 1, "size": 25, "totalElements": 0, "totalPages": 0}
	}`)

	_, _, err := ExtractEmbedded(raw, "conversations")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `"conversations" not found`)
}

func TestExtractEmbedded_MalformedJSON(t *testing.T) {
	_, _, err := ExtractEmbedded(json.RawMessage(`{broken`), "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing HAL response")
}

func TestExtractEmbedded_MalformedEmbedded(t *testing.T) {
	raw := json.RawMessage(`{"_embedded": "not-an-object", "page": {}}`)
	_, _, err := ExtractEmbedded(raw, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing _embedded")
}

func TestExtractEmbedded_PageParsing(t *testing.T) {
	raw := json.RawMessage(`{
		"_embedded": {"items": [{"id": 1}, {"id": 2}]},
		"page": {"number": 2, "size": 10, "totalElements": 55, "totalPages": 6}
	}`)

	_, page, err := ExtractEmbedded(raw, "items")
	require.NoError(t, err)
	assert.Equal(t, 2, page.Number)
	assert.Equal(t, 10, page.Size)
	assert.Equal(t, 55, page.TotalElements)
	assert.Equal(t, 6, page.TotalPages)
}
