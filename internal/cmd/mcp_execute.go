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
	argv, commandLine, err := r.buildInvocation(spec, args)
	if err != nil {
		return mcpErrorResult(err.Error())
	}

	command := exec.CommandContext(ctx, r.executablePath, argv...)
	command.Env = setEnvVar(os.Environ(), "HS_NO_UPDATE_CHECK", "1")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	stdoutText := strings.TrimSpace(stdout.String())
	stderrText := strings.TrimSpace(stderr.String())

	if runErr != nil {
		msg := fmt.Sprintf("command failed: %s\n%s", commandLine, strings.TrimSpace(stderrText))
		if msg == fmt.Sprintf("command failed: %s\n", commandLine) && stdoutText != "" {
			msg = fmt.Sprintf("command failed: %s\n%s", commandLine, stdoutText)
		}
		if msg == fmt.Sprintf("command failed: %s\n", commandLine) {
			msg = fmt.Sprintf("command failed: %s\n%s", commandLine, runErr.Error())
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
	argv := make([]string, 0, len(spec.CommandPath)+8)
	if r.configPath != "" {
		argv = append(argv, "--config", r.configPath)
	}
	if r.debug {
		argv = append(argv, "--debug=true")
	}
	argv = append(argv, spec.CommandPath...)

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
		return nil, "", fmt.Errorf("unknown arguments: %s", strings.Join(unknown, ", "))
	}

	for _, arg := range spec.PositionalArgs {
		raw, ok := args[arg.Property]
		if !ok {
			if arg.Required {
				return nil, "", fmt.Errorf("missing required argument: %s", arg.Property)
			}
			continue
		}

		value, err := mcpStringFromRaw(raw)
		if err != nil {
			return nil, "", fmt.Errorf("invalid %s: %w", arg.Property, err)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			if arg.Required {
				return nil, "", fmt.Errorf("missing required argument: %s", arg.Property)
			}
			continue
		}
		argv = append(argv, value)
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
				return nil, "", fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			argv = append(argv, fmt.Sprintf("--%s=%t", flag.Name, v))
		case "int", "int64", "uint", "uint64":
			v, err := mcpIntFromRaw(raw)
			if err != nil {
				return nil, "", fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			argv = append(argv, "--"+flag.Name, strconv.FormatInt(v, 10))
		case "stringSlice", "stringArray":
			values, err := mcpStringSliceFromRaw(raw)
			if err != nil {
				return nil, "", fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			for _, value := range values {
				argv = append(argv, "--"+flag.Name, value)
			}
		default:
			v, err := mcpStringFromRaw(raw)
			if err != nil {
				return nil, "", fmt.Errorf("invalid %s: %w", flag.Property, err)
			}
			argv = append(argv, "--"+flag.Name, v)
		}
	}

	outputMode := r.defaultOutputMode
	if raw, ok := args["output_mode"]; ok {
		mode, err := mcpStringFromRaw(raw)
		if err != nil {
			return nil, "", fmt.Errorf("invalid output_mode: %w", err)
		}
		mode = strings.ToLower(strings.TrimSpace(mode))
		if mode == "json-full" {
			mode = mcpOutputJSONFull
		}
		if !isValidMCPOutputMode(mode) {
			return nil, "", fmt.Errorf("invalid output_mode: %q (expected json|json_full)", mode)
		}
		outputMode = mode
	}

	formatFlag := "json"
	if outputMode == mcpOutputJSONFull {
		formatFlag = "json-full"
	}
	argv = append(argv, "--format", formatFlag)

	commandLine := "hs " + strings.Join(argv, " ")
	return argv, commandLine, nil
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
