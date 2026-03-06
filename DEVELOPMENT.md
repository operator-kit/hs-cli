# Developer guide

## Prerequisites

- Go 1.25+

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
# All unit tests
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

## Test architecture

- **api package**: Uses `httptest.Server` with a URL-rewriting transport to test real HTTP round-trips without hitting the HelpScout API
- **cmd package**: Uses a `mockClient` implementing `ClientAPI` with function fields. Global state (`apiClient`, `cfg`, `output.Out`) is swapped per-test. Not `t.Parallel()` safe due to global mutation. E2E tests use `isolateHome` to sandbox HOME/config dirs
- **config package**: Uses `t.TempDir()` for filesystem tests and `t.Setenv()` for env var tests
- **output package**: Formatters write to `bytes.Buffer` for assertion
- **selfupdate package**: Uses `httptest.Server` for GitHub API mocking, `DirOverride`/`InstallDirOverride` for filesystem isolation

## Release

Releases are automated via GitHub Actions. Push a `v*` tag to trigger a draft release with platform binaries:

```bash
git tag v0.1.0
git push origin v0.1.0
```
