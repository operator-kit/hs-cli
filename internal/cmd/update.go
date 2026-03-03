package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/selfupdate"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update hs to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionStr == "dev" {
			fmt.Fprintln(cmd.ErrOrStderr(), "Skipping update: running dev build")
			return nil
		}

		release, err := selfupdate.FetchLatestRelease()
		if err != nil {
			return fmt.Errorf("check for update: %w", err)
		}
		if release == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "No published release found.")
			return nil
		}

		latest := strings.TrimPrefix(release.TagName, "v")
		if selfupdate.CompareVersions(versionStr, latest) >= 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Already up to date (v%s).\n", versionStr)
			return nil
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Updating v%s → v%s\n", versionStr, latest)
		if err := selfupdate.Update(release, cmd.ErrOrStderr()); err != nil {
			return fmt.Errorf("update: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated to v%s.\n", latest)
		return nil
	},
}
