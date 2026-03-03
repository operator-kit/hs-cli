package output

import (
	"fmt"
	"io"
	"strings"
)

type TableFormatter struct{}

func (f *TableFormatter) Format(w io.Writer, columns []string, rows []map[string]string) error {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No results.")
		return nil
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}
	for _, row := range rows {
		for i, col := range columns {
			if l := len(row[col]); l > widths[i] {
				widths[i] = l
			}
		}
	}

	// Print header
	for i, col := range columns {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", widths[i], strings.ToUpper(col))
	}
	fmt.Fprintln(w)

	// Print separator
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, strings.Repeat("─", width))
	}
	fmt.Fprintln(w)

	// Print rows
	for _, row := range rows {
		for i, col := range columns {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			fmt.Fprintf(w, "%-*s", widths[i], row[col])
		}
		fmt.Fprintln(w)
	}
	return nil
}
