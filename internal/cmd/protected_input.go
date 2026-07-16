package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	protectedInputFlagName  = "protected-input"
	protectedFlagAnnotation = "operator-kit.io/protected-input"
	protectedInputSchema    = 1
	maxProtectedInputBytes  = 4 << 20
)

var protectedInputPath string

type protectedInputApplication struct {
	flags  map[*pflag.Flag]struct{}
	values []string
}

type protectedInputEnvelope struct {
	Schema      int                        `json:"schema"`
	Command     []string                   `json:"command"`
	Positionals []protectedPositionalInput `json:"positionals,omitempty"`
	Flags       map[string]json.RawMessage `json:"flags,omitempty"`
}

type protectedPositionalInput struct {
	Index int    `json:"index"`
	Value string `json:"value"`
}

func protectedArgumentPlaceholder(index int) string {
	return fmt.Sprintf("__HS_PROTECTED_ARG_%d__", index)
}

func applyProtectedInput(cmd *cobra.Command, args []string) (protectedInputApplication, error) {
	applied := protectedInputApplication{flags: make(map[*pflag.Flag]struct{})}
	path := strings.TrimSpace(protectedInputPath)
	if path == "" {
		return applied, nil
	}

	// Cobra command trees are reused by tests. Clear the process-scoped control
	// as soon as it has been captured so a later invocation cannot reread stdin.
	protectedInputPath = ""
	if flag := cmd.Root().PersistentFlags().Lookup(protectedInputFlagName); flag != nil {
		flag.Changed = false
	}

	raw, err := readProtectedInput(cmd, path)
	if err != nil {
		return protectedInputApplication{}, err
	}

	var envelope protectedInputEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return protectedInputApplication{}, fmt.Errorf("decode protected input: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return protectedInputApplication{}, fmt.Errorf("decode protected input: %w", err)
	}
	if envelope.Schema != protectedInputSchema {
		return protectedInputApplication{}, fmt.Errorf("unsupported protected input schema %d", envelope.Schema)
	}
	if !reflect.DeepEqual(envelope.Command, commandPathSegments(cmd)) {
		return protectedInputApplication{}, fmt.Errorf("protected input command does not match %q", cmd.CommandPath())
	}

	seenPositions := make(map[int]struct{}, len(envelope.Positionals))
	for _, input := range envelope.Positionals {
		if input.Index < 0 || input.Index >= len(args) {
			return protectedInputApplication{}, fmt.Errorf("protected positional index %d is out of range", input.Index)
		}
		if _, exists := seenPositions[input.Index]; exists {
			return protectedInputApplication{}, fmt.Errorf("duplicate protected positional index %d", input.Index)
		}
		seenPositions[input.Index] = struct{}{}
		if args[input.Index] != protectedArgumentPlaceholder(input.Index) {
			return protectedInputApplication{}, fmt.Errorf("protected positional index %d has no matching placeholder", input.Index)
		}
		args[input.Index] = input.Value
	}

	for name, rawValue := range envelope.Flags {
		if name == protectedInputFlagName {
			return protectedInputApplication{}, fmt.Errorf("protected input cannot set --%s", protectedInputFlagName)
		}
		flag := lookupCommandFlag(cmd, name)
		if flag == nil {
			return protectedInputApplication{}, fmt.Errorf("protected input contains unknown flag --%s", name)
		}
		if err := setProtectedFlagValue(flag, rawValue); err != nil {
			return protectedInputApplication{}, fmt.Errorf("protected input flag --%s: %w", name, err)
		}
		applied.flags[flag] = struct{}{}
		if flag.Annotations != nil && len(flag.Annotations[protectedFlagAnnotation]) > 0 {
			applied.values = append(applied.values, protectedValuesFromRaw(flag, rawValue)...)
		}
	}

	applied.values = compactProtectedValues(applied.values)
	return applied, nil
}

func protectedValuesFromRaw(flag *pflag.Flag, raw json.RawMessage) []string {
	switch flag.Value.Type() {
	case "string":
		value, err := decodeProtectedString(raw)
		if err == nil {
			return []string{value}
		}
	case "stringSlice", "stringArray":
		values, err := decodeProtectedStringSlice(raw)
		if err == nil {
			return values
		}
	}
	return nil
}

