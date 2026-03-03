package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/auth"
)

func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	authCmd.AddCommand(authLoginCmd(), authStatusCmd(), authLogoutCmd())
	return authCmd
}

func authLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with HelpScout Inbox API credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			fmt.Print("App ID: ")
			appID, _ := reader.ReadString('\n')
			appID = strings.TrimSpace(appID)

			fmt.Print("App Secret: ")
			appSecret, _ := reader.ReadString('\n')
			appSecret = strings.TrimSpace(appSecret)

			if appID == "" || appSecret == "" {
				return fmt.Errorf("app ID and secret are required")
			}

			// Validate by fetching a token and testing with /mailboxes
			fmt.Print("Validating credentials... ")
			client := api.New(context.Background(), appID, appSecret, debug)
			data, err := client.ListMailboxes(context.Background(), nil)
			if err != nil {
				fmt.Println("failed")
				return fmt.Errorf("authentication failed: %w", err)
			}

			// Count mailboxes
			var resp struct {
				Page struct {
					TotalElements int `json:"totalElements"`
				} `json:"page"`
			}
			json.Unmarshal(data, &resp)

			// Store in keyring
			if err := auth.StoreInboxCredentials(appID, appSecret); err != nil {
				return fmt.Errorf("storing credentials: %w", err)
			}

			fmt.Printf("Authenticated. Found %d mailboxes.\n", resp.Page.TotalElements)
			return nil
		},
	}
}

func authStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _, err := auth.LoadInboxCredentials()
			if err != nil || id == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Not authenticated. Run: hs inbox auth login")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Authenticated (app: %s...%s)\n", id[:4], id[len(id)-4:])
			return nil
		},
	}
}

func authLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			auth.DeleteInboxCredentials()
			fmt.Fprintln(cmd.OutOrStdout(), "Credentials removed.")
			return nil
		},
	}
}
