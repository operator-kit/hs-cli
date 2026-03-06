package output

import "github.com/fatih/color"

// Terminal colors used across table, detail, and custom output.
// fatih/color auto-disables when stdout is not a TTY or NO_COLOR is set.
var (
	Blue  = color.New(color.FgHiBlue).SprintFunc()
	Green = color.New(color.FgHiGreen).SprintFunc()
	Dim   = color.New(color.FgHiBlack).SprintFunc()
)
