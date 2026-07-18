package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/operator-kit/hs-cli/internal/config"
	"github.com/operator-kit/hs-cli/internal/output"
	"github.com/operator-kit/hs-cli/internal/selfupdate"
)

// setRootArgs lets command tests describe ordinary CLI inputs while exercising
// the same protected transport required from real callers. Values never enter
// the argv passed to Cobra when their flag is annotated as protected.
func setRootArgs(t *testing.T, args []string) {
	t.Helper()
	resetChangedFlags(rootCmd)
	protectedInputPath = ""

	command := testCommandForArgs(rootCmd, args)
	protected := make(map[string]any)
	safeArgs := make([]string, 0, len(args)+2)
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			safeArgs = append(safeArgs, arg)
			continue
		}

		nameValue := strings.TrimPrefix(arg, "--")
		name, value, inline := strings.Cut(nameValue, "=")
		flag := lookupCommandFlag(command, name)
		if flag == nil || flag.Annotations == nil {
			safeArgs = append(safeArgs, arg)
			continue
		}
		if _, isProtected := flag.Annotations[protectedFlagAnnotation]; !isProtected {
			safeArgs = append(safeArgs, arg)
			continue
		}
		if !inline {
			if index+1 >= len(args) {
				t.Fatalf("test protected flag --%s has no value", name)
			}
			index++
			value = args[index]
		}

		switch flag.Value.Type() {
		case "stringSlice":
			values, _ := protected[name].([]string)
			values = append(values, strings.Split(value, ",")...)
			protected[name] = values
		case "stringArray":
			values, _ := protected[name].([]string)
			protected[name] = append(values, value)
		default:
			protected[name] = value
		}
	}

	if len(protected) > 0 {
		envelope := protectedInputEnvelope{
			Schema:  protectedInputSchema,
			Command: commandPathSegments(command),
			Flags:   make(map[string]json.RawMessage, len(protected)),
		}
		for name, value := range protected {
			raw, err := json.Marshal(value)
			require.NoError(t, err)
			envelope.Flags[name] = raw
		}
		raw, err := json.Marshal(envelope)
		require.NoError(t, err)
		rootCmd.SetIn(bytes.NewReader(raw))
		safeArgs = append([]string{"--" + protectedInputFlagName, "-"}, safeArgs...)
	}
	rootCmd.SetArgs(safeArgs)
}

func testCommandForArgs(root *cobra.Command, args []string) *cobra.Command {
	current := root
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		for _, child := range current.Commands() {
			if child.Name() == arg || child.HasAlias(arg) {
				current = child
				break
			}
		}
	}
	return current
}

func resetChangedFlags(command *cobra.Command) {
	command.Flags().VisitAll(func(flag *pflag.Flag) { flag.Changed = false })
	command.PersistentFlags().VisitAll(func(flag *pflag.Flag) { flag.Changed = false })
	for _, child := range command.Commands() {
		resetChangedFlags(child)
	}
}

// isolateHome creates a sandboxed home directory so E2E tests don't touch
// the real config, keyring, or shell rc files.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()

	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	// Go os.UserConfigDir() isolation
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))

	return home
}

// saveRestore captures and restores global state for E2E tests.
func saveRestore(t *testing.T) {
	t.Helper()
	origCfg := cfg
	origCfgPath := cfgPath
	origApiClient := apiClient
	origFormat := format
	origUnredacted := unredacted
	origNoPaginate := noPaginate
	origPage := page
	origPerPage := perPage
	origDebug := debug
	origVersionStr := versionStr
	origUpdateDir := selfupdate.DirOverride
	origUpdateResult := updateResult
	origSetClientID := setInboxAppID
	origSetClientSecret := setInboxAppSecret
	origSetDefaultMailbox := setInboxMailbox
	origSetFormat := setFormat
	origSetPIIMode := setInboxPIIMode
	origSetPIIAllowRaw := setInboxPIIAllow
	origResolvePIIContext := resolvePIIContext
	origInvocationPIIPrepared := invocationPIIPrepared
	origInvocationPIIMode := invocationPIIMode
	origInvocationPIIContext := invocationPIIContext
	origInvocationProtectedValues := append([]string(nil), invocationProtectedValues...)

	selfupdate.DirOverride = t.TempDir()
	versionStr = "dev"
	resetPIIInvocation()

	t.Cleanup(func() {
		cfg = origCfg
		cfgPath = origCfgPath
		apiClient = origApiClient
		format = origFormat
		unredacted = origUnredacted
		noPaginate = origNoPaginate
		page = origPage
		perPage = origPerPage
		debug = origDebug
		versionStr = origVersionStr
		selfupdate.DirOverride = origUpdateDir
		updateResult = origUpdateResult
		setInboxAppID = origSetClientID
		setInboxAppSecret = origSetClientSecret
		setInboxMailbox = origSetDefaultMailbox
		setFormat = origSetFormat
		setInboxPIIMode = origSetPIIMode
		setInboxPIIAllow = origSetPIIAllowRaw
		resolvePIIContext = origResolvePIIContext
		invocationPIIPrepared = origInvocationPIIPrepared
		invocationPIIMode = origInvocationPIIMode
		invocationPIIContext = origInvocationPIIContext
		invocationProtectedValues = origInvocationProtectedValues
		configSetCmd.Flags().VisitAll(func(f *pflag.Flag) {
			f.Changed = false
		})
	})
}

