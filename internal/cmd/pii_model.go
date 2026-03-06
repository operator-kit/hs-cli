package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/pii/ner"
)

func init() {
	rootCmd.AddCommand(newPIIModelCmd())
}

func newPIIModelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pii-model",
		Short: "Manage PII redaction model",
	}
	cmd.AddCommand(piiModelInstallCmd(), piiModelUninstallCmd(), piiModelStatusCmd())
	return cmd
}

func piiModelInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Download PII redaction model for the current platform",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ner.IsModelReady() {
				fmt.Fprintln(cmd.OutOrStdout(), "PII model already installed.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Downloading PII model v%s...\n", ner.ModelVersion)

			_, err := ner.EnsureModel(func(read, total int64) {
				if total > 0 {
					pct := float64(read) / float64(total) * 100
					fmt.Fprintf(cmd.ErrOrStderr(), "\r  %.0f%% (%d / %d MB)", pct, read/1024/1024, total/1024/1024)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "\r  %d MB downloaded", read/1024/1024)
				}
			})
			fmt.Fprintln(cmd.ErrOrStderr()) // newline after progress
			if err != nil {
				return fmt.Errorf("install failed: %w", err)
			}

			dir, _ := ner.CacheDir()
			fmt.Fprintf(cmd.OutOrStdout(), "Model installed to %s\n", dir)
			return nil
		},
	}
}

func piiModelStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show PII model installation status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !ner.IsModelReady() {
				fmt.Fprintln(cmd.OutOrStdout(), "PII model: not installed")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'hs pii-model install' to download.")
				return nil
			}

			dir, _ := ner.CacheDir()
			fmt.Fprintf(cmd.OutOrStdout(), "PII model: installed (v%s)\n", ner.ModelVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Location: %s\n", dir)
			fmt.Fprintln(cmd.OutOrStdout(), "Model: distilbert-base-multilingual-cased-ner-hrl (INT8)")
			fmt.Fprintln(cmd.OutOrStdout(), "Languages: Arabic, German, English, Spanish, French, Italian, Latvian, Dutch, Portuguese, Chinese")
			return nil
		},
	}
}

func piiModelUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove cached PII model files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !ner.IsModelReady() {
				fmt.Fprintln(cmd.OutOrStdout(), "PII model is not installed.")
				return nil
			}

			dir, _ := ner.CacheDir()
			if err := ner.RemoveModel(); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed PII model from %s\n", dir)
			return nil
		},
	}
}

