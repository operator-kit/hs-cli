# @operatorkit/hs

A command-line interface and [MCP](https://modelcontextprotocol.io/) server for the [HelpScout](https://www.helpscout.com/) API. Manage mailboxes, conversations, customers, tags, users, workflows, webhooks, and knowledge base content from the terminal.

> **Built for shared and AI-assisted workflows**
> ML-powered, deterministic PII redaction — real identities are replaced with consistent fake ones so output stays fully readable for LLMs, debugging, and triage.
> Allowlist-based permissions (`resource:operation` pairs) restrict exactly which actions are permitted.
> See [PII Redaction Pipeline](https://github.com/operator-kit/hs-cli#pii-redaction-pipeline) · [Permissions](https://github.com/operator-kit/hs-cli#permissions).

## Install

```bash
# Run directly (no install)
npx -y @operatorkit/hs version

# Global install
npm i -g @operatorkit/hs
```

On install, the matching platform binary is downloaded from GitHub Releases. Supported: `linux`, `darwin`, `windows` on `amd64`/`arm64`.

## Quick start

```bash
# Authenticate (Inbox API — OAuth2 app credentials)
npx -y @operatorkit/hs inbox auth login

# Authenticate (Docs API — API key)
npx -y @operatorkit/hs docs auth login

# List conversations
npx -y @operatorkit/hs inbox conversations list --status active

# Get a conversation with threads
npx -y @operatorkit/hs inbox conversations get 67890 --embed threads

# Search articles
npx -y @operatorkit/hs docs articles search --query "getting started"

# Team briefing — conversation counts per agent
npx -y @operatorkit/hs inbox tools briefing
```

## Authentication

### Inbox API (OAuth2)

Requires an App ID and App Secret from your HelpScout app settings:

> **Your Profile** > My Apps > Create App

```bash
npx -y @operatorkit/hs inbox auth login
```

Prompts for your App ID and App Secret, validates them against the API, and stores them securely in your OS keyring.

### Docs API (API key)

Requires a Docs API key from your HelpScout account:

```bash
npx -y @operatorkit/hs docs auth login
```

**Credential resolution order:** environment variables > OS keyring > config file.

For non-interactive setup, use `config set` commands or pass environment variables directly. For MCP, pass credentials via [MCP config](#mcp-server).

## API coverage

### Inbox API (`hs inbox ...`)

Full CRUD for all Inbox API resources:

- **Conversations** — list, get, create, update, delete, tags, custom fields, snooze
- **Threads** — list, reply, note, create (chat/customer/phone), update, source, source-rfc822
- **Customers** — list, get, create, update, overwrite, delete
- **Mailboxes** — list, get, folders, custom fields, routing
- **Tags** — list, get
- **Users** — list, get, me, delete, status
- **Teams** — list, members
- **Organizations** — list, get, create, update, delete, conversations, customers, properties
- **Workflows** — list, run, update-status
- **Webhooks** — list, get, create, update, delete
- **Saved Replies** — list, get, create, update, delete
- **Reports** — chats, company, conversations, customers, docs, email, productivity, ratings, users
- **Properties** — customer properties, conversation properties
- **Ratings** — get
- **Attachments** — upload, list, get, delete

See [full Inbox API reference](https://github.com/operator-kit/hs-cli/blob/main/docs/inbox-api.md) for all commands, flags, and options.

### Docs API (`hs docs ...`)

Full CRUD for all Docs API resources:

- **Articles** — list, search, get, create, update, delete, upload, revisions, drafts, related, view count
- **Categories** — list, get, create, update, reorder, delete
- **Collections** — list, get, create, update, delete
- **Sites** — list, get, create, update, delete, restrictions
- **Redirects** — list, get, find, create, update, delete
- **Assets** — article upload, settings upload

See [full Docs API reference](https://github.com/operator-kit/hs-cli/blob/main/docs/docs-api.md) for all commands, flags, and options.

### Tools (beyond the API)

**Team briefing** aggregates data across multiple API calls for an instant overview of your support team's workload:

```bash
hs inbox tools briefing                                            # team overview — counts per agent
hs inbox tools briefing --assigned-to 531600                       # agent summary — list conversations
hs inbox tools briefing --assigned-to 531600 --embed threads       # full agent briefing with thread data
```

The briefing with `--embed threads` is particularly useful for feeding to an LLM for triage, summarisation, or draft replies.

## MCP server

Start the embedded MCP server for AI-assisted workflows:

```bash
npx -y @operatorkit/hs mcp -t stdio
```

124 MCP tools auto-discovered from the command tree (85 inbox + 39 docs). Management commands (`auth`, `config`, `permissions`) are excluded.

### MCP client config

```json
{
  "mcpServers": {
    "helpscout": {
      "command": "npx",
      "args": ["-y", "@operatorkit/hs", "mcp", "-t", "stdio"],
      "env": {
        "HS_INBOX_APP_ID": "your-app-id",
        "HS_INBOX_APP_SECRET": "your-app-secret",
        "HS_DOCS_API_KEY": "your-docs-api-key",
        "HS_INBOX_PERMISSIONS": "*:read",
        "HS_DOCS_PERMISSIONS": "*:read"
      }
    }
  }
}
```

Only the credentials for the APIs you use are required — `HS_INBOX_APP_ID` + `HS_INBOX_APP_SECRET` for Inbox, `HS_DOCS_API_KEY` for Docs. Permission and PII variables are optional.

## PII redaction

An ML-powered PII redaction system designed for shared terminals, MCP/LLM workflows, and incident-safe exports.

- **ML-based name detection** — a multilingual NER model detects person names in freeform text (bodies, subjects, notes) and replaces them with consistent fake identities. Supports 10 languages: Arabic, Chinese, Dutch, English, French, German, Italian, Latvian, Portuguese, and Spanish.
- **Deterministic pseudonyms** — same real identity always maps to the same fake name, email, and phone across commands and sessions. No mappings stored anywhere.
- **Mode-aware** — `customers` mode redacts only customer data; `all` mode redacts everyone.
- **Runs locally** — the model runs entirely on your machine via ONNX Runtime. No API calls, no data leaves your system.

```bash
hs pii-model install                                    # download model (~100 MB, one-time)
hs pii-model status                                     # check install status
hs inbox config set --inbox-pii-mode customers           # enable redaction
hs inbox conversations get 12345 --embed threads         # output uses fake identities
```

Without the model installed, freeform text fields are hidden with a notice. Structured field redaction (names, emails, phones) always works regardless.

## Output formats

| Format | Flag | Description |
|--------|------|-------------|
| Table | `--format table` | Human-readable table (default) |
| JSON | `--format json` | Clean JSON — HAL noise stripped, HTML→markdown, fields normalized |
| JSON-full | `--format json-full` | Raw API response, pretty-printed |
| CSV | `--format csv` | RFC 4180 CSV with headers |

## Environment variables

| Variable | Description |
|----------|-------------|
| `HS_INBOX_APP_ID` | Inbox API App ID |
| `HS_INBOX_APP_SECRET` | Inbox API App Secret |
| `HS_DOCS_API_KEY` | Docs API key |
| `HS_FORMAT` | Default output format |
| `HS_INBOX_PII_MODE` | PII redaction mode: `off`, `customers`, `all` |
| `HS_INBOX_PII_ALLOW_UNREDACTED` | Allow `--unredacted` bypass |
| `HS_INBOX_PII_SECRET` | Secret salt for deterministic pseudonyms |
| `HS_INBOX_PERMISSIONS` | Inbox permission policy |
| `HS_DOCS_PERMISSIONS` | Docs permission policy |
| `HS_NO_UPDATE_CHECK` | Disable daily update check (`1`) |

## Links

- [GitHub](https://github.com/operator-kit/hs-cli)
- [Inbox API reference](https://github.com/operator-kit/hs-cli/blob/main/docs/inbox-api.md)
- [Docs API reference](https://github.com/operator-kit/hs-cli/blob/main/docs/docs-api.md)
- [Developer guide](https://github.com/operator-kit/hs-cli/blob/main/DEVELOPMENT.md)
- [Issues](https://github.com/operator-kit/hs-cli/issues)
- [HelpScout API docs](https://developer.helpscout.com/)

## License

[MIT](https://github.com/operator-kit/hs-cli/blob/main/LICENSE)
