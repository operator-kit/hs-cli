package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/auth"
	"github.com/operator-kit/hs-cli/internal/config"
)

func newDocsAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Docs API authentication",
	}
	authCmd.AddCommand(docsAuthLoginCmd(), docsAuthStatusCmd(), docsAuthLogoutCmd())
	return authCmd
}

func docsAuthLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with HelpScout Docs API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			fmt.Print("Docs API Key: ")
			apiKey, _ := reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)

			if apiKey == "" {
				return fmt.Errorf("API key is required")
			}

			fmt.Print("Validating... ")
			client := api.NewDocs(apiKey, debug)
			_, err := client.ListCollections(context.Background(), nil)
			if err != nil {
				fmt.Println("failed")
				return fmt.Errorf("authentication failed: %w", err)
			}

			// Store in keyring, fall back to config file
			if err := auth.StoreDocsAPIKey(apiKey); err != nil {
				if err := promptConfigFallback(reader, cfgPath, err, func(c *config.Config) {
					c.DocsAPIKey = apiKey
				}); err != nil {
					return err
				}
			}

			fmt.Println("Authenticated.")
			return nil
		},
	}
}

func docsAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Docs API authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Try keyring first
			key, err := auth.LoadDocsAPIKey()
			if err == nil && key != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Authenticated (key: %s...%s)\n", key[:4], key[len(key)-4:])
				return nil
			}

			// Fall back to config file
			c, cerr := config.Load(cfgPath)
			if cerr == nil && c.DocsAPIKey != "" {
				key = c.DocsAPIKey
				fmt.Fprintf(cmd.OutOrStdout(), "Authenticated (key: %s...%s)\n", key[:4], key[len(key)-4:])
				return nil
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Not authenticated. Run: hs docs auth login")
			return nil
		},
	}
}

func docsAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored Docs API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			auth.DeleteDocsAPIKey()

			// Also clear from config file
			c, err := config.Load(cfgPath)
			if err == nil && c.DocsAPIKey != "" {
				c.DocsAPIKey = ""
				_ = config.Save(cfgPath, c)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Docs API key removed.")
			return nil
		},
	}
}
