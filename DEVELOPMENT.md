# Developer guide

## Prerequisites

- Go 1.25+
- Docker Desktop or Docker Engine for recommended local unit and race-test runs

## Project structure

```
cmd/hs/main.go          Entry point, version ldflags
internal/
  api/
    client.go                   HTTP client, rate limiting, retry
    client_api.go               ClientAPI interface (for mocking)
    debug.go                    --debug transport (logs to hs-debug.log)
    hal.go                      HAL+JSON response parsing
    pagination.go               Multi-page fetching
  auth/
    auth.go                     OAuth2 client credentials
    store.go                    OS keyring storage
  cmd/
    root.go                     Root command, global flags, PersistentPreRunE
    mcp.go                      MCP command entrypoint
    mcp_server.go               Stdio MCP server + JSON-RPC handlers
    mcp_catalog.go              Dynamic tool catalog from inbox command tree
    mcp_execute.go              MCP args -> CLI argv execution bridge
    json_clean.go               Per-resource JSON cleanup (json vs json-full)
    auth.go                     login / status / logout
    config.go                   config set / get / path
    update.go                   self-update command
    mailboxes.go                mailboxes list / get
    conversations.go            conversations CRUD
    threads.go                  threads list / reply / note
    customers.go                customers CRUD
    tags.go                     tags list
    users.go                    users list / get
    workflows.go                workflows list / run / update-status
    webhooks.go                 webhooks CRUD
    tools.go                    workflow tools (briefing)
    docs.go                     docs command tree
    docs_auth.go                docs auth login / status / logout
    docs_sites.go               docs sites CRUD + restrictions
    docs_collections.go         docs collections CRUD
    docs_categories.go          docs categories CRUD + reorder
    docs_articles.go            docs articles CRUD + drafts + revisions
    docs_redirects.go           docs redirects CRUD
    docs_assets.go              docs asset uploads
    version.go                  version command
    completion.go               shell completion
  config/
    config.go                   YAML config + env overrides
  output/
    formatter.go                Formatter interface, Print/PrintRaw
    table.go                    Table output
    json.go                     JSON output
    csv.go                      CSV output
  selfupdate/
    version.go                  Semver parsing + comparison
    check.go                    GitHub release check with 24h cache
    update.go                   Download, verify, replace binary
  types/                        API response/request structs
npm/
  package.json                  npx wrapper package (@operatorkit/hs)
  bin/install.js               platform binary downloader
  bin/hs.js                    binary launcher
```

## Build

```bash
go build -o build/hs ./cmd/hs
```

With version info:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o build/helpscout ./cmd/helpscout
```

## Run tests

```bash
# All unit tests in Docker (recommended for local full-suite runs)
bash ./scripts/test-docker.sh

# Specific package in Docker
bash ./scripts/test-docker.sh ./internal/cmd

# Native all unit tests
go test ./...

# Verbose
go test -v ./...

# Specific package
go test -v ./internal/api/
go test -v ./internal/cmd/
go test -v ./internal/config/
go test -v ./internal/output/
go test -v ./internal/selfupdate/

