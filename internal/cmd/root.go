package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/operator-kit/hs-cli/internal/api"
	"github.com/operator-kit/hs-cli/internal/auth"
	"github.com/operator-kit/hs-cli/internal/config"
	"github.com/operator-kit/hs-cli/internal/permission"
	"github.com/operator-kit/hs-cli/internal/selfupdate"
)

var (
	cfgPath    string
	format     string
	unredacted bool
	noPaginate bool
	page       int
	perPage    int
	debug      bool

	cfg        *config.Config
	apiClient  api.ClientAPI
	docsClient api.DocsClientAPI

	versionStr string
	commitStr  string
	dateStr    string

	updateResult chan string
)

func SetVersion(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

var rootCmd = &cobra.Command{
	Use:   "hs",
	Short: "HelpScout CLI — manage mailboxes, conversations, customers and more",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip everything for config subcommands
		for c := cmd; c != nil; c = c.Parent() {
			if c == configCmd {
				return nil
			}
		}

		startUpdateCheck(cmd)

		var err error
		cfg, err = config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		// Flag overrides config
		if cmd.Flags().Changed("format") {
			cfg.Format = format
		} else if format == "" {
			format = cfg.Format
		}

		// Skip client init for command groups that only organize subcommands.
		if cmd.RunE == nil && cmd.Run == nil {
			return nil
		}

		// Determine if command is under docs subtree
		isDocs := isUnderSubtree(cmd, "docs")

		// Permission check: enforce policy for annotated commands.
		if res, ok := cmd.Annotations[permission.AnnotationResource]; ok {
			op := cmd.Annotations[permission.AnnotationOperation]
			permStr := cfg.InboxPermissions
			envHint := "HS_INBOX_PERMISSIONS"
			if isDocs {
				permStr = cfg.DocsPermissions
				envHint = "HS_DOCS_PERMISSIONS"
			}
			policy, perr := permission.Parse(permStr)
			if perr != nil {
				return fmt.Errorf("invalid permissions: %w", perr)
			}
			if !policy.Allows(res, op) {
				return fmt.Errorf("permission denied: %s:%s not allowed\n\nCurrent policy: %s\nTo allow, add %s:%s to %s",
					res, op, policy, res, op, envHint)
			}
		}

		// Skip client init for auth/version/completion/update commands
		name := cmd.Name()
		parent := ""
		if cmd.Parent() != nil {
			parent = cmd.Parent().Name()
		}
		if name == "login" || name == "logout" || name == "status" ||
			name == "version" || name == "completion" || name == "update" ||
			name == "permissions" || name == "mcp" ||
			parent == "completion" || parent == "ner" ||
			name == "hs" || name == "inbox" || name == "docs" || name == "ner" {
			return nil
		}

		// Docs API client init
		if isDocs {
			if docsClient != nil {
				return nil
			}
			docsAPIKey := cfg.DocsAPIKey
			if docsAPIKey == "" {
				key, kerr := auth.LoadDocsAPIKey()
				if kerr == nil {
					docsAPIKey = key
				}
			}
			if docsAPIKey == "" {
				return fmt.Errorf(
					"not authenticated for Docs API.\n" +
						"Set HS_DOCS_API_KEY in your environment (or MCP server env).\n" +
						"Interactive setup: hs docs auth login",
				)
			}
			docsClient = api.NewDocs(docsAPIKey, debug)
			return nil
		}

		// Skip client init if already set (e.g. by tests)
		if apiClient != nil {
			return nil
		}

		// Resolve credentials: env > keyring > config
		appID := cfg.InboxAppID
		appSecret := cfg.InboxAppSecret
		if appID == "" || appSecret == "" {
			id, secret, kerr := auth.LoadInboxCredentials()
			if kerr == nil {
				appID = id
				appSecret = secret
			}
		}
		if appID == "" || appSecret == "" {
			return fmt.Errorf(
				"not authenticated.\n" +
					"Set HS_INBOX_APP_ID and HS_INBOX_APP_SECRET in your environment (or MCP server env).\n" +
					"Interactive setup: hs inbox auth login\n" +
					"npx fallback: npx -y @operatorkit/hs inbox auth login",
			)
		}

		apiClient = api.New(context.Background(), appID, appSecret, debug)
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if updateResult == nil {
			return nil
		}
		select {
		case latest := <-updateResult:
			if latest != "" {
				fmt.Fprintf(os.Stderr, "\nA new version of hs is available: v%s (current: v%s)\nRun 'hs update' to upgrade.\n", latest, versionStr)
			}
		default:
			// goroutine hasn't finished, skip silently
		}
		return nil
	},
}

func startUpdateCheck(cmd *cobra.Command) {
	if os.Getenv("HS_NO_UPDATE_CHECK") == "1" {
		return
	}
	if versionStr == "dev" {
		return
	}
	name := cmd.Name()
	if name == "version" || name == "completion" || name == "update" || name == "mcp" {
		return
	}
	if !selfupdate.ShouldCheck(versionStr) {
		return
	}
	updateResult = make(chan string, 1)
	go func() {
		updateResult <- selfupdate.CheckForUpdate(versionStr)
	}()
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path")
	rootCmd.PersistentFlags().StringVar(&format, "format", "", "output format: table|json|json-full|csv")
	rootCmd.PersistentFlags().BoolVar(&noPaginate, "no-paginate", false, "fetch all pages")
	rootCmd.PersistentFlags().IntVar(&page, "page", 1, "page number")
	rootCmd.PersistentFlags().IntVar(&perPage, "per-page", 25, "results per page")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "show HTTP debug output")

	rootCmd.AddCommand(updateCmd)
}

func Execute() error {
	executedCmd, err := rootCmd.ExecuteC()
	if err == nil {
		return nil
	}

	if executedCmd == nil {
		executedCmd = rootCmd
	}

	fmt.Fprintf(executedCmd.ErrOrStderr(), "Error: %v\n", err)
	if shouldShowUsageForError(err) {
		fmt.Fprintln(executedCmd.ErrOrStderr())
		_ = executedCmd.Usage()
	}

	return err
}

func getFormat() string {
	if format != "" {
		return format
	}
	if cfg != nil && cfg.Format != "" {
		return cfg.Format
	}
	return "table"
}

// isJSON returns true for both "json" (clean) and "json-full" (raw) formats.
func isJSON() bool {
	f := getFormat()
	return f == "json" || f == "json-full"
}

// isJSONClean returns true only for the cleaned "json" format.
func isJSONClean() bool {
	return getFormat() == "json"
}

// isUnderSubtree checks if cmd is nested under a parent with the given name.
func isUnderSubtree(cmd *cobra.Command, name string) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == name {
			return true
		}
	}
	return false
}

func shouldShowUsageForError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "unknown command"):
		return true
	case strings.Contains(msg, "unknown shorthand flag"):
		return true
	case strings.Contains(msg, "unknown flag"):
		return true
	case strings.Contains(msg, "required flag"):
		return true
	case strings.Contains(msg, "accepts ") && strings.Contains(msg, " arg(s), received "):
		return true
	case strings.Contains(msg, "requires at least ") && strings.Contains(msg, " arg(s)"):
		return true
	case strings.Contains(msg, "requires at most ") && strings.Contains(msg, " arg(s)"):
		return true
	case strings.HasPrefix(msg, "invalid argument ") && strings.Contains(msg, " for "):
		return true
	default:
		return false
	}
}