func markProtectedFlags(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			panic(fmt.Sprintf("protected flag --%s is not registered on %s", name, cmd.CommandPath()))
		}
		if flag.Annotations == nil {
			flag.Annotations = make(map[string][]string)
		}
		flag.Annotations[protectedFlagAnnotation] = []string{"true"}
		if !strings.Contains(flag.Usage, "protected input only") {
			flag.Usage += " (protected input only)"
		}
	}
}

func rejectUnprotectedFlagValues(cmd *cobra.Command, applied protectedInputApplication) error {
	var rejected string
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if rejected != "" || !flag.Changed || flag.Annotations == nil {
			return
		}
		if _, protected := flag.Annotations[protectedFlagAnnotation]; !protected {
			return
		}
		if _, safelyApplied := applied.flags[flag]; safelyApplied {
			return
		}
		rejected = flag.Name
	})
	if rejected == "" {
		return nil
	}
	return fmt.Errorf("--%s accepts protected input only; use --%s with a private file or stdin", rejected, protectedInputFlagName)
}

func readProtectedInput(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "-" {
		return readBoundedProtectedInput(cmd.InOrStdin())
	}

	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("open protected input: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("protected input must be a regular file")
	}
	if info.Size() > maxProtectedInputBytes {
		return nil, fmt.Errorf("protected input exceeds %d bytes", maxProtectedInputBytes)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("protected input file permissions must not allow group or other access")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open protected input: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) || !openedInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("protected input changed while opening")
	}
	if openedInfo.Size() > maxProtectedInputBytes {
		return nil, fmt.Errorf("protected input exceeds %d bytes", maxProtectedInputBytes)
	}
	if runtime.GOOS != "windows" && openedInfo.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("protected input file permissions must not allow group or other access")
	}
	return readBoundedProtectedInput(file)
}

func readBoundedProtectedInput(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxProtectedInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read protected input: %w", err)
	}
	if len(raw) > maxProtectedInputBytes {
		return nil, fmt.Errorf("protected input exceeds %d bytes", maxProtectedInputBytes)
	}
	return raw, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("multiple JSON values are not allowed")
}

func commandPathSegments(cmd *cobra.Command) []string {
	segments := make([]string, 0, 6)
	for current := cmd; current != nil && current.Parent() != nil; current = current.Parent() {
		segments = append(segments, current.Name())
	}
	for left, right := 0, len(segments)-1; left < right; left, right = left+1, right-1 {
		segments[left], segments[right] = segments[right], segments[left]
	}
	return segments
}

func lookupCommandFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if flag := cmd.Flags().Lookup(name); flag != nil {
		return flag
	}
	return cmd.InheritedFlags().Lookup(name)
}

func setProtectedFlagValue(flag *pflag.Flag, raw json.RawMessage) error {
	switch flag.Value.Type() {
	case "string":
		value, err := decodeProtectedString(raw)
		if err != nil {
			return err
		}
		if err := flag.Value.Set(value); err != nil {
			return err
		}
	case "stringSlice", "stringArray":
		values, err := decodeProtectedStringSlice(raw)
		if err != nil {
			return err
		}
		slice, ok := flag.Value.(pflag.SliceValue)
		if !ok {
			return fmt.Errorf("flag type %s does not support protected slices", flag.Value.Type())
		}
		if err := slice.Replace(values); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported protected flag type %s", flag.Value.Type())
	}
	flag.Changed = true
	return nil
}

func decodeProtectedString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("expected string")
	}
	return value, nil
}

func decodeProtectedStringSlice(raw json.RawMessage) ([]string, error) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, fmt.Errorf("expected array of strings")
	}
	return values, nil
}

func compactProtectedValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

func redactProtectedValues(text string, values []string) string {
	for _, value := range values {
		variants := []string{value, strconv.Quote(value)}
		if encoded, err := json.Marshal(value); err == nil {
			variants = append(variants, string(encoded))
		}
		for _, variant := range compactProtectedValues(variants) {
			text = strings.ReplaceAll(text, variant, "[protected]")
		}
	}
	return text
}
