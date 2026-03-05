package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/pii/ner"
)

func init() {
	rootCmd.AddCommand(newNERCmd())
}

func newNERCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ner",
		Short: "Manage NER model for PII name detection",
	}
	cmd.AddCommand(nerInstallCmd(), nerStatusCmd(), nerRemoveCmd())
	return cmd
}

func nerInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Download NER model bundle for the current platform",
		RunE: func(cmd *cobra.Command, args []string) error {
			if ner.IsModelReady() {
				fmt.Fprintln(cmd.OutOrStdout(), "NER model already installed.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Downloading NER model v%s...\n", ner.ModelVersion)

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

func nerStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show NER model installation status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !ner.IsModelReady() {
				fmt.Fprintln(cmd.OutOrStdout(), "NER model: not installed")
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'hs ner install' to download.")
				return nil
			}

			dir, _ := ner.CacheDir()
			fmt.Fprintf(cmd.OutOrStdout(), "NER model: installed (v%s)\n", ner.ModelVersion)
			fmt.Fprintf(cmd.OutOrStdout(), "Location: %s\n", dir)
			fmt.Fprintln(cmd.OutOrStdout(), "Model: distilbert-base-multilingual-cased-ner-hrl (INT8)")
			fmt.Fprintln(cmd.OutOrStdout(), "Languages: Arabic, German, English, Spanish, French, Italian, Latvian, Dutch, Portuguese, Chinese")
			return nil
		},
	}
}

func nerRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove",
		Short: "Delete cached NER model files",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !ner.IsModelReady() {
				fmt.Fprintln(cmd.OutOrStdout(), "NER model is not installed.")
				return nil
			}

			dir, _ := ner.CacheDir()
			if err := ner.RemoveModel(); err != nil {
				return fmt.Errorf("remove failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed NER model from %s\n", dir)
			return nil
		},
	}
}

