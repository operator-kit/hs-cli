package cmd

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mediumSensitiveEmail = "alice.medium@example.test"
	mediumSensitiveBody  = "Medical escalation for Alice Medium: diagnosis fixture 8472"
)

func mediumSensitiveMCPFixture() (mcpToolSpec, map[string]json.RawMessage) {
	spec := mcpToolSpec{
		CommandPath: []string{"inbox", "conversations", "threads", "reply"},
		PositionalArgs: []mcpPositionalArgSpec{
			{Name: "conversation-id", Property: "conversation_id", Required: true},
		},
		Flags: []mcpFlagSpec{
			{Name: "customer", Property: "customer", Type: "string", Required: true, Protected: true},
			{Name: "body", Property: "body", Type: "string", Required: true, Protected: true},
		},
	}
	args := map[string]json.RawMessage{
		"conversation_id": json.RawMessage(`"42"`),
		"customer":        json.RawMessage(`"` + mediumSensitiveEmail + `"`),
		"body":            json.RawMessage(`"` + mediumSensitiveBody + `"`),
	}
	return spec, args
}

func TestPIIRegression_Medium10_SensitiveMCPInputsStayOffProcessBoundaries(t *testing.T) {
	spec, args := mediumSensitiveMCPFixture()
	sensitiveValues := []string{mediumSensitiveEmail, mediumSensitiveBody}

	t.Run("child argv", func(t *testing.T) {
		runner := &mcpCommandRunner{defaultOutputMode: mcpOutputJSON}
		argv, commandLine, err := runner.buildInvocation(spec, args)
		require.NoError(t, err)

		joinedArgv := strings.Join(argv, "\x00")
		for _, value := range sensitiveValues {
			assert.NotContains(t, joinedArgv, value,
				"sensitive MCP input must be transported outside child-process argv")
			assert.NotContains(t, commandLine, value,
				"diagnostic command text must never reconstruct sensitive arguments")
		}
	})

	t.Run("echoed command errors", func(t *testing.T) {
		runner := &mcpCommandRunner{
			executablePath:    filepath.Join(t.TempDir(), "missing-hs"),
			defaultOutputMode: mcpOutputJSON,
		}
		result := runner.execute(context.Background(), spec, args)
		require.True(t, result.IsError)

		var rendered strings.Builder
		for _, content := range result.Content {
			rendered.WriteString(content.Text)
		}
		for _, value := range sensitiveValues {
			assert.NotContains(t, rendered.String(), value,
				"MCP failures must not echo sensitive tool arguments")
		}
	})

	t.Run("direct CLI requires protected transport", func(t *testing.T) {
		previousOutput := output.Out
		saveRestore(t)
		apiCalled := false
		setupTest(&mockClient{CreateReplyFn: func(context.Context, string, any) error {
			apiCalled = true
			return nil
		}})
		t.Cleanup(func() { output.Out = previousOutput })
		resetChangedFlags(rootCmd)
		rootCmd.SetArgs([]string{
			"inbox", "conversations", "threads", "reply", "42",
			"--customer", mediumSensitiveEmail,
			"--body", mediumSensitiveBody,
		})

		err := rootCmd.Execute()
		require.Error(t, err)
		assert.NotContains(t, err.Error(), mediumSensitiveBody)
		assert.NotContains(t, err.Error(), mediumSensitiveEmail)
		assert.Contains(t, err.Error(), "--protected-input")
		assert.False(t, apiCalled, "unprotected PII must be rejected before API work")
	})
}
