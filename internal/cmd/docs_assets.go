package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
)

func newDocsAssetsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Manage Docs assets",
	}

	articleCmd := &cobra.Command{Use: "article", Short: "Article assets"}
	articleUploadCmd := docsAssetsArticleUploadCmd()
	permission.Annotate(articleUploadCmd, "assets", permission.OpWrite)
	articleUploadCmd.Flags().String("file", "", "file path to upload (required)")
	articleUploadCmd.MarkFlagRequired("file")
	articleCmd.AddCommand(articleUploadCmd)

	settingsCmd := &cobra.Command{Use: "settings", Short: "Settings assets"}
	settingsUploadCmd := docsAssetsSettingsUploadCmd()
	permission.Annotate(settingsUploadCmd, "assets", permission.OpWrite)
	settingsUploadCmd.Flags().String("file", "", "file path to upload (required)")
	settingsUploadCmd.MarkFlagRequired("file")
	settingsCmd.AddCommand(settingsUploadCmd)

	cmd.AddCommand(articleCmd, settingsCmd)
	return cmd
}

func docsAssetsArticleUploadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upload",
		Short: "Upload an article asset",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, _ := cmd.Flags().GetString("file")
			data, err := docsClient.UploadArticleSettingsAsset(context.Background(), filePath)
			if err != nil {
				return err
			}
			return output.PrintRaw(data)
		},
	}
}

func docsAssetsSettingsUploadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upload",
		Short: "Upload a settings asset",
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath, _ := cmd.Flags().GetString("file")
			data, err := docsClient.UploadSettingsAsset(context.Background(), filePath)
			if err != nil {
				return err
			}
			return output.PrintRaw(data)
		},
	}
}
