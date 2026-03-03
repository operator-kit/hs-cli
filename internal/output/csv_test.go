package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVFormatter_HeaderRow(t *testing.T) {
	var buf bytes.Buffer
	f := &CSVFormatter{}
	cols := []string{"id", "name", "email"}
	rows := []map[string]string{
		{"id": "1", "name": "Alice", "email": "alice@test.com"},
	}

	require.NoError(t, f.Format(&buf, cols, rows))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	assert.Equal(t, "id,name,email", lines[0])
}

func TestCSVFormatter_DataRows(t *testing.T) {
	var buf bytes.Buffer
	f := &CSVFormatter{}
	cols := []string{"id", "name"}
	rows := []map[string]string{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}

	require.NoError(t, f.Format(&buf, cols, rows))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	assert.Len(t, lines, 3) // header + 2 rows
	assert.Equal(t, "1,Alice", lines[1])
	assert.Equal(t, "2,Bob", lines[2])
}

func TestCSVFormatter_Escaping(t *testing.T) {
	var buf bytes.Buffer
	f := &CSVFormatter{}
	cols := []string{"name"}
	rows := []map[string]string{
		{"name": `has "quotes" and, commas`},
	}

	require.NoError(t, f.Format(&buf, cols, rows))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// CSV should quote the field with embedded quotes/commas
	assert.Equal(t, `"has ""quotes"" and, commas"`, lines[1])
}
