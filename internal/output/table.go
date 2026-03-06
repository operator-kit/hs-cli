package output

import (
	"fmt"
	"io"
	"strings"
	"unicode"
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

	// Print header (pad before coloring to preserve alignment)
	for i, col := range columns {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		padded := fmt.Sprintf("%-*s", widths[i], titleCase(col))
		fmt.Fprint(w, Dim(padded))
	}
	fmt.Fprintln(w)

	// Print separator
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, Dim(strings.Repeat("─", width)))
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
	fmt.Fprintln(w)
	return nil
}

// abbreviations that should stay fully uppercase in title case.
var upperWords = map[string]bool{
	"id": true, "ip": true, "url": true, "api": true, "ssl": true,
}

// titleCase converts "some_column" to "Some Column", keeping abbreviations uppercase.
func titleCase(s string) string {
	words := strings.Split(strings.ReplaceAll(s, "_", " "), " ")
	for i, w := range words {
		if upperWords[strings.ToLower(w)] {
			words[i] = strings.ToUpper(w)
		} else if len(w) > 0 {
			words[i] = string(unicode.ToUpper(rune(w[0]))) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
