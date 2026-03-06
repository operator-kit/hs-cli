# hs-cli

A command-line interface for the [HelpScout](https://www.helpscout.com/) API. Manage mailboxes, conversations, customers, tags, users, workflows, webhooks, docs sites, articles, and more from the terminal.

> [!TIP]
> **Built for shared and AI-assisted workflows**
> ML-powered, deterministic PII redaction — real identities are replaced with consistent fake ones so output stays fully readable for LLMs, debugging, and triage.
> Allowlist-based permissions (`resource:operation` pairs) restrict exactly which actions are permitted.
> See [PII Redaction Pipeline](#pii-redaction-pipeline) · [Permissions](#permissions).

## Install

```bash
# One-liner (Linux/macOS)
curl -sSL https://raw.githubusercontent.com/operator-kit/hs-cli/main/install.sh | bash

# PowerShell (Windows)
irm https://raw.githubusercontent.com/operator-kit/hs-cli/main/install.ps1 | iex

# Homebrew
brew install operator-kit/tap/hs

# From source (requires Go)
go install github.com/operator-kit/hs-cli/cmd/hs@latest

# MCP-first install (no manual binary setup)
npx -y @operatorkit/hs mcp -t stdio
```

### Build from source

```bash
git clone https://github.com/operator-kit/hs-cli.git
cd hs-cli
go build -o build/hs ./cmd/hs
```

## Quick start

### Inbox API (conversations, customers, mailboxes, etc.)

```bash
# Authenticate with HelpScout OAuth2 credentials
hs inbox auth login

# List conversations
hs inbox conversations list

# Filter by status, mailbox, tag, search
hs inbox conversations list --status pending --mailbox 12345
hs inbox conversations list --query "billing issue"

# Get a conversation with threads
hs inbox conversations get 67890 --embed threads

# Reply to a conversation
hs inbox conversations threads reply 67890 --customer user@example.com --body "Thanks for reaching out!"

# Add an internal note
hs inbox conversations threads note 67890 --body "Escalated to engineering"

# Create a conversation
hs inbox conversations create --mailbox 12345 --subject "New issue" --customer user@example.com --body "Details here"

# List customers, users, tags
hs inbox customers list --query "alice@example.com"
hs inbox users list
hs inbox tags list

# Team briefing — conversation counts per agent
hs inbox tools briefing

# Agent briefing with full thread data
hs inbox tools briefing --assigned-to 531600 --embed threads --format json

# Reports
hs inbox reports conversations --start 2026-01-01 --end 2026-01-31

# Workflows
hs inbox workflows run 33333 --conversation-ids 100,200,300

# Webhooks
hs inbox webhooks list
hs inbox webhooks create --url https://example.com/hook --events "convo.created" --secret my-secret
```

See [full Inbox API reference](docs/inbox-api.md) for all commands, flags, and options.

### Docs API (sites, collections, categories, articles)

```bash
# Authenticate with Docs API key
hs docs auth login

# List sites and collections
hs docs sites list
hs docs collections list --site <site-id>

# List and search articles
hs docs articles list --collection <collection-id>
hs docs articles search --query "password reset"

# Get article details (including draft version)
hs docs articles get <id>
hs docs articles get <id> --draft

# Create an article
hs docs articles create --collection <id> --name "Getting Started" --text "<p>Welcome...</p>"

# Manage categories
hs docs categories list <collection-id>
hs docs categories create --collection <id> --name "FAQs"

# Redirects
hs docs redirects create --site <id> --url-mapping /old --redirect /new

# Upload assets
hs docs assets article upload --file ./screenshot.png
```

See [full Docs API reference](docs/docs-api.md) for all commands, flags, and options.

### Tools (beyond the API)

In addition to 1:1 API command coverage, hs-cli includes higher-level workflow tools that aggregate data across multiple API calls.

**Team briefing** gives you an instant overview of your support team's workload:

```bash
# Team overview — conversation counts per agent
hs inbox tools briefing

# Filter by status
hs inbox tools briefing --status pending

# Agent summary — list a specific agent's conversations
hs inbox tools briefing --assigned-to 531600

# Full agent briefing with thread data (ideal for LLM context)
hs inbox tools briefing --assigned-to 531600 --embed threads --format json
```

The briefing command operates in three modes. Without flags, it shows every agent with their active conversation count. With `--assigned-to`, it lists that agent's conversations (same columns as `conversations list`). Add `--embed threads` to include full thread data per conversation — particularly useful for feeding to an LLM for triage, summarisation, or draft replies.

## Output formats

All commands support `--format` with four modes:

```bash
hs inbox conversations list                    # table (default)
hs inbox conversations list --format json      # clean JSON (HAL stripped, HTML→markdown)
hs inbox conversations list --format json-full # raw API response
hs inbox conversations list --format csv       # RFC 4180 CSV
```

`--format json` is read-optimized. Compared to the raw API response:

- Drops HAL noise (`_links`, `_embedded` wrappers)
- Converts HTML bodies to markdown (threads, saved replies)
- Flattens person objects to `"Name (email)"` strings
- Drops sentinel values (`closedBy: 0`, `closedByUser: {id: 0, ...}`)
- Drops empty arrays/strings and default-noise fields (`state: "published"`, `photoUrl`, etc.)
- Hoists embedded sub-resources to top level (e.g. customer `_embedded.emails` → `emails`)
- Renames for clarity (`userUpdatedAt` → `updatedAt`, `threads` count → `threadCount`)

Use `--format json-full` when you need write-safe data (e.g. round-tripping back into update commands).

## Pagination

```bash
hs inbox conversations list --page 2 --per-page 50  # navigate pages
hs inbox conversations list --no-paginate            # fetch all pages
```

## Configuration

```bash
# Set config values
hs inbox config set --inbox-app-id xxx --inbox-app-secret yyy
hs inbox config set --inbox-default-mailbox 12345 --format json
hs inbox config set --docs-api-key your-docs-key

# View config
hs inbox config get
hs inbox config path
```

Config file: `~/.config/hs/config.yaml` (Linux/macOS) or `%APPDATA%\helpscout\config.yaml` (Windows).

**Credential resolution order:** environment variables > OS keyring > config file.

### Environment variables

| Variable | Description |
|----------|-------------|
| `HS_INBOX_APP_ID` | HelpScout App ID |
| `HS_INBOX_APP_SECRET` | HelpScout App Secret |
| `HS_DOCS_API_KEY` | Docs API key |
| `HS_FORMAT` | Output format |
| `HS_INBOX_PII_MODE` | PII redaction mode: `off`, `customers`, `all` |
| `HS_INBOX_PII_ALLOW_UNREDACTED` | Allow `--unredacted` bypass |
| `HS_INBOX_PII_SECRET` | Secret salt for deterministic pseudonyms |
| `HS_INBOX_PERMISSIONS` | Inbox permission policy |
| `HS_DOCS_PERMISSIONS` | Docs permission policy |
| `HS_NO_UPDATE_CHECK` | Disable daily update check (`1`) |

## MCP Server

hs-cli ships an embedded MCP server with one tool per operational leaf command. No binary install required — npx handles everything:

```json
{
  "mcpServers": {
    "helpscout": {
      "command": "npx",
      "args": ["-y", "@operatorkit/hs", "mcp", "-t", "stdio"],
      "env": {
        "HS_INBOX_APP_ID": "your-app-id",
        "HS_INBOX_APP_SECRET": "your-app-secret",
        "HS_INBOX_PERMISSIONS": "*:read",
        "HS_DOCS_API_KEY": "your-docs-api-key",
        "HS_DOCS_PERMISSIONS": "*:read"
      }
    }
  }
}
```

Only the credentials for the APIs you use are required — `HS_INBOX_APP_ID` + `HS_INBOX_APP_SECRET` for Inbox, `HS_DOCS_API_KEY` for Docs. Permission and PII variables are optional.

If using the binary directly, replace `"npx"` / `["-y", "@operatorkit/hs", "mcp", "-t", "stdio"]` with `"hs"` / `["mcp", "-t", "stdio"]`.

Tool names are namespaced (e.g. `helpscout_inbox_conversations_list`). Default output is clean JSON; set `output_mode: "json_full"` per call or `--default-output-mode json_full` server-wide. Auth, config, and permissions commands are excluded from the MCP surface.

## PII Redaction Pipeline

hs-cli includes an ML-powered PII redaction system designed for shared terminals, MCP/LLM workflows, and incident-safe exports.

### Why this matters

Traditional redaction tools either hide entire blocks of content (destroying context) or rely on brittle regex patterns that miss real names. hs-cli takes a different approach:

- **Full content, no PII.** An ML-based Named Entity Recognition (NER) model detects person names in freeform text — conversation bodies, notes, subjects — and replaces them with consistent fake identities. The output reads naturally and retains its full meaning.
- **LLM-ready output.** Redacted conversations can be piped directly to AI tools for summarisation, triage, or analysis without leaking customer data. The content stays complete and coherent, unlike blanked-out or `[REDACTED]` approaches.
- **Deterministic pseudonyms.** The same real identity always maps to the same fake name, email, and phone — across commands and sessions. You can follow a conversation thread, cross-reference between outputs, and reason about the data just as you would with the originals.
- **Mode-aware.** In `customers` mode, only customer data is redacted; team member names are preserved so internal context stays clear.

### How it works

Redaction is applied in layered stages:

1. **Structured identity redaction** — known person/customer/user fields (names, emails, phones) across all output formats. A JSON walker covers nested payloads.
2. **ML-powered free-text redaction** — a multilingual DistilBERT NER model detects person names in freeform text. Downloaded on first use (~100 MB), runs entirely locally. Supports 10 languages: Arabic, Chinese, Dutch, English, French, German, Italian, Latvian, Portuguese, and Spanish. A regex pipeline catches non-name PII: emails, phones, SSNs, credit cards, addresses, IPs, MACs, and URLs.
3. **Raw source protection** — `threads source` and `threads source-rfc822` are redacted when PII mode is enabled.
4. **Fallback before model download** — freeform text fields are hidden entirely rather than shown unredacted. Structured field redaction still works.

### Deterministic anonymization — no PII stored

Fake names are **computed, not stored**. The CLI never writes a mapping of real identities to fake ones — not to disk, not to a database, nowhere. Each time you run a command, fake names are derived on-the-fly from the original using a one-way hash. Because the hash is deterministic, the same real identity always produces the same fake name across commands and sessions — so you can follow conversations and cross-reference outputs naturally, without any PII being persisted.

When the command finishes, all in-memory mappings are discarded.

Setting `HS_INBOX_PII_SECRET` (optional) adds a secret salt to the hash, making fake names unique to your environment and harder to reverse-engineer.

### NER model management

The CLI handles model setup automatically — on first use you'll be prompted to download the model bundle (~100 MB). Everything runs locally; no API calls, no data leaves your machine.

```bash
hs pii-model install     # download the model bundle
hs pii-model status      # check install status
hs pii-model uninstall   # remove the model from disk
```

The model supports Linux (amd64/arm64), macOS (amd64/arm64), and Windows (amd64/arm64).

### Quick start

```bash
# Enable customer-only redaction
hs inbox config set --inbox-pii-mode customers

# Run with redaction — full content, fake identities
hs inbox conversations get 12345 --embed threads

# Temporarily bypass (when allowed)
hs inbox --unredacted conversations get 12345 --format json-full
```

### Limitations

PII redaction is a best-effort safety layer, not a guarantee:

- The NER model may miss unusual names, single-token names under 3 characters, or names in dense code/markup.
- Regex patterns for emails, phones, addresses may occasionally match non-PII strings.
- The redaction pipeline operates on CLI output only — it does not modify data in HelpScout.

For high-sensitivity environments, pair PII redaction with `inbox_pii_allow_unredacted: false` to prevent accidental bypasses.

## Permissions

An allowlist-based permission system that restricts which operations are permitted per API namespace. When set, only explicitly granted `resource:operation` pairs are allowed. When unset, everything is allowed.

```bash
# Inbox: read-only access
HS_INBOX_PERMISSIONS="conversations:read,customers:read,mailboxes:read"

# Docs: full access to articles, read-only everything else
HS_DOCS_PERMISSIONS="articles:*,*:read"

# Inspect current policy
hs inbox permissions
```

| Namespace | Env var | Resources |
|-----------|---------|-----------|
| Inbox | `HS_INBOX_PERMISSIONS` | `conversations`, `customers`, `mailboxes`, `tags`, `users`, `teams`, `organizations`, `properties`, `workflows`, `webhooks`, `saved-replies`, `reports`, `ratings` |
| Docs | `HS_DOCS_PERMISSIONS` | `sites`, `collections`, `categories`, `articles`, `redirects`, `assets` |

**Operations**: `read` (list/get), `write` (create/update/reply/note/run/upload), `delete`

See the [Inbox API reference](docs/inbox-api.md#permissions) and [Docs API reference](docs/docs-api.md#permissions) for detailed examples.

## Shell completion

```bash
hs completion bash > /etc/bash_completion.d/helpscout   # Bash
hs completion zsh > "${fpath[1]}/_helpscout"             # Zsh
hs completion fish > ~/.config/fish/completions/hs.fish  # Fish
hs completion powershell | Out-String | Invoke-Expression # PowerShell
```

## Self-update

```bash
hs update
```

Checks for a newer release on GitHub and replaces the binary in-place. A background check runs daily and prints a notice to stderr when a new version is available. Disable with `HS_NO_UPDATE_CHECK=1`.

## Create a HelpScout App with App ID & App Secret

This is *not* via the `Manage` -> `Apps` flow.

1. Select your profile icon in the top right
2. Then "Your Profile"
3. Select "My Apps" in the left bar.
4. "Create App"
5. Enter an App Name - "Agent CLI"
6. Then a redirection URL (not needed) use a valid `https` url - "https://mysite.com"
7. You have generated an App ID and App Secret - use these in the `hs inbox auth login` command
8. Result should be: `Validating credentials... Authenticated. Found 1 mailboxes.`

## Developer guide

See [DEVELOPMENT.md](DEVELOPMENT.md) for project structure, build instructions, test architecture, and release process.

## Roadmap

Planned features and improvements:

- **Reverse PII mapping for replies** — Compose replies using fake names (manually or via LLM), and the CLI transparently swaps them back to real identities before sending. The LLM never sees real PII, but the customer receives a properly addressed response. True end-to-end PII redaction without compromising functionality.

## License

[MIT](LICENSE)
