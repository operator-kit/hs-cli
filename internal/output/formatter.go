package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Out is the writer used by Print/PrintRaw. Tests swap this to a bytes.Buffer.
var Out io.Writer = os.Stdout

// Formatter writes structured data to output.
type Formatter interface {
	Format(w io.Writer, columns []string, rows []map[string]string) error
}

// New returns a Formatter for the given format name.
func New(format string) Formatter {
	switch format {
	case "json":
		return &JSONFormatter{}
	case "csv":
		return &CSVFormatter{}
	default:
		return &TableFormatter{}
	}
}

// Print is a convenience: format rows to Out.
func Print(format string, columns []string, rows []map[string]string) error {
	return New(format).Format(Out, columns, rows)
}

// PrintRaw prints raw JSON to Out (used when --format json and we have raw API data).
func PrintRaw(data json.RawMessage) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		_, err = fmt.Fprintln(Out, string(data))
		return err
	}
	enc := json.NewEncoder(Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
