package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverMCPTools_ExcludesManagementCommands(t *testing.T) {
	tools, err := discoverMCPTools()
	require.NoError(t, err)
	require.NotEmpty(t, tools)

	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}

	assert.Contains(t, names, "helpscout_inbox_conversations_list")
	assert.Contains(t, names, "helpscout_inbox_conversations_threads_reply")
	assert.Contains(t, names, "helpscout_inbox_tools_briefing")

	for _, name := range names {
		assert.NotContains(t, name, "_inbox_auth_")
		assert.NotContains(t, name, "_inbox_config_")
	}
	assert.NotContains(t, names, "helpscout_inbox_permissions")
}

func TestBuildMCPToolSpec_IncludesOutputModeSchema(t *testing.T) {
	tools, err := discoverMCPTools()
	require.NoError(t, err)
	require.NotEmpty(t, tools)

	found := false
	for _, tool := range tools {
		if tool.Name != "helpscout_inbox_conversations_list" {
			continue
		}
		found = true
		props, ok := tool.InputSchema["properties"].(map[string]any)
		require.True(t, ok)
		mode, ok := props["output_mode"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "string", mode["type"])
		assert.Equal(t, mcpOutputJSON, mode["default"])
	}
	assert.True(t, found, "expected conversations list tool to be present")
}
