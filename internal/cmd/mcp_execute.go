package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type mcpToolCallResult struct {
	Content           []mcpToolResultContent `json:"content"`
	StructuredContent any                    `json:"structuredContent,omitempty"`
	IsError           bool                   `json:"isError,omitempty"`
}

type mcpToolResultContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type mcpCommandRunner struct {
	executablePath    string
	defaultOutputMode string
	configPath        string
	debug             bool
}

type mcpInvocation struct {
	Args            []string
	Stdin           []byte
	SafeDisplay     string
	ProtectedValues []string
}

func newMCPCommandRunner(defaultOutputMode, configPath string, debug bool) (*mcpCommandRunner, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}
	return &mcpCommandRunner{
		executablePath:    exePath,
		defaultOutputMode: defaultOutputMode,
		configPath:        configPath,
		debug:             debug,
	}, nil
}

func (r *mcpCommandRunner) execute(ctx context.Context, spec mcpToolSpec, args map[string]json.RawMessage) mcpToolCallResult {
	invocation, err := r.buildExecutionInvocation(spec, args)
	if err != nil {
		return mcpErrorResult(err.Error())
	}

	command := exec.CommandContext(ctx, r.executablePath, invocation.Args...)
	command.Env = setEnvVar(os.Environ(), "HS_NO_UPDATE_CHECK", "1")
	command.Stdin = bytes.NewReader(invocation.Stdin)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	stdoutText := strings.TrimSpace(redactProtectedValues(stdout.String(), invocation.ProtectedValues))
	stderrText := strings.TrimSpace(redactProtectedValues(stderr.String(), invocation.ProtectedValues))

	if runErr != nil {
		msg := fmt.Sprintf("command failed: %s\n%s", invocation.SafeDisplay, strings.TrimSpace(stderrText))
		if msg == fmt.Sprintf("command failed: %s\n", invocation.SafeDisplay) && stdoutText != "" {
			msg = fmt.Sprintf("command failed: %s\n%s", invocation.SafeDisplay, stdoutText)
		}
		if msg == fmt.Sprintf("command failed: %s\n", invocation.SafeDisplay) {
			msg = fmt.Sprintf("command failed: %s\n%s", invocation.SafeDisplay,
				redactProtectedValues(runErr.Error(), invocation.ProtectedValues))
		}
		return mcpErrorResult(msg)
	}

	result := mcpToolCallResult{
		Content: []mcpToolResultContent{{
			Type: "text",
			Text: formatMCPTextOutput(stdoutText, stderrText),
		}},
	}

	if structured, ok := parseStructuredContent(stdoutText); ok {
		result.StructuredContent = structured
	}

	return result
}

func (r *mcpCommandRunner) buildInvocation(spec mcpToolSpec, args map[string]json.RawMessage) ([]string, string, error) {
	invocation, err := r.buildExecutionInvocation(spec, args)
	if err != nil {
		return nil, "", err
	}
	return invocation.Args, invocation.SafeDisplay, nil
}

