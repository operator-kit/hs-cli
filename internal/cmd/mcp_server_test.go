package cmd

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadMCPRequest_ContentLengthFraming(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	frame := "Content-Length: " + strconv.Itoa(len(payload)) + "\r\n\r\n" + payload

	req, err := readMCPRequest(bufio.NewReader(strings.NewReader(frame)))
	require.NoError(t, err)
	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, "ping", req.Method)
	assert.Equal(t, float64(1), req.ID)
}

func TestMCPHandleRequest_ToolsList(t *testing.T) {
	server := &mcpServer{
		toolsByName: map[string]mcpToolSpec{
			"helpscout_inbox_conversations_list": {
				Name:        "helpscout_inbox_conversations_list",
				Description: "List conversations",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		orderedTools: []mcpToolSpec{{
			Name:        "helpscout_inbox_conversations_list",
			Description: "List conversations",
			InputSchema: map[string]any{"type": "object"},
		}},
	}

	req := mcpRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	resp := server.handleRequest(context.Background(), req)
	require.NotNil(t, resp)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)

	resultMap, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	tools, ok := resultMap["tools"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, tools, 1)
	assert.Equal(t, "helpscout_inbox_conversations_list", tools[0]["name"])
}

func TestWriteMCPResponse_JSONLineFraming(t *testing.T) {
	var out strings.Builder
	writer := bufio.NewWriter(&out)

	err := writeMCPResponse(writer, mcpResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result:  map[string]any{"ok": true},
	})
	require.NoError(t, err)

	raw := out.String()
	assert.Equal(t, "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n", raw)
}
