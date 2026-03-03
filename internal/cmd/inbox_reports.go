package cmd

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
)

var reportFamilies = map[string]string{
	"chats":         "chat",
	"company":       "company",
	"conversations": "conversations",
	"customers":     "customers",
	"docs":          "docs",
	"email":         "email",
	"productivity":  "productivity",
	"ratings":       "happiness/ratings",
	"users":         "user",
}

func newReportsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reports",
		Short: "Read Help Scout reports",
	}

	for name, path := range reportFamilies {
		familyCmd := newReportFamilyCmd(name, path)
		permission.Annotate(familyCmd, "reports", permission.OpRead)
		cmd.AddCommand(familyCmd)
	}
	return cmd
}

func newReportFamilyCmd(name string, path string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Get " + name + " report",
		RunE: func(cmd *cobra.Command, args []string) error {
			params := url.Values{}
			if v, _ := cmd.Flags().GetString("start"); v != "" {
				params.Set("start", v)
			}
			if v, _ := cmd.Flags().GetString("end"); v != "" {
				params.Set("end", v)
			}
			if v, _ := cmd.Flags().GetString("mailbox"); v != "" {
				params.Set("mailbox", v)
			}
			if v, _ := cmd.Flags().GetString("view"); v != "" {
				params.Set("view", v)
			}
			paramFlags, _ := cmd.Flags().GetStringSlice("param")
			for _, p := range paramFlags {
				parts := strings.SplitN(p, "=", 2)
				if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
					return fmt.Errorf("invalid --param value %q: expected key=value", p)
				}
				params.Set(strings.TrimSpace(parts[0]), parts[1])
			}

			data, err := apiClient.GetReport(context.Background(), path, params)
			if err != nil {
				return err
			}
			if isJSON() {
				return output.PrintRaw(data)
			}
			return output.Print("table", []string{"report", "summary"}, []map[string]string{{
				"report":  name,
				"summary": truncate(string(data), 120),
			}})
		},
	}
	cmd.Flags().String("start", "", "report start date/time")
	cmd.Flags().String("end", "", "report end date/time")
	cmd.Flags().String("mailbox", "", "mailbox ID filter")
	cmd.Flags().String("view", "", "report view filter")
	cmd.Flags().StringSlice("param", nil, "additional query params as key=value (repeatable)")
	return cmd
}
