package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	mcpOutputJSON     = "json"
	mcpOutputJSONFull = "json_full"
)

func init() {
	rootCmd.AddCommand(newMCPCmd())
}

func newMCPCmd() *cobra.Command {
	var transport string
	var defaultOutputMode string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for HelpScout tools",
		RunE: func(cmd *cobra.Command, args []string) error {
			if transport != "stdio" {
				return fmt.Errorf("unsupported transport: %s (only stdio is supported)", transport)
			}

			defaultOutputMode = strings.ToLower(strings.TrimSpace(defaultOutputMode))
			if defaultOutputMode == "json-full" {
				defaultOutputMode = mcpOutputJSONFull
			}

			if !isValidMCPOutputMode(defaultOutputMode) {
				return fmt.Errorf("invalid --default-output-mode: %q (expected json|json_full)", defaultOutputMode)
			}

			server, err := newMCPServer(defaultOutputMode, cfgPath, debug)
			if err != nil {
				return err
			}
			return server.serve(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&transport, "transport", "t", "stdio", "MCP transport (stdio)")
	cmd.Flags().StringVar(&defaultOutputMode, "default-output-mode", mcpOutputJSON, "default output mode for tool calls: json|json_full")
	return cmd
}

func isValidMCPOutputMode(mode string) bool {
	return mode == mcpOutputJSON || mode == mcpOutputJSONFull
}
