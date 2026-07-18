package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func protectedReplyCommand(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "hs"}
	inbox := &cobra.Command{Use: "inbox"}
	conversations := &cobra.Command{Use: "conversations"}
	threads := newThreadsCmd()
	root.AddCommand(inbox)
	inbox.AddCommand(conversations)
	conversations.AddCommand(threads)

	for _, command := range threads.Commands() {
		if command.Name() == "reply" {
			return command
		}
	}
	t.Fatal("reply command not found")
	return nil
}

func TestApplyProtectedInput_AppliesPositionalsAndStringFlags(t *testing.T) {
	command := protectedReplyCommand(t)
	envelope := protectedInputEnvelope{
		Schema:  protectedInputSchema,
		Command: []string{"inbox", "conversations", "threads", "reply"},
		Positionals: []protectedPositionalInput{
			{Index: 0, Value: "42"},
		},
		Flags: map[string]json.RawMessage{
			"customer": json.RawMessage(`"alice@example.test"`),
			"body":     json.RawMessage(`"private fixture body"`),
			"to":       json.RawMessage(`["one@example.test","two@example.test"]`),
		},
	}
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	command.SetIn(bytes.NewReader(raw))
	protectedInputPath = "-"
	t.Cleanup(func() { protectedInputPath = "" })

	args := []string{protectedArgumentPlaceholder(0)}
	applied, err := applyProtectedInput(command, args)
	require.NoError(t, err)
	require.Len(t, applied.flags, 3)
	assert.ElementsMatch(t, []string{"alice@example.test", "private fixture body", "one@example.test", "two@example.test"}, applied.values)
	require.NoError(t, rejectUnprotectedFlagValues(command, applied))
	assert.Equal(t, []string{"42"}, args)

	customer, err := command.Flags().GetString("customer")
	require.NoError(t, err)
	body, err := command.Flags().GetString("body")
	require.NoError(t, err)
	to, err := command.Flags().GetStringSlice("to")
	require.NoError(t, err)
	assert.Equal(t, "alice@example.test", customer)
	assert.Equal(t, "private fixture body", body)
	assert.Equal(t, []string{"one@example.test", "two@example.test"}, to)
}

func TestRejectUnprotectedFlagValues_DoesNotEchoSensitiveValue(t *testing.T) {
	command := protectedReplyCommand(t)
	sensitive := "diagnosis fixture that must not appear"
	require.NoError(t, command.Flags().Set("body", sensitive))

	err := rejectUnprotectedFlagValues(command, protectedInputApplication{flags: map[*pflag.Flag]struct{}{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--body")
	assert.NotContains(t, err.Error(), sensitive)
}

func TestApplyProtectedInput_RejectsCommandMismatchAndOversize(t *testing.T) {
	t.Run("command mismatch", func(t *testing.T) {
		command := protectedReplyCommand(t)
		raw := `{"schema":1,"command":["inbox","customers","get"]}`
		command.SetIn(strings.NewReader(raw))
		protectedInputPath = "-"
		t.Cleanup(func() { protectedInputPath = "" })

		_, err := applyProtectedInput(command, []string{"42"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not match")
	})

	t.Run("oversize", func(t *testing.T) {
		command := protectedReplyCommand(t)
		command.SetIn(strings.NewReader(strings.Repeat("x", maxProtectedInputBytes+1)))
		protectedInputPath = "-"
		t.Cleanup(func() { protectedInputPath = "" })

		_, err := applyProtectedInput(command, []string{"42"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds")
	})

	t.Run("non-string flag value", func(t *testing.T) {
		command := protectedReplyCommand(t)
		raw := `{"schema":1,"command":["inbox","conversations","threads","reply"],"flags":{"body":42}}`
		command.SetIn(strings.NewReader(raw))
		protectedInputPath = "-"
		t.Cleanup(func() { protectedInputPath = "" })

		_, err := applyProtectedInput(command, []string{"42"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected string")
	})
}

func TestReadProtectedInput_ValidatesPrivateRegularFiles(t *testing.T) {
	t.Run("private regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "request.json")
		want := []byte(`{"schema":1}`)
		require.NoError(t, os.WriteFile(path, want, 0o600))

		got, err := readProtectedInput(&cobra.Command{}, path)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target.json")
		link := filepath.Join(dir, "link.json")
		require.NoError(t, os.WriteFile(target, []byte(`{}`), 0o600))
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symbolic links unavailable: %v", err)
		}

		_, err := readProtectedInput(&cobra.Command{}, link)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "regular file")
	})

	t.Run("public Unix permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("portable Windows mode bits do not describe the file ACL")
		}
		path := filepath.Join(t.TempDir(), "public.json")
		require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o644))

		_, err := readProtectedInput(&cobra.Command{}, path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permissions")
	})
}

func TestDecodeProtectedStringSliceRejectsNull(t *testing.T) {
	_, err := decodeProtectedStringSlice(json.RawMessage(`null`))
	require.Error(t, err)
}
