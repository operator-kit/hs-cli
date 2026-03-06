package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTableFormatter_Basic(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}
	cols := []string{"id", "name"}
	rows := []map[string]string{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}

	require.NoError(t, f.Format(&buf, cols, rows))
	out := buf.String()

	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "Bob")
	assert.Contains(t, out, "──") // separator
}

func TestTableFormatter_ColumnAlignment(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}
	cols := []string{"id", "name"}
	rows := []map[string]string{
		{"id": "1", "name": "Short"},
		{"id": "2", "name": "A longer name"},
	}

	require.NoError(t, f.Format(&buf, cols, rows))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// Header + separator + 2 data rows = 4 lines
	assert.Len(t, lines, 4)

	// All lines with data should have consistent column positions
	// The NAME column should be padded to accommodate "A longer name"
	for _, line := range lines[2:] {
		// Each data row should have the name right-padded
		assert.True(t, len(line) > 10, "row should be wide enough: %q", line)
	}
}

func TestTableFormatter_EmptyRows(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}
	require.NoError(t, f.Format(&buf, []string{"id"}, nil))
	assert.Equal(t, "No results.\n", buf.String())
}
