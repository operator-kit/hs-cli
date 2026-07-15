package cmd

import (
	"fmt"
	"io"
	"path/filepath"

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
			status := ner.Status()
			if status.State == ner.ModelUnsupported {
				return fmt.Errorf("cannot install PII model: %s", status.Reason)
			}
			if status.Usable() {
				fmt.Fprintln(cmd.OutOrStdout(), "PII model already installed.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Downloading PII model v%s...\n", ner.ModelVersion)

			paths, err := ner.EnsureModelContext(cmd.Context(), func(read, total int64) {
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

			fmt.Fprintf(cmd.OutOrStdout(), "Model installed to %s\n", filepath.Dir(paths.ModelONNX))
			return nil
		},
	}
}

func piiModelStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show PII model installation status",
		RunE: func(cmd *cobra.Command, args []string) error {
			writePIIModelStatus(cmd.OutOrStdout(), ner.Status())
			return nil
		},
	}
}

func writePIIModelStatus(out io.Writer, status ner.ModelStatus) {
	switch status.State {
	case ner.ModelUnsupported:
		fmt.Fprintf(out, "PII model: unsupported on %s\n", status.Platform.Key())
		fmt.Fprintln(out, status.Reason)
		return
	case ner.ModelAbsent:
		fmt.Fprintln(out, "PII model: not installed")
		fmt.Fprintln(out, "Run 'hs pii-model install' to download.")
		return
	case ner.ModelCorrupt:
		fmt.Fprintln(out, "PII model: corrupt or incomplete")
		fmt.Fprintln(out, status.Reason)
		fmt.Fprintln(out, "Run 'hs pii-model install' to replace it.")
		return
	case ner.ModelInstalledUnverified:
		fmt.Fprintf(out, "PII model: installed, unverified legacy bundle (v%s)\n", ner.ModelVersion)
		fmt.Fprintln(out, "The files predate trusted-manifest verification and will not be loaded.")
	case ner.ModelReady:
		fmt.Fprintf(out, "PII model: installed and verified (v%s)\n", ner.ModelVersion)
	}

	fmt.Fprintf(out, "Location: %s\n", status.Dir)
	fmt.Fprintln(out, "Model: distilbert-base-multilingual-cased-ner-hrl (INT8)")
	fmt.Fprintln(out, "Languages: Arabic, German, English, Spanish, French, Italian, Latvian, Dutch, Portuguese, Chinese")
}

func piiModelUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove cached PII model files",
		RunE: func(cmd *cobra.Command, args []string) error {
			status := ner.Status()
			if !status.Present {
				fmt.Fprintln(cmd.OutOrStdout(), "PII model is not installed.")
				return nil
			}

			if err := ner.RemoveModel(); err != nil {
				return fmt.Errorf("uninstall failed: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed PII model from %s\n", status.Dir)
			return nil
		},
	}
}
