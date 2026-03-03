package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONFormatter_ValidOutput(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	cols := []string{"id", "name"}
	rows := []map[string]string{
		{"id": "1", "name": "Test"},
	}

	require.NoError(t, f.Format(&buf, cols, rows))

	var result []map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Len(t, result, 1)
	assert.Equal(t, "1", result[0]["id"])
	assert.Equal(t, "Test", result[0]["name"])
}

func TestJSONFormatter_PrettyPrinted(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}
	cols := []string{"id"}
	rows := []map[string]string{{"id": "1"}}

	require.NoError(t, f.Format(&buf, cols, rows))
	// Pretty-printed JSON should contain indentation
	assert.Contains(t, buf.String(), "  ")
	assert.Contains(t, buf.String(), "\n")
}
