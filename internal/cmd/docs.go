package cmd

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(newDocsCmd())
}

func newDocsCmd() *cobra.Command {
	docsCmd := &cobra.Command{
		Use:   "docs",
		Short: "Manage HelpScout Docs API resources",
	}

	docsCmd.AddCommand(
		newDocsAuthCmd(),
		newDocsCollectionsCmd(),
		newDocsCategoriesCmd(),
		newDocsArticlesCmd(),
		newDocsSitesCmd(),
		newDocsRedirectsCmd(),
		newDocsAssetsCmd(),
	)

	return docsCmd
}
