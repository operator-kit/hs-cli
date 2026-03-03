package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const mcpProtocolVersion = "2024-11-05"

type mcpServer struct {
	reader       *bufio.Reader
	writer       *bufio.Writer
	toolsByName  map[string]mcpToolSpec
	orderedTools []mcpToolSpec
	runner       *mcpCommandRunner
}

func newMCPServer(defaultOutputMode, configPath string, debug bool) (*mcpServer, error) {
	tools, err := discoverMCPTools()
	if err != nil {
		return nil, err
	}

	toolMap := make(map[string]mcpToolSpec, len(tools))
	for _, tool := range tools {
		toolMap[tool.Name] = tool
	}

	runner, err := newMCPCommandRunner(defaultOutputMode, configPath, debug)
	if err != nil {
		return nil, err
	}

	return &mcpServer{
		reader:       bufio.NewReader(os.Stdin),
		writer:       bufio.NewWriter(os.Stdout),
		toolsByName:  toolMap,
		orderedTools: tools,
		runner:       runner,
	}, nil
}

func (s *mcpServer) serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		req, err := readMCPRequest(s.reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			writeErr := writeMCPResponse(s.writer, mcpResponse{
				JSONRPC: "2.0",
				Error: &mcpRPCError{
					Code:    -32700,
					Message: "parse error",
					Data:    err.Error(),
				},
			})
			if writeErr != nil {
				return writeErr
			}
			continue
		}

		resp := s.handleRequest(ctx, req)
		if resp == nil {
			continue
		}
		if err := writeMCPResponse(s.writer, *resp); err != nil {
			return err
		}
	}
}

func (s *mcpServer) handleRequest(ctx context.Context, req mcpRequest) *mcpResponse {
	switch req.Method {
	case "initialize":
		return &mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": mcpProtocolVersion,
				"capabilities": map[string]any{
					"tools": map[string]any{
						"listChanged": false,
					},
				},
				"serverInfo": map[string]any{
					"name":    "hs-cli",
					"version": versionStr,
				},
			},
		}
	case "notifications/initialized":
		return nil
	case "ping":
		return &mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		}
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.orderedTools))
		for _, tool := range s.orderedTools {
			tools = append(tools, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}

		return &mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": tools,
			},
		}
	case "tools/call":
		var params mcpToolCallParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return &mcpResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &mcpRPCError{
						Code:    -32602,
						Message: "invalid params",
						Data:    err.Error(),
					},
				}
			}
		}

		spec, ok := s.toolsByName[params.Name]
		if !ok {
			return &mcpResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &mcpRPCError{
					Code:    -32602,
					Message: "unknown tool",
					Data:    params.Name,
				},
			}
		}

		result := s.runner.execute(ctx, spec, params.Arguments)
		return &mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
	default:
		if req.ID == nil {
			return nil
		}
		return &mcpResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &mcpRPCError{
				Code:    -32601,
				Message: "method not found",
				Data:    req.Method,
			},
		}
	}
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id,omitempty"`
	Result  any          `json:"result,omitempty"`
	Error   *mcpRPCError `json:"error,omitempty"`
}

type mcpRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolCallParams struct {
	Name      string                     `json:"name"`
	Arguments map[string]json.RawMessage `json:"arguments,omitempty"`
}

func readMCPRequest(reader *bufio.Reader) (mcpRequest, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return mcpRequest{}, err
	}
	line = strings.TrimSpace(line)
	for line == "" {
		line, err = reader.ReadString('\n')
		if err != nil {
			return mcpRequest{}, err
		}
		line = strings.TrimSpace(line)
	}

	var payload []byte
	if strings.HasPrefix(line, "{") {
		payload = []byte(line)
	} else {
		contentLength := -1
		if n, ok, err := parseContentLengthHeader(line); err != nil {
			return mcpRequest{}, err
		} else if ok {
			contentLength = n
		}

		for {
			headerLine, err := reader.ReadString('\n')
			if err != nil {
				return mcpRequest{}, err
			}
			headerLine = strings.TrimSpace(headerLine)
			if headerLine == "" {
				break
			}
			if n, ok, err := parseContentLengthHeader(headerLine); err != nil {
				return mcpRequest{}, err
			} else if ok {
				contentLength = n
			}
		}

		if contentLength < 0 {
			return mcpRequest{}, fmt.Errorf("missing Content-Length header")
		}

		payload = make([]byte, contentLength)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return mcpRequest{}, err
		}
	}

	var req mcpRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return mcpRequest{}, err
	}
	return req, nil
}

func parseContentLengthHeader(line string) (int, bool, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return 0, false, nil
	}
	if strings.TrimSpace(strings.ToLower(parts[0])) != "content-length" {
		return 0, false, nil
	}
	length, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, false, fmt.Errorf("invalid Content-Length value")
	}
	if length < 0 {
		return 0, false, fmt.Errorf("negative Content-Length value")
	}
	return length, true, nil
}

func writeMCPResponse(writer *bufio.Writer, resp mcpResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.WriteByte('\n'); err != nil {
		return err
	}
	return writer.Flush()
}
