package cmd

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(newInboxCmd())
}

func newInboxCmd() *cobra.Command {
	inboxCmd := &cobra.Command{
		Use:   "inbox",
		Short: "Manage HelpScout Inbox API resources",
	}
	inboxCmd.PersistentFlags().BoolVar(&unredacted, "unredacted", false, "disable PII redaction for this command (requires pii_allow_unredacted when redaction is enabled)")

	inboxCmd.AddCommand(
		configCmd,
		newAuthCmd(),
		newMailboxesCmd(),
		newConversationsCmd(),
		newCustomersCmd(),
		newTagsCmd(),
		newUsersCmd(),
		newTeamsCmd(),
		newOrganizationsCmd(),
		newPropertiesCmd(),
		newRatingsCmd(),
		newReportsCmd(),
		newWorkflowsCmd(),
		newWebhooksCmd(),
		newSavedRepliesCmd(),
		newPermissionsCmd(),
		newToolsCmd(),
	)

	return inboxCmd
}
