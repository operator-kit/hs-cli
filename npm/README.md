# @operatorkit/hs

A command-line interface for the [HelpScout](https://www.helpscout.com/) API. Manage mailboxes, conversations, customers, tags, users, workflows, webhooks, and knowledge base content from the terminal.

Ships with an embedded [MCP](https://modelcontextprotocol.io/) server for AI-assisted workflows.

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

# List mailboxes
npx -y @operatorkit/hs inbox mailboxes list

# List conversations
npx -y @operatorkit/hs inbox conversations list --status active

# Search articles
npx -y @operatorkit/hs docs articles search --query "getting started"
```

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

### Docs API (`hs docs ...`)

Full CRUD for all Docs API resources:

- **Articles** — list, search, get, create, update, delete, upload, revisions, drafts, related, view count
- **Categories** — list, get, create, update, reorder, delete
- **Collections** — list, get, create, update, delete
- **Sites** — list, get, create, update, delete, restrictions
- **Redirects** — list, get, find, create, update, delete
- **Assets** — article upload, settings upload

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
        "HS_INBOX_PERMISSIONS": "*:read"
      }
    }
  }
}
```

## Output formats

| Format | Flag | Description |
|--------|------|-------------|
| Table | `--format table` | Human-readable table (default) |
| JSON | `--format json` | Clean JSON — HAL noise stripped, HTML→markdown, fields normalized |
| JSON-full | `--format json-full` | Raw API response, pretty-printed |
| CSV | `--format csv` | RFC 4180 CSV with headers |

## Safety features

- **PII redaction** — deterministic, layered pipeline (structured fields + free-text + source payloads). Modes: `off`, `customers`, `all`.
- **Permissions** — allowlist-based `resource:operation` pairs restrict which actions are permitted. Set via `HS_INBOX_PERMISSIONS` / `HS_DOCS_PERMISSIONS`.
- **Rate limiting** — built-in rate limiters respect API quotas (Inbox: 200/min, Docs: 2000/10min).

## Environment variables

| Variable | Description |
|----------|-------------|
| `HS_INBOX_APP_ID` | Inbox API App ID |
| `HS_INBOX_APP_SECRET` | Inbox API App Secret |
| `HS_DOCS_API_KEY` | Docs API key |
| `HS_FORMAT` | Default output format |
| `HS_INBOX_PII_MODE` | PII redaction mode: `off`, `customers`, `all` |
| `HS_INBOX_PERMISSIONS` | Inbox permission policy |
| `HS_DOCS_PERMISSIONS` | Docs permission policy |

## Links

- [GitHub](https://github.com/operator-kit/hs-cli)
- [Issues](https://github.com/operator-kit/hs-cli/issues)
- [HelpScout API docs](https://developer.helpscout.com/)

## License

[MIT](https://github.com/operator-kit/hs-cli/blob/main/LICENSE)