func (r *mcpCommandRunner) buildExecutionInvocation(spec mcpToolSpec, args map[string]json.RawMessage) (mcpInvocation, error) {
	globalArgs := make([]string, 0, 4)
	if r.configPath != "" {
		globalArgs = append(globalArgs, "--config", r.configPath)
	}
	if r.debug {
		globalArgs = append(globalArgs, "--debug=true")
	}
	commandArgs := append([]string{}, spec.CommandPath...)
	safeArgs := append([]string{}, spec.CommandPath...)
	envelope := protectedInputEnvelope{
		Schema:  protectedInputSchema,
		Command: append([]string{}, spec.CommandPath...),
		Flags:   map[string]json.RawMessage{},
	}
	protectedValues := make([]string, 0, len(args))

	allowed := map[string]struct{}{
		"output_mode": {},
	}
	for _, a := range spec.PositionalArgs {
		allowed[a.Property] = struct{}{}
	}
	for _, f := range spec.Flags {
		allowed[f.Property] = struct{}{}
	}

	unknown := make([]string, 0, 4)
	for key := range args {
		if _, ok := allowed[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return mcpInvocation{}, fmt.Errorf("unknown arguments: %s", strings.Join(unknown, ", "))
	}

	for _, arg := range spec.PositionalArgs {
		raw, ok := args[arg.Property]
		if !ok {
			if arg.Required {
				return mcpInvocation{}, fmt.Errorf("missing required argument: %s", arg.Property)
			}
			continue
		}

		value, err := mcpStringFromRaw(raw)
		if err != nil {
			return mcpInvocation{}, fmt.Errorf("invalid %s: %w", arg.Property, err)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			if arg.Required {
				return mcpInvocation{}, fmt.Errorf("missing required argument: %s", arg.Property)
			}
			continue
		}
		position := len(commandArgs) - len(spec.CommandPath)
		placeholder := protectedArgumentPlaceholder(position)
		commandArgs = append(commandArgs, placeholder)
		safeArgs = append(safeArgs, "[protected]")
		envelope.Positionals = append(envelope.Positionals, protectedPositionalInput{Index: position, Value: value})
	}

	for _, flag := range spec.Flags {
		raw, ok := args[flag.Property]
		if !ok {
			continue
		}

		switch flag.Type {
		case "bool":
			v, err := mcpBoolFromRaw(raw)
			if err != nil {
				return mcpInvocation{}, fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			value := fmt.Sprintf("--%s=%t", flag.Name, v)
			commandArgs = append(commandArgs, value)
			safeArgs = append(safeArgs, value)
		case "int", "int64", "uint", "uint64":
			v, err := mcpIntFromRaw(raw)
			if err != nil {
				return mcpInvocation{}, fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			value := strconv.FormatInt(v, 10)
			commandArgs = append(commandArgs, "--"+flag.Name, value)
			safeArgs = append(safeArgs, "--"+flag.Name, value)
		case "stringSlice", "stringArray":
			values, err := mcpStringSliceFromRaw(raw)
			if err != nil {
				return mcpInvocation{}, fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			normalized, marshalErr := json.Marshal(values)
			if marshalErr != nil {
				return mcpInvocation{}, fmt.Errorf("encode %s: %w", flag.Property, marshalErr)
			}
			envelope.Flags[flag.Name] = normalized
			if flag.Protected {
				protectedValues = append(protectedValues, values...)
			}
			safeArgs = append(safeArgs, "--"+flag.Name, "[protected]")
		default:
			v, err := mcpStringFromRaw(raw)
			if err != nil {
				return mcpInvocation{}, fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			normalized, marshalErr := json.Marshal(v)
			if marshalErr != nil {
				return mcpInvocation{}, fmt.Errorf("encode %s: %w", flag.Property, marshalErr)
			}
			envelope.Flags[flag.Name] = normalized
			if flag.Protected {
				protectedValues = append(protectedValues, v)
			}
			safeArgs = append(safeArgs, "--"+flag.Name, "[protected]")
		}
	}

	outputMode := r.defaultOutputMode
	if raw, ok := args["output_mode"]; ok {
		mode, err := mcpStringFromRaw(raw)
		if err != nil {
			return mcpInvocation{}, fmt.Errorf("invalid output_mode: %w", err)
		}
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "json-full" {
			mode = mcpOutputJSONFull
		}
		if !isValidMCPOutputMode(mode) {
			return mcpInvocation{}, fmt.Errorf("invalid output_mode: %q (expected json|json_full)", mode)
		}
		outputMode = mode
	}

	formatFlag := "json"
	if outputMode == mcpOutputJSONFull {
		formatFlag = "json-full"
	}
	commandArgs = append(commandArgs, "--format", formatFlag)
	safeArgs = append(safeArgs, "--format", formatFlag)

	argv := append([]string{}, globalArgs...)
	var stdin []byte
	if len(envelope.Positionals) > 0 || len(envelope.Flags) > 0 {
		var err error
		stdin, err = json.Marshal(envelope)
		if err != nil {
			return mcpInvocation{}, fmt.Errorf("encode protected invocation: %w", err)
		}
		argv = append(argv, "--"+protectedInputFlagName, "-")
	}
	argv = append(argv, commandArgs...)
	return mcpInvocation{
		Args:            argv,
		Stdin:           stdin,
		SafeDisplay:     "hs " + strings.Join(safeArgs, " "),
		ProtectedValues: compactProtectedValues(protectedValues),
	}, nil
}

func mcpErrorResult(message string) mcpToolCallResult {
	return mcpToolCallResult{
		Content: []mcpToolResultContent{{
			Type: "text",
			Text: strings.TrimSpace(message),
		}},
		IsError: true,
	}
}

func formatMCPTextOutput(stdoutText, stderrText string) string {
	switch {
	case stdoutText != "" && stderrText != "":
		return stdoutText + "\n\nstderr:\n" + stderrText
	case stdoutText != "":
		return stdoutText
	case stderrText != "":
		return stderrText
	default:
		return "{}"
	}
}

func parseJSONValue(value string) (any, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

func parseStructuredContent(value string) (map[string]any, bool) {
	parsed, ok := parseJSONValue(value)
	if !ok {
		return nil, false
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return nil, false
	}
	return obj, true
}

func setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

func mcpStringFromRaw(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err == nil {
		return n.String(), nil
	}

	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return strconv.FormatBool(b), nil
	}

	return "", fmt.Errorf("expected string")
}

func mcpBoolFromRaw(raw json.RawMessage) (bool, error) {
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return false, fmt.Errorf("expected boolean")
		}
		return v, nil
	}

	return false, fmt.Errorf("expected boolean")
}

func mcpIntFromRaw(raw json.RawMessage) (int64, error) {
	var i int64
	if err := json.Unmarshal(raw, &i); err == nil {
		return i, nil
	}

	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if float64(int64(f)) != f {
			return 0, fmt.Errorf("expected integer")
		}
		return int64(f), nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("expected integer")
		}
		return v, nil
	}

	return 0, fmt.Errorf("expected integer")
}

func mcpStringSliceFromRaw(raw json.RawMessage) ([]string, error) {
	var ss []string
	if err := json.Unmarshal(raw, &ss); err == nil {
		return ss, nil
	}

	var generic []any
	if err := json.Unmarshal(raw, &generic); err == nil {
		out := make([]string, 0, len(generic))
		for _, v := range generic {
			switch t := v.(type) {
			case string:
				out = append(out, t)
			case float64:
				out = append(out, strconv.FormatFloat(t, 'f', -1, 64))
			case bool:
				out = append(out, strconv.FormatBool(t))
			default:
				return nil, fmt.Errorf("expected array of strings")
			}
		}
		return out, nil
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return []string{}, nil
		}
		if strings.Contains(single, ",") {
			parts := strings.Split(single, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			return out, nil
		}
		return []string{single}, nil
	}

	return nil, fmt.Errorf("expected string array")
}
