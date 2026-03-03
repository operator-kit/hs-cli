package output

import (
	"fmt"
	"io"
)

// Field is a label-value pair for detail output.
type Field struct {
	Label string
	Value string
}

// PrintDetail writes a vertical key-value layout to w.
// Labels are right-padded to align values. Empty values are skipped.
func PrintDetail(w io.Writer, fields []Field) {
	if len(fields) == 0 {
		fmt.Fprintln(w, "No results.")
		return
	}

	maxLabel := 0
	for _, f := range fields {
		if f.Value != "" && len(f.Label) > maxLabel {
			maxLabel = len(f.Label)
		}
	}

	for _, f := range fields {
		if f.Value == "" {
			continue
		}
		fmt.Fprintf(w, "%-*s  %s\n", maxLabel+1, f.Label+":", f.Value)
	}
}
