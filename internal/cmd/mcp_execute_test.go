package cmd

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPBuildInvocation_BuildsCLIArgs(t *testing.T) {
	runner := &mcpCommandRunner{defaultOutputMode: mcpOutputJSON}
	spec := mcpToolSpec{
		CommandPath: []string{"inbox", "conversations", "get"},
		PositionalArgs: []mcpPositionalArgSpec{
			{Name: "id", Property: "id", Required: true},
		},
		Flags: []mcpFlagSpec{
			{Name: "no-paginate", Property: "no_paginate", Type: "bool"},
			{Name: "page", Property: "page", Type: "int"},
			{Name: "tag", Property: "tag", Type: "stringSlice", Protected: true},
		},
	}

	args := map[string]json.RawMessage{
		"id":          json.RawMessage(`"12345"`),
		"no_paginate": json.RawMessage(`true`),
		"page":        json.RawMessage(`2`),
		"tag":         json.RawMessage(`["vip","billing"]`),
		"output_mode": json.RawMessage(`"json_full"`),
	}

	invocation, err := runner.buildExecutionInvocation(spec, args)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"--protected-input", "-",
		"inbox", "conversations", "get",
		"__HS_PROTECTED_ARG_0__",
		"--no-paginate=true",
		"--page", "2",
		"--format", "json-full",
	}, invocation.Args)
	assert.Equal(t, "hs inbox conversations get [protected] --no-paginate=true --page 2 --tag [protected] --format json-full", invocation.SafeDisplay)
	assert.ElementsMatch(t, []string{"vip", "billing"}, invocation.ProtectedValues)

	var envelope protectedInputEnvelope
	require.NoError(t, json.Unmarshal(invocation.Stdin, &envelope))
	assert.Equal(t, protectedInputSchema, envelope.Schema)
	assert.Equal(t, spec.CommandPath, envelope.Command)
	assert.Equal(t, []protectedPositionalInput{{Index: 0, Value: "12345"}}, envelope.Positionals)
	assert.JSONEq(t, `["vip","billing"]`, string(envelope.Flags["tag"]))
}

func TestRedactProtectedValues_CoversQuotedAndJSONEscapedErrors(t *testing.T) {
	protected := "private line one\nline two <tag>"
	rendered := fmt.Sprintf("raw=%s quoted=%q json=%s", protected, protected, `"private line one\nline two \u003ctag\u003e"`)
	out := redactProtectedValues(rendered, []string{protected})
	assert.NotContains(t, out, "private line")
	assert.NotContains(t, out, "line two")
	assert.NotContains(t, out, "tag")
	assert.Contains(t, out, "[protected]")
}

func TestMCPBuildInvocation_RejectsUnknownArguments(t *testing.T) {
	runner := &mcpCommandRunner{defaultOutputMode: mcpOutputJSON}
	spec := mcpToolSpec{
		CommandPath: []string{"inbox", "customers", "get"},
		PositionalArgs: []mcpPositionalArgSpec{
			{Name: "id", Property: "id", Required: true},
		},
	}

	_, _, err := runner.buildInvocation(spec, map[string]json.RawMessage{
		"id":      json.RawMessage(`"1"`),
		"unknown": json.RawMessage(`"x"`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown arguments")
}

func TestMCPStringSliceFromRaw_SupportsCommaSeparatedString(t *testing.T) {
	values, err := mcpStringSliceFromRaw(json.RawMessage(`"a, b, c"`))
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, values)
}

func TestMCPBuildInvocation_ForwardsGlobalConfigAndDebug(t *testing.T) {
	runner := &mcpCommandRunner{
		defaultOutputMode: mcpOutputJSON,
		configPath:        "/tmp/hs-config.yaml",
		debug:             true,
	}
	spec := mcpToolSpec{
		CommandPath: []string{"inbox", "mailboxes", "list"},
	}

	invocation, err := runner.buildExecutionInvocation(spec, map[string]json.RawMessage{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"--config", "/tmp/hs-config.yaml",
		"--debug=true",
		"inbox", "mailboxes", "list",
		"--format", "json",
	}, invocation.Args)
	assert.Empty(t, invocation.Stdin)
}

func TestMCPBuildInvocation_RejectsEmptyRequiredPositionalArgument(t *testing.T) {
	runner := &mcpCommandRunner{defaultOutputMode: mcpOutputJSON}
	spec := mcpToolSpec{
		CommandPath: []string{"inbox", "teams", "members"},
		PositionalArgs: []mcpPositionalArgSpec{
			{Name: "team-id", Property: "team_id", Required: true},
		},
	}

	_, _, err := runner.buildInvocation(spec, map[string]json.RawMessage{
		"team_id": json.RawMessage(`""`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required argument: team_id")
}

func TestParseStructuredContent_ObjectOnly(t *testing.T) {
	obj, ok := parseStructuredContent(`{"ok":true}`)
	require.True(t, ok)
	assert.Equal(t, true, obj["ok"])

	_, ok = parseStructuredContent(`[{"id":1}]`)
	assert.False(t, ok)
}