// setupE2E combines isolateHome + saveRestore + config for E2E auth tests.
// Credentials are set via config file (not keyring) for test isolation.
func setupE2E(t *testing.T) (home string, buf *bytes.Buffer) {
	t.Helper()
	home = isolateHome(t)
	saveRestore(t)
	useTestPIISecretResolver()

	// Prevent update check from hitting the network
	versionStr = "dev"

	cfgDir := filepath.Join(home, ".config", "hs")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	cfgFile := filepath.Join(cfgDir, "config.yaml")
	cfgPath = cfgFile

	buf = new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	return home, buf
}

func TestVersionCmd(t *testing.T) {
	saveRestore(t)
	SetVersion("1.0.0", "abc123", "2024-01-01")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	setRootArgs(t, []string{"version"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "1.0.0")
	assert.Contains(t, buf.String(), "abc123")
}

func TestUpdateCmd_DevBuild(t *testing.T) {
	saveRestore(t)
	versionStr = "dev"

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	setRootArgs(t, []string{"update"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Skipping update: running dev build")
}

// --- E2E auth tests using config-file credentials ---

func TestAuthStatus_NotAuthenticated_E2E(t *testing.T) {
	_, buf := setupE2E(t)

	// No credentials anywhere
	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")

	setRootArgs(t, []string{"inbox", "auth", "status"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Not authenticated")
}

func TestAuthStatus_WithConfigCreds_E2E(t *testing.T) {
	home, buf := setupE2E(t)

	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")

	// Write config with credentials — status now falls back to config when keyring unavailable
	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:     "test-id-1234abcd",
		InboxAppSecret: "test-secret",
	}))
	cfgPath = cfgFile

	setRootArgs(t, []string{"inbox", "auth", "status"})
	require.NoError(t, rootCmd.Execute())
	assert.Contains(t, buf.String(), "Authenticated")
}

func TestAuthLogout_E2E(t *testing.T) {
	_, buf := setupE2E(t)

	setRootArgs(t, []string{"inbox", "auth", "logout"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Credentials removed")
}

func TestConfigCredentialPath_E2E(t *testing.T) {
	home, _ := setupE2E(t)

	// Set credentials via env vars (highest priority)
	t.Setenv("HS_INBOX_APP_ID", "env-id-test")
	t.Setenv("HS_INBOX_APP_SECRET", "env-secret-test")

	// Config should pick up env vars
	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "env-id-test", loaded.InboxAppID)
	assert.Equal(t, "env-secret-test", loaded.InboxAppSecret)
}

func TestConfigFile_E2E(t *testing.T) {
	home, _ := setupE2E(t)

	// Write config
	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:          "file-id",
		InboxAppSecret:      "file-secret",
		InboxDefaultMailbox: 12345,
		Format:              "json",
	}))

	// Clear env vars so config file is used
	t.Setenv("HS_INBOX_APP_ID", "")
	t.Setenv("HS_INBOX_APP_SECRET", "")
	t.Setenv("HS_FORMAT", "")

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "file-id", loaded.InboxAppID)
	assert.Equal(t, "file-secret", loaded.InboxAppSecret)
	assert.Equal(t, 12345, loaded.InboxDefaultMailbox)
	assert.Equal(t, "json", loaded.Format)
}

func TestEnvOverridesConfig_E2E(t *testing.T) {
	home, _ := setupE2E(t)

	// Write config with one set of creds
	cfgFile := filepath.Join(home, ".config", "hs", "config.yaml")
	require.NoError(t, config.Save(cfgFile, &config.Config{
		InboxAppID:     "file-id",
		InboxAppSecret: "file-secret",
		Format:         "table",
	}))

	// Env vars override
	t.Setenv("HS_INBOX_APP_ID", "env-id")
	t.Setenv("HS_INBOX_APP_SECRET", "env-secret")
	t.Setenv("HS_FORMAT", "json")

	loaded, err := config.Load(cfgFile)
	require.NoError(t, err)
	assert.Equal(t, "env-id", loaded.InboxAppID)
	assert.Equal(t, "env-secret", loaded.InboxAppSecret)
	assert.Equal(t, "json", loaded.Format)
}

// TestCommandWithEnvCreds_E2E verifies that commands work with env-var credentials.
func TestCommandWithEnvCreds_E2E(t *testing.T) {
	_, buf := setupE2E(t)

	// Set up a mock client to avoid real API calls
	mock := &mockClient{
		ListMailboxesFn: func(ctx context.Context, params url.Values) (json.RawMessage, error) {
			return halJSON("mailboxes", `[{"id":1,"name":"Support","email":"support@test.com","slug":"support"}]`), nil
		},
	}
	apiClient = mock
	output.Out = buf
	format = "table"
	t.Cleanup(func() { output.Out = os.Stdout })

	setRootArgs(t, []string{"inbox", "mailboxes", "list"})
	require.NoError(t, rootCmd.Execute())

	assert.Contains(t, buf.String(), "Support")
}
