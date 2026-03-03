package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/config"
	"github.com/operator-kit/hs-cli/internal/pii"
)

var (
	setInboxAppID      string
	setInboxAppSecret  string
	setInboxMailbox    int
	setInboxPIIMode    string
	setInboxPIIAllow   bool
	setDocsAPIKey      string
	setDocsPermissions string
	setFormat          string

	configCmd    *cobra.Command
	configSetCmd *cobra.Command
)

func init() {
	configCmd = newConfigCmd()
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify configuration",
	}
	configSetCmd = newConfigSetCmd()
	cmd.AddCommand(configSetCmd, newConfigGetCmd(), newConfigPathCmd())
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set config values",
		RunE:  runConfigSet,
	}
	cmd.Flags().StringVar(&setInboxAppID, "inbox-app-id", "", "HelpScout Inbox App ID")
	cmd.Flags().StringVar(&setInboxAppSecret, "inbox-app-secret", "", "HelpScout Inbox App Secret")
	cmd.Flags().IntVar(&setInboxMailbox, "inbox-default-mailbox", 0, "default mailbox ID")
	cmd.Flags().StringVar(&setFormat, "format", "", "output format: table|json|csv")
	cmd.Flags().StringVar(&setInboxPIIMode, "inbox-pii-mode", "", "PII redaction mode: off|customers|all")
	cmd.Flags().BoolVar(&setInboxPIIAllow, "inbox-pii-allow-unredacted", false, "allow per-request --unredacted override")
	cmd.Flags().StringVar(&setDocsAPIKey, "docs-api-key", "", "HelpScout Docs API key")
	cmd.Flags().StringVar(&setDocsPermissions, "docs-permissions", "", "Docs permission policy")
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Print config values",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConfigGet,
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print config file location",
		RunE:  runConfigPath,
	}
}

func configFilePath() string {
	if cfgPath != "" {
		return cfgPath
	}
	return config.DefaultPath()
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	path := configFilePath()

	existing, err := config.Load(path)
	if err != nil {
		existing = &config.Config{}
	}

	if cmd.Flags().Changed("inbox-app-id") {
		existing.InboxAppID = setInboxAppID
	}
	if cmd.Flags().Changed("inbox-app-secret") {
		existing.InboxAppSecret = setInboxAppSecret
	}
	if cmd.Flags().Changed("inbox-default-mailbox") {
		existing.InboxDefaultMailbox = setInboxMailbox
	}
	if cmd.Flags().Changed("format") {
		existing.Format = setFormat
	}
	if cmd.Flags().Changed("inbox-pii-mode") {
		if !pii.IsValidMode(setInboxPIIMode) {
			return fmt.Errorf("invalid --inbox-pii-mode: %q (expected off|customers|all)", setInboxPIIMode)
		}
		existing.InboxPIIMode = setInboxPIIMode
	}
	if cmd.Flags().Changed("inbox-pii-allow-unredacted") {
		existing.InboxPIIAllowUnredacted = setInboxPIIAllow
	}
	if cmd.Flags().Changed("docs-api-key") {
		existing.DocsAPIKey = setDocsAPIKey
	}
	if cmd.Flags().Changed("docs-permissions") {
		existing.DocsPermissions = setDocsPermissions
	}

	if err := config.Save(path, existing); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Config saved to %s\n", path)
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	path := configFilePath()

	c, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	out := cmd.OutOrStdout()

	if len(args) == 0 {
		fmt.Fprintf(out, "inbox-app-id: %s\n", c.InboxAppID)
		fmt.Fprintf(out, "inbox-app-secret: %s\n", c.InboxAppSecret)
		fmt.Fprintf(out, "inbox-default-mailbox: %d\n", c.InboxDefaultMailbox)
		fmt.Fprintf(out, "format: %s\n", c.Format)
		fmt.Fprintf(out, "inbox-pii-mode: %s\n", c.InboxPIIMode)
		fmt.Fprintf(out, "inbox-pii-allow-unredacted: %t\n", c.InboxPIIAllowUnredacted)
		fmt.Fprintf(out, "docs-api-key: %s\n", c.DocsAPIKey)
		fmt.Fprintf(out, "docs-permissions: %s\n", c.DocsPermissions)
		return nil
	}

	switch args[0] {
	case "inbox-app-id":
		fmt.Fprintf(out, "%s\n", c.InboxAppID)
	case "inbox-app-secret":
		fmt.Fprintf(out, "%s\n", c.InboxAppSecret)
	case "inbox-default-mailbox":
		fmt.Fprintf(out, "%d\n", c.InboxDefaultMailbox)
	case "format":
		fmt.Fprintf(out, "%s\n", c.Format)
	case "inbox-pii-mode":
		fmt.Fprintf(out, "%s\n", c.InboxPIIMode)
	case "inbox-pii-allow-unredacted":
		fmt.Fprintf(out, "%t\n", c.InboxPIIAllowUnredacted)
	case "docs-api-key":
		fmt.Fprintf(out, "%s\n", c.DocsAPIKey)
	case "docs-permissions":
		fmt.Fprintf(out, "%s\n", c.DocsPermissions)
	default:
		return fmt.Errorf("unknown config key: %s", args[0])
	}
	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), configFilePath())
	return nil
}
