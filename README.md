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

### Protected command input

Values such as customer emails, search queries, message bodies, authored Docs
content, credential secrets, and local upload paths must not be typed on the
command line. Direct use is rejected, and flags marked `protected input only`
in `--help` must be supplied in a schema-versioned JSON envelope over stdin or
through a private regular file. To keep values out of shell history, start the
command first and only then paste the envelope—do not embed it in a shell
pipeline:

```bash
hs --protected-input - inbox conversations threads reply 67890
```

Then paste the envelope into its stdin and signal EOF (`Ctrl-D` on Unix;
`Ctrl-Z`, then Enter, in a Windows terminal):

```json
{
  "schema": 1,
  "command": ["inbox", "conversations", "threads", "reply"],
  "flags": {
    "customer": "user@example.com",
    "body": "Thanks for reaching out!"
  }
}
```

The `command` array must match the invoked command exactly. `flags` accepts
string and string-array values. Protected files are limited to 4 MiB, must be
regular files, and on Unix must not be accessible to group or other users:

```bash
chmod 600 request.json
hs --protected-input request.json inbox conversations threads reply 67890
```

On Windows, prefer stdin or restrict the file ACL yourself; the CLI can verify
the regular file type but cannot infer privacy from Go's portable mode bits.

MCP performs this transport automatically. Its child argv and rendered command
errors contain placeholders, never the supplied protected values.

### Inbox API (conversations, customers, mailboxes, etc.)

```bash
# Authenticate with HelpScout OAuth2 credentials
hs inbox auth login

# List conversations
hs inbox conversations list

# Filter by status, mailbox, and tag
hs inbox conversations list --status pending --mailbox 12345

# Get a conversation with threads
hs inbox conversations get 67890 --embed threads

# List customers, users, tags
hs inbox customers list
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
```

See [full Inbox API reference](docs/inbox-api.md) for all commands, flags, and options.

### Docs API (sites, collections, categories, articles)

```bash
# Authenticate with Docs API key
hs docs auth login

# List sites and collections
hs docs sites list
hs docs collections list --site <site-id>

# List articles
hs docs articles list --collection <collection-id>

# Get article details (including draft version)
hs docs articles get <id>
hs docs articles get <id> --draft

# Manage categories
hs docs categories list <collection-id>
```

See [full Docs API reference](docs/docs-api.md) for all commands, flags, and options.

### Tools (beyond the API)

In addition to 1:1 API command coverage, hs-cli includes higher-level workflow tools that aggregate data across multiple API calls.

**Team briefing** gives you an instant overview of your support team's workload:

```bash
# Team overview — active/pending/closed counts per agent
hs inbox tools briefing

# Agent summary — list a specific agent's open conversations
hs inbox tools briefing --assigned-to 531600

# Full agent briefing with thread data (ideal for LLM context)
hs inbox tools briefing --assigned-to 531600 --embed threads --format json
```

The briefing command operates in three tiers. Without flags, it shows every agent with their active, pending, and closed (7d) conversation counts. With `--assigned-to`, it shows a summary line for that agent and lists their open conversations. Add `--embed threads` to include full thread data per conversation — particularly useful for feeding to an LLM for triage, summarisation, or draft replies.

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
hs inbox config set --inbox-app-id xxx
hs inbox config set --inbox-default-mailbox 12345 --format json

# Use auth login prompts or the protected-input envelope for credential values.

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
| `HS_INBOX_PII_SECRET` | Optional explicit private HMAC key; requires `HS_INBOX_PII_KEY_ID` |
| `HS_INBOX_PII_KEY_ID` | Public rotation ID paired with an explicit PII secret |
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

Maintainers should treat the
[PII redaction hardening contract](docs/pii-redaction-hardening-contract.md)
as the normative checklist for all 14 completed security and privacy findings.
The [OpenAI Privacy Filter evaluation](docs/openai-privacy-filter-evaluation.md)
documents the candidate model's coverage gains, footprint and runtime trade-offs,
and the gated path recommended before any detector migration.
The accompanying
[single-detector migration plan](docs/openai-privacy-filter-migration-plan.md)
defines the phased implementation, permanent regression and performance tests,
objective pass/fail gates, preview, rollback, and eventual DistilBERT retirement.

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

Fake names are **computed, not stored**. The CLI never writes a mapping of real identities to fake ones — not to disk, not to a database, nowhere. Each time you run a command, fake names are derived on-the-fly with keyed HMAC. Because the derivation is deterministic, the same real identity produces the same fake name across commands and sessions — so you can follow conversations and cross-reference outputs naturally, without any PII mapping being persisted. Display names include a short deterministic disambiguator and the public key ID so distinct people and secret rotations remain visually distinguishable; generated emails retain their established deterministic format.

When the command finishes, all in-memory mappings are discarded.

When neither key variable is set, the CLI generates a private 32-byte secret and
an independent public key ID, then stores the versioned record in the OS
keyring. Existing raw secret records are migrated under the initialization lock
without changing the secret bytes. For an explicit cross-machine identity
domain, set both `HS_INBOX_PII_SECRET` and `HS_INBOX_PII_KEY_ID`; the key ID is
normalized to lowercase and must contain 1–32 letters, numbers, `.`, `_`, or
`-`. Keep both stable for
deterministic display identities, and change both intentionally during a
rotation. The public ID is supplied independently and is never derived from the
secret.

### NER model management

The CLI handles model setup automatically — on first use you'll be prompted to download the model bundle (~100 MB). Everything runs locally; no API calls, no data leaves your machine.

```bash
hs pii-model install     # download the model bundle
hs pii-model status      # check install status
hs pii-model uninstall   # remove the model from disk
```

The model runtime currently supports Linux (amd64/arm64) and macOS
(amd64/arm64). Windows keeps free-form content fail-closed until a native
runtime/model smoke test is available.

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
