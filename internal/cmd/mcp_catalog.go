package cmd

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type mcpToolSpec struct {
	Name           string
	Description    string
	CommandPath    []string
	PositionalArgs []mcpPositionalArgSpec
	Flags          []mcpFlagSpec
	InputSchema    map[string]any
}

type mcpPositionalArgSpec struct {
	Name     string
	Property string
	Required bool
}

type mcpFlagSpec struct {
	Name     string
	Property string
	Usage    string
	Type     string
	Required bool
}

func discoverMCPTools() ([]mcpToolSpec, error) {
	tools := make([]mcpToolSpec, 0, 128)

	inboxCmd := findSubtreeCommand("inbox")
	if inboxCmd != nil {
		for _, child := range visibleChildren(inboxCmd) {
			walkMCPCommandTree(child, []string{"inbox"}, &tools)
		}
	}

	docsCmd := findSubtreeCommand("docs")
	if docsCmd != nil {
		for _, child := range visibleChildren(docsCmd) {
			walkMCPCommandTree(child, []string{"docs"}, &tools)
		}
	}

	if len(tools) == 0 {
		return nil, fmt.Errorf("no command trees available")
	}

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools, nil
}

func findSubtreeCommand(name string) *cobra.Command {
	for _, c := range rootCmd.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func walkMCPCommandTree(cmd *cobra.Command, path []string, tools *[]mcpToolSpec) {
	name := cmd.Name()
	if name == "" || name == "help" {
		return
	}

	if len(path) == 1 && isMCPManagementSubtree(name) {
		return
	}

	nextPath := append(append([]string{}, path...), name)
	children := visibleChildren(cmd)
	if len(children) == 0 && (cmd.Run != nil || cmd.RunE != nil) {
		*tools = append(*tools, buildMCPToolSpec(cmd, nextPath))
		return
	}
	for _, child := range children {
		walkMCPCommandTree(child, nextPath, tools)
	}
}

func visibleChildren(cmd *cobra.Command) []*cobra.Command {
	children := cmd.Commands()
	visible := make([]*cobra.Command, 0, len(children))
	for _, child := range children {
		if child == nil || child.Hidden || child.Name() == "help" {
			continue
		}
		visible = append(visible, child)
	}
	return visible
}

func isMCPManagementSubtree(name string) bool {
	switch name {
	case "auth", "config", "permissions":
		return true
	default:
		return false
	}
}

func buildMCPToolSpec(cmd *cobra.Command, path []string) mcpToolSpec {
	positional := parsePositionalArgsFromUse(cmd.Use)
	flags := collectMCPFlags(cmd)

	toolName := buildMCPToolName(path)
	desc := strings.TrimSpace(cmd.Short)
	if desc == "" {
		desc = "HelpScout CLI operation"
	}
	desc = fmt.Sprintf("%s (CLI: hs %s)", desc, strings.Join(path, " "))

	spec := mcpToolSpec{
		Name:           toolName,
		Description:    desc,
		CommandPath:    append([]string{}, path...),
		PositionalArgs: positional,
		Flags:          flags,
	}
	spec.InputSchema = buildMCPInputSchema(spec)
	return spec
}

func buildMCPToolName(path []string) string {
	parts := make([]string, 0, len(path)+1)
	parts = append(parts, "helpscout")
	for _, segment := range path {
		name := sanitizeMCPPropertyName(segment)
		if name == "" {
			continue
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, "_")
}

func parsePositionalArgsFromUse(use string) []mcpPositionalArgSpec {
	fields := strings.Fields(use)
	if len(fields) < 2 {
		return nil
	}

	args := make([]mcpPositionalArgSpec, 0, len(fields)-1)
	seen := map[string]int{}

	for _, token := range fields[1:] {
		required := strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">")
		optional := strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]")
		if !required && !optional {
			continue
		}

		raw := strings.Trim(token, "<>[]")
		raw = strings.TrimSuffix(raw, "...")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			raw = "arg"
		}

		property := sanitizeMCPPropertyName(raw)
		if property == "" {
			property = "arg"
		}
		if seen[property] > 0 {
			property = fmt.Sprintf("%s_%d", property, seen[property]+1)
		}
		seen[property]++

		args = append(args, mcpPositionalArgSpec{
			Name:     raw,
			Property: property,
			Required: required,
		})
	}

	return args
}

func collectMCPFlags(cmd *cobra.Command) []mcpFlagSpec {
	flags := make([]mcpFlagSpec, 0, 24)
	seen := map[string]struct{}{}

	add := func(flag *pflag.Flag, required bool) {
		if flag == nil || flag.Name == "help" {
			return
		}
		if _, ok := seen[flag.Name]; ok {
			return
		}
		seen[flag.Name] = struct{}{}

		flags = append(flags, mcpFlagSpec{
			Name:     flag.Name,
			Property: sanitizeMCPPropertyName(flag.Name),
			Usage:    flag.Usage,
			Type:     flag.Value.Type(),
			Required: required,
		})
	}

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		add(flag, isRequiredMCPFlag(flag))
	})

	// Make shared runtime controls available for list-heavy and PII-sensitive commands.
	for _, name := range []string{"no-paginate", "page", "per-page", "unredacted"} {
		add(cmd.InheritedFlags().Lookup(name), false)
	}

	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Name < flags[j].Name
	})
	return flags
}

func isRequiredMCPFlag(flag *pflag.Flag) bool {
	if flag == nil || flag.Annotations == nil {
		return false
	}
	_, ok := flag.Annotations[cobra.BashCompOneRequiredFlag]
	return ok
}

func buildMCPInputSchema(spec mcpToolSpec) map[string]any {
	properties := map[string]any{}
	required := make([]string, 0, len(spec.PositionalArgs)+len(spec.Flags))

	for _, arg := range spec.PositionalArgs {
		properties[arg.Property] = map[string]any{
			"type":        "string",
			"description": fmt.Sprintf("Positional argument: %s", arg.Name),
		}
		if arg.Required {
			required = append(required, arg.Property)
		}
	}

	for _, flag := range spec.Flags {
		properties[flag.Property] = mcpSchemaForFlag(flag)
		if flag.Required {
			required = append(required, flag.Property)
		}
	}

	properties["output_mode"] = map[string]any{
		"type":        "string",
		"enum":        []string{mcpOutputJSON, mcpOutputJSONFull},
		"default":     mcpOutputJSON,
		"description": "Response format from CLI execution",
	}

	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		sort.Strings(required)
		schema["required"] = required
	}
	return schema
}

func mcpSchemaForFlag(flag mcpFlagSpec) map[string]any {
	schema := map[string]any{
		"description": flag.Usage,
	}

	switch flag.Type {
	case "bool":
		schema["type"] = "boolean"
	case "int", "int64", "uint", "uint64":
		schema["type"] = "integer"
	case "stringSlice", "stringArray":
		schema["type"] = "array"
		schema["items"] = map[string]any{"type": "string"}
	default:
		schema["type"] = "string"
	}

	return schema
}

func sanitizeMCPPropertyName(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return ""
	}

	var b strings.Builder
	prevUnderscore := false
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}

	out := strings.Trim(b.String(), "_")
	if out == "" {
		return ""
	}
	if out[0] >= '0' && out[0] <= '9' {
		out = "arg_" + out
	}
	return out
}
