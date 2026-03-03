package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/permission"
)

func newPermissionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "permissions",
		Short: "Show current permission policy and per-command access",
		RunE: func(cmd *cobra.Command, args []string) error {
			raw := ""
			source := "none (unrestricted)"
			if v := os.Getenv("HS_INBOX_PERMISSIONS"); v != "" {
				raw = v
				source = "env: HS_INBOX_PERMISSIONS"
			} else if cfg != nil && cfg.InboxPermissions != "" {
				raw = cfg.InboxPermissions
				source = "config: inbox_permissions"
			}

			policy, err := permission.Parse(raw)
			if err != nil {
				return fmt.Errorf("invalid permissions: %w", err)
			}

			fmt.Fprintf(output.Out, "Source: %s\n", source)
			fmt.Fprintf(output.Out, "Policy: %s\n\n", policy)

			if policy.IsUnrestricted() {
				fmt.Fprintln(output.Out, "All commands are allowed (no restrictions).")
				return nil
			}

			// Walk the inbox command tree and collect annotated leaf commands.
			inboxCmd := findInboxCmd(cmd.Root())
			if inboxCmd == nil {
				return fmt.Errorf("inbox command not found")
			}

			type entry struct {
				path     string
				resource string
				op       string
				allowed  bool
			}
			var entries []entry
			walkCommands(inboxCmd, "", func(c *cobra.Command, path string) {
				res, ok := c.Annotations[permission.AnnotationResource]
				if !ok {
					return
				}
				op := c.Annotations[permission.AnnotationOperation]
				entries = append(entries, entry{
					path:     path,
					resource: res,
					op:       op,
					allowed:  policy.Allows(res, op),
				})
			})

			sort.Slice(entries, func(i, j int) bool {
				return entries[i].path < entries[j].path
			})

			cols := []string{"command", "resource", "operation", "access"}
			rows := make([]map[string]string, len(entries))
			for i, e := range entries {
				access := "DENY"
				if e.allowed {
					access = "ALLOW"
				}
				rows[i] = map[string]string{
					"command":   e.path,
					"resource":  e.resource,
					"operation": e.op,
					"access":    access,
				}
			}
			return output.Print(getFormat(), cols, rows)
		},
	}
}

func findInboxCmd(root *cobra.Command) *cobra.Command {
	for _, c := range root.Commands() {
		if c.Name() == "inbox" {
			return c
		}
	}
	return nil
}

func walkCommands(cmd *cobra.Command, prefix string, fn func(*cobra.Command, string)) {
	path := prefix
	if prefix != "" {
		path = prefix + " " + cmd.Name()
	} else {
		path = cmd.Name()
	}

	// Leaf command (has RunE or Run)
	if cmd.RunE != nil || cmd.Run != nil {
		fn(cmd, path)
	}

	for _, sub := range cmd.Commands() {
		walkCommands(sub, path, fn)
	}
}