# Integration tests (requires real API credentials)
HS_INBOX_APP_ID=xxx HS_INBOX_APP_SECRET=yyy go test -tags integration ./internal/api/
```

On Windows, use the PowerShell wrapper:

```powershell
.\scripts\test-docker.ps1
.\scripts\test-docker.ps1 ./internal/cmd -run TestConversationTags
```

The Docker wrappers set an isolated `HOME`, `USERPROFILE`, `XDG_CONFIG_HOME`,
`APPDATA`, `GOCACHE`, and `GOMODCACHE` inside the container. This keeps full
test runs from reading or deleting credentials/config from the host dev
environment while still reusing Docker volumes for Go module/build caches.
The wrappers use a digest-pinned `golang:1.25.9-bookworm` image.

### Race detector

The canonical local race suite runs in the same isolated Docker environment as
the full unit suite. Developers do not need to install or configure CGO or a C
compiler on the host:

```bash
bash ./scripts/test-race.sh
bash ./scripts/test-race.sh ./internal/pii/... -count=1
```

```powershell
.\scripts\test-race.ps1
.\scripts\test-race.ps1 ./internal/pii/... -count=1
```

CI runs this Docker suite for broad concurrency coverage and a separate native
Windows job for Windows-only files such as the PII secret-store lock. A
developer changing Windows-specific code can run that job locally when a
recent MinGW-w64 GCC is already available on `PATH`:

```powershell
.\scripts\test-race-native-windows.ps1
```

The native wrapper validates that GCC provides `libsynchronization.a`, enables
CGO, and selects the compiler only for its own process. Native Windows tooling
is optional for normal development because the authoritative Windows run is in
CI.

See the official Go documentation for the
[race detector requirements](https://go.dev/doc/articles/race_detector) and
[Go 1.25+ Windows CGO requirements](https://go.dev/wiki/MinimumRequirements).

## Test architecture

- **api package**: Uses `httptest.Server` with a URL-rewriting transport to test real HTTP round-trips without hitting the HelpScout API
- **cmd package**: Uses a `mockClient` implementing `ClientAPI` with function fields. Global state (`apiClient`, `cfg`, `output.Out`) is swapped per-test. Not `t.Parallel()` safe due to global mutation. E2E tests use `isolateHome` to sandbox HOME/config dirs
- **config package**: Uses `t.TempDir()` for filesystem tests and `t.Setenv()` for env var tests
- **output package**: Formatters write to `bytes.Buffer` for assertion
- **selfupdate package**: Uses `httptest.Server` for GitHub API mocking, `DirOverride`/`InstallDirOverride` for filesystem isolation

### PII regression architecture

Normal tests use the synthetic corpus at
`internal/pii/testdata/multilingual_privacy_corpus.json`; it contains no
production or Help Scout API data. The hermetic evaluator injects exact fixture
name spans and checks redaction, preservation, customer/staff separation, and
long-content behavior. The existing model-release smoke test loads the real
bundle and scores the same corpus for complete expected-name coverage,
unexpected person spans, chunking, and a two-minute evaluation budget:

```bash
HS_PII_MODEL_SMOKE_DIR=/path/to/extracted/bundle \
  go test ./internal/pii/ner -run TestRuntimeBundleSmoke -count=1 -v -timeout=5m
```

The model-tag workflow runs that smoke on every advertised Linux and macOS
target. Normal unit jobs skip it when `HS_PII_MODEL_SMOKE_DIR` is absent.

Command tests call `setRootArgs`, which automatically moves annotated fixture
values into the protected stdin envelope before Cobra parses argv. A regression
that intentionally proves raw-argv rejection must call `rootCmd.SetArgs`
directly and reset Cobra's changed flags first.

### Protected inputs and pseudonym keys

Use `markProtectedFlags` for any new flag that can contain PII, credentials,
authored free text, or a private local path. The root preflight accepts those
values only from the schema-1 `--protected-input` envelope. MCP transports all
string values through that channel and uses the annotation to scrub genuinely
sensitive values from child failures without erasing public IDs from output.

Enabled PII engines accept only a `pii.PseudonymContext`, never bare secret
material. The context combines the private HMAC key, explicit public key ID,
and identity schema. The setup resolver persists a versioned keyring record;
under the cross-process lock it migrates the pre-versioning 32-byte record by
preserving those exact secret bytes and generating an independent random key
ID. Never derive a public key ID from secret material: that would provide an
offline verifier. Explicit deployments must set `HS_INBOX_PII_SECRET` and
`HS_INBOX_PII_KEY_ID` together and rotate them together.

## Release

Releases are automated via GitHub Actions. Push a `v*` tag to trigger a draft release with platform binaries:

```bash
git tag v0.1.0
git push origin v0.1.0
```
