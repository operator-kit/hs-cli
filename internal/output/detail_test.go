package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintDetail(t *testing.T) {
	var buf bytes.Buffer
	fields := []Field{
		{Label: "ID", Value: "42"},
		{Label: "Subject", Value: "Test subject"},
		{Label: "Empty", Value: ""},
		{Label: "Status", Value: "active"},
	}
	PrintDetail(&buf, fields)
	out := buf.String()

	assert.Contains(t, out, "ID:")
	assert.Contains(t, out, "42")
	assert.Contains(t, out, "Subject:")
	assert.Contains(t, out, "Test subject")
	assert.Contains(t, out, "Status:")
	assert.NotContains(t, out, "Empty:")
}

func TestPrintDetailEmpty(t *testing.T) {
	var buf bytes.Buffer
	PrintDetail(&buf, nil)
	assert.Equal(t, "No results.\n", buf.String())
}

func TestPrintDetailAlignment(t *testing.T) {
	var buf bytes.Buffer
	fields := []Field{
		{Label: "ID", Value: "1"},
		{Label: "Long Label", Value: "val"},
	}
	PrintDetail(&buf, fields)
	out := buf.String()

	// Both lines should have values starting at the same column
	assert.Contains(t, out, "ID:          1")
	assert.Contains(t, out, "Long Label:  val")
}
