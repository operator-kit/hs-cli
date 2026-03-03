# hs-cli

A command-line interface for the [HelpScout](https://www.helpscout.com/) API. Manage mailboxes, conversations, customers, tags, users, workflows, and webhooks from the terminal.

> **Built for shared and AI-assisted workflows**
> hs-cli ships with a deterministic, layered PII redaction pipeline (structured fields + free-text + source payload protection), plus strict per-command override controls.
> An allowlist-based permission system (`resource:operation` pairs) lets you restrict exactly which actions are permitted.
> See [PII Redaction Pipeline](#pii-redaction-pipeline) · [Permissions](#permissions).

## Install

```bash
# One-liner (Linux/macOS)
curl -sSL https://raw.githubusercontent.com/operator-kit/hs-cli/main/install.sh | bash

# PowerShell (Windows)
irm https://raw.githubusercontent.com/operator-kit/hs-cli/main/install.ps1 | iex

# Homebrew
brew install operator-kit/tap/hs

# Custom install directory
curl -sSL https://raw.githubusercontent.com/operator-kit/hs-cli/main/install.sh | INSTALL_DIR=~/.local/bin bash

# Specific version
curl -sSL https://raw.githubusercontent.com/operator-kit/hs-cli/main/install.sh | HS_VERSION=v0.0.1 bash

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

## Authentication

The CLI uses HelpScout's OAuth2 client credentials flow. You'll need an App ID and App Secret from your HelpScout app settings (My Apps > Create My App).

### Interactive login (recommended)

```bash
hs inbox auth login
```

This prompts for your App ID and App Secret, validates them against the API, and stores them securely in your OS keyring.

### Non-interactive setup

```bash
hs inbox config set --inbox-app-id your-app-id --inbox-app-secret your-app-secret
hs inbox config set --inbox-default-mailbox 12345 --format json
hs inbox config set --inbox-pii-mode customers --inbox-pii-allow-unredacted
```

### Environment variables

```bash
export HS_INBOX_APP_ID=your-app-id
export HS_INBOX_APP_SECRET=your-app-secret
export HS_INBOX_PII_MODE=customers
export HS_INBOX_PII_ALLOW_UNREDACTED=1
```

### Config file

Create `~/.config/hs/config.yaml` (Linux/macOS) or `%APPDATA%\helpscout\config.yaml` (Windows):

```yaml
inbox_app_id: your-app-id
inbox_app_secret: your-app-secret
inbox_default_mailbox: 12345
format: table
inbox_pii_mode: off
inbox_pii_allow_unredacted: false
```

**Credential resolution order:** environment variables > OS keyring > config file.

### Check status

```bash
hs inbox auth status
```

### Logout

```bash
hs inbox auth logout
```

## Global flags

| Flag | Description | Default |
|------|-------------|---------|
| `--config` | Config file path | `~/.config/hs/config.yaml` |
| `--format` | Output format: `table`, `json`, `json-full`, `csv` | `table` |
| `--no-paginate` | Fetch all pages (combine all results) | `false` |
| `--page` | Page number | `1` |
| `--per-page` | Results per page | `25` |
| `--debug` | Log HTTP requests/responses to `hs-debug.log` | `false` |
| `--unredacted` | Disable PII redaction for this command when allowed by config/env | `false` |

The `--format` flag can also be set permanently via the `format` key in the config file, or the `HS_FORMAT` environment variable.

## MCP Server

hs-cli ships an embedded MCP server, exposed as:

```bash
hs mcp -t stdio
```

### Coverage model

- One MCP tool per operational `hs inbox ...` leaf command.
- Tool names are namespaced with prefixes like `helpscout_inbox_conversations_list`.
- `inbox auth`, `inbox config`, and `inbox permissions` are intentionally excluded.
- Existing command permissions (`HS_INBOX_PERMISSIONS`) still apply.
- Existing redaction controls (`inbox_pii_mode`, `--unredacted`, `inbox_pii_allow_unredacted`) still apply.

### Output contract

- Default MCP tool output mode is clean JSON (`--format json`).
- Per tool call, set `output_mode: "json_full"` for raw payload shape.
- Server-wide default can be changed with:

```bash
hs mcp -t stdio --default-output-mode json_full
```

### MCP client config examples

Binary install:

```json
{
  "mcpServers": {
    "helpscout": {
      "command": "hs",
      "args": ["mcp", "-t", "stdio"],
      "env": {
        "HS_INBOX_APP_ID": "your-app-id",
        "HS_INBOX_APP_SECRET": "your-app-secret",
        "HS_INBOX_PERMISSIONS": "*:read"
      }
    }
  }
}
```

npx wrapper:

```json
{
  "mcpServers": {
    "helpscout": {
      "command": "npx",
      "args": ["-y", "@operatorkit/hs", "mcp", "-t", "stdio"],
      "env": {
        "HS_INBOX_APP_ID": "your-app-id",
        "HS_INBOX_APP_SECRET": "your-app-secret",
        "HS_INBOX_PERMISSIONS": "*:read"
      }
    }
  }
}
```

## PII Redaction Pipeline

hs-cli includes a production-focused PII redaction system designed for shared terminals, MCP/LLM workflows, and incident-safe exports.

### Why this matters

- Prevents accidental exposure of customer/agent data in terminal output.
- Keeps output useful for debugging and triage by preserving structure and readability.
- Gives operators explicit control: strict defaults with an auditable per-command escape hatch.

### Depth of protection

Redaction is applied in layered stages:

1. Structured identity redaction
- Redacts known person/customer/user fields (names, emails, phones) across table, csv, json, and json-full outputs.
- Covers nested payloads through a JSON walker.

2. Free-text redaction
- Scans thread/content text (`body`, `action`, `subject`, `preview`, source payloads) with a broad regex pipeline.
- Detects and replaces common PII classes such as emails, phones, SSNs, card-like values, addresses, IPs, URLs, and person-name patterns.

3. Raw source protection
- `threads source` and `threads source-rfc822` are redacted when PII mode is enabled.

### Deterministic anonymization and cache behavior

- Replacements are deterministic: the same input maps to the same pseudonym across commands and runs.
- `HS_INBOX_PII_SECRET` (optional) adds a secret salt for stronger pseudonym generation.
- Without `HS_INBOX_PII_SECRET`, deterministic hashing is still used (stable fallback).
- The engine also keeps an in-memory per-command cache so repeated values in the same output are co-derived consistently and processed efficiently.

### Modes and override controls

PII mode:

- `off`: no redaction (default)
- `customers`: redact customer identities
- `all`: redact customer + user identities

Override policy:

- `--unredacted` disables redaction for a single command
- It is only allowed when `inbox_pii_allow_unredacted: true` or `HS_INBOX_PII_ALLOW_UNREDACTED=1`
- If overrides are disallowed and redaction is enabled, the command fails fast with a clear error

### Quick-start examples

```bash
# Enable customer-only redaction globally
hs inbox config set --inbox-pii-mode customers

# Allow temporary per-command bypasses (for incident response/debugging)
hs inbox config set --inbox-pii-allow-unredacted

# Run safely redacted output
hs inbox conversations list --format json
hs inbox conversations threads source-rfc822 12345 67890

# Temporarily bypass redaction for one command (only if allowed)
hs inbox --unredacted conversations get 12345 --format json-full

# Disable per-command bypass again
hs inbox config set --inbox-pii-allow-unredacted=false

# Fully disable redaction
hs inbox config set --inbox-pii-mode off --inbox-pii-allow-unredacted=false
```

## Commands

Inbox API commands are namespaced under `hs inbox ...`.

### MCP

```bash
# Start MCP server over stdio
hs mcp -t stdio

# Use raw json-full output as the MCP default
hs mcp -t stdio --default-output-mode json_full
```

### Config

```bash
# Set one or more config values
hs inbox config set --inbox-app-id xxx --inbox-app-secret yyy
hs inbox config set --inbox-default-mailbox 12345
hs inbox config set --format json
hs inbox config set --inbox-pii-mode customers
hs inbox config set --inbox-pii-allow-unredacted

# Disable/unset PII features explicitly
hs inbox config set --inbox-pii-allow-unredacted=false
hs inbox config set --inbox-pii-mode off --inbox-pii-allow-unredacted=false

# View all config values
hs inbox config get

# View a single value
hs inbox config get inbox-app-id

# Print config file path
hs inbox config path
```

#### config set flags

| Flag | Type | Description |
|------|------|-------------|
| `--inbox-app-id` | string | HelpScout App ID |
| `--inbox-app-secret` | string | HelpScout App Secret |
| `--inbox-default-mailbox` | int | Default mailbox ID |
| `--format` | string | Output format: table, json, json-full, csv |
| `--inbox-pii-mode` | string | PII redaction mode: off, customers, all |
| `--inbox-pii-allow-unredacted` | bool | Allow per-request `--unredacted` override |
| `--docs-api-key` | string | HelpScout Docs API key |

### Self-update

```bash
hs update
```

Checks for a newer release on GitHub and replaces the binary in-place. A background check runs daily and prints a notice to stderr when a new version is available. Disable with `HS_NO_UPDATE_CHECK=1`.

### Mailboxes

```bash
# List all mailboxes
hs inbox mailboxes list
hs inbox mb list                 # alias

# Get mailbox details
hs inbox mailboxes get 12345

# Mailbox folders, custom fields, and routing
hs inbox mailboxes folders list 12345
hs inbox mailboxes custom-fields list 12345
hs inbox mailboxes routing get 12345
hs inbox mailboxes routing update 12345 --json '{"enabled":true}'

# JSON output
hs inbox mailboxes list --format json

# Fetch all pages
hs inbox mailboxes list --no-paginate
```

#### mailboxes routing update flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--json` | string | yes | Routing update payload as JSON object |

### Conversations

```bash
# List conversations
hs inbox conversations list
hs inbox conv list               # alias

# Filter by status
hs inbox conversations list --status closed
hs inbox conversations list --status pending
hs inbox conversations list --status all

# Filter by mailbox
hs inbox conversations list --mailbox 12345

# Advanced filters
hs inbox conversations list --folder 12 --tag billing --assigned-to 99
hs inbox conversations list --modified-since "2026-01-01T00:00:00Z"
hs inbox conversations list --number 23053

# Search
hs inbox conversations list --query "billing issue"

# Sort and custom fields
hs inbox conversations list --sort-field createdAt --sort-order desc
hs inbox conversations list --custom-fields-by-ids "10:foo,11:bar"

# Embed threads in response
hs inbox conversations list --embed threads

# Get conversation details
hs inbox conversations get 67890
hs inbox conversations get 67890 --embed threads

# Create a conversation
hs inbox conversations create \
  --mailbox 12345 \
  --subject "New issue" \
  --customer user@example.com \
  --body "Description of the issue" \
  --type email \
  --status active \
  --tags "bug,urgent" \
  --assign-to 999 \
  --created-at "2026-01-02T15:04:05Z" \
  --imported \
  --auto-reply \
  --field "10=foo" \
  --field "11=bar"

# Update a conversation
hs inbox conversations update 67890 --subject "Updated subject"
hs inbox conversations update 67890 --status closed

# Delete a conversation
hs inbox conversations delete 67890

# Set conversation tags
hs inbox conversations tags set 67890 --tag vip,bug

# Set conversation custom fields
hs inbox conversations fields set 67890 --field "10=foo" --field "11=bar"

# Snooze and unsnooze
hs inbox conversations snooze set 67890 --until "2026-02-20T00:00:00Z"
hs inbox conversations snooze clear 67890

# Upload/list/get/delete conversation attachments
hs inbox conversations attachments upload 67890 --thread-id 123 --file ./invoice.pdf --filename report.pdf --mime-type application/pdf
hs inbox conversations attachments list 67890
hs inbox conversations attachments get 67890 555
hs inbox conversations attachments delete 67890 555
```

#### conversations list flags

| Flag | Type | Description |
|------|------|-------------|
| `--status` | string | Filter: active, closed, pending, spam, all (default: active) |
| `--mailbox` | string | Filter by mailbox ID |
| `--folder` | string | Filter by folder ID |
| `--tag` | string | Filter by tag |
| `--assigned-to` | string | Filter by assigned user ID |
| `--modified-since` | string | Filter by last-modified timestamp |
| `--number` | string | Filter by conversation number |
| `--sort-field` | string | Sort field (createdAt, modifiedAt, number, etc.) |
| `--sort-order` | string | Sort direction: asc, desc |
| `--custom-fields-by-ids` | string | Custom field filters (format: `id:value,id:value`) |
| `--query` | string | Free-text search query |
| `--embed` | string | Embed sub-resources (e.g. `threads`) |

#### conversations get flags

| Flag | Type | Description |
|------|------|-------------|
| `--embed` | string | Embed sub-resources (e.g. `threads`) |

#### conversations create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--mailbox` | string | yes | Mailbox ID |
| `--subject` | string | yes | Conversation subject |
| `--customer` | string | yes | Customer email |
| `--body` | string | yes | Initial message body |
| `--type` | string | | Conversation type (default: email) |
| `--status` | string | | Initial status (default: active) |
| `--tags` | strings | | Comma-separated tags |
| `--assign-to` | int | | User ID to assign to |
| `--created-at` | string | | Creation timestamp (for imports) |
| `--imported` | bool | | Mark as imported |
| `--auto-reply` | bool | | Trigger auto-reply |
| `--field` | strings | | Custom field `<id>=<value>` (repeatable) |

#### conversations update flags

| Flag | Type | Description |
|------|------|-------------|
| `--subject` | string | New subject |
| `--status` | string | New status |

#### conversations tags set flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--tag` | strings | yes | Tags to apply (comma-separated) |

#### conversations fields set flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--field` | strings | yes | Custom field `<id>=<value>` (repeatable) |

#### conversations snooze set flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--until` | string | yes | Snooze-until timestamp |

#### conversations attachments upload flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--thread-id` | int | yes | Thread ID |
| `--file` | string | yes | Path to file |
| `--filename` | string | | Override filename |
| `--mime-type` | string | | Override MIME type |

### Threads

Threads are subcommands of conversations.

```bash
# List threads for a conversation
hs inbox conversations threads list 67890

# Reply to a conversation
hs inbox conversations threads reply 67890 \
  --customer user@example.com \
  --body "Thanks for reaching out!" \
  --status closed \
  --user-id 999 \
  --to "recipient@example.com" \
  --cc "cc@example.com" \
  --bcc "bcc@example.com" \
  --draft \
  --imported \
  --created-at "2026-01-01T00:00:00Z" \
  --type email \
  --attachment-id 1,2

# Add an internal note
hs inbox conversations threads note 67890 \
  --body "Internal note about this ticket" \
  --user-id 999 \
  --status pending \
  --attachment-id 4,5

# Create chat/customer/phone threads
hs inbox conversations threads create-chat 67890 --customer user@example.com --body "Live chat transcript"
hs inbox conversations threads create-customer 67890 --customer user@example.com --body "Customer follow-up" --imported --created-at "2026-01-01T00:00:00Z"
hs inbox conversations threads create-phone 67890 --body "Phone call summary" --attachment-id 3,4

# Update a thread
hs inbox conversations threads update 67890 123 --text "Updated text" --status closed

# Get original source
hs inbox conversations threads source 67890 123 --format json
hs inbox conversations threads source-rfc822 67890 123
```

#### threads reply flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--customer` | string | yes | Customer email |
| `--body` | string | yes | Reply body |
| `--status` | string | | Set conversation status |
| `--user-id` | int | | Author user ID |
| `--to` | strings | | Recipient emails |
| `--cc` | strings | | CC emails |
| `--bcc` | strings | | BCC emails |
| `--draft` | bool | | Create as draft |
| `--imported` | bool | | Mark as imported |
| `--created-at` | string | | Thread creation timestamp |
| `--type` | string | | Thread type (e.g. email) |
| `--attachment-id` | ints | | Attachment IDs (comma-separated) |

#### threads note flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--body` | string | yes | Note body |
| `--user-id` | int | | Author user ID |
| `--status` | string | | Set conversation status |
| `--attachment-id` | ints | | Attachment IDs (comma-separated) |

#### threads create-chat / create-customer / create-phone flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--body` | string | yes | Thread body |
| `--customer` | string | | Customer email |
| `--imported` | bool | | Mark as imported |
| `--created-at` | string | | Thread creation timestamp |
| `--attachment-id` | ints | | Attachment IDs (comma-separated) |

#### threads update flags

| Flag | Type | Description |
|------|------|-------------|
| `--text` | string | Updated body text |
| `--status` | string | Updated status |

### Customers

```bash
# List customers
hs inbox customers list
hs inbox cust list               # alias

# Search customers
hs inbox customers list --query "alice@example.com"

# Advanced filters
hs inbox customers list --mailbox 12345 --first-name Alice --last-name Smith
hs inbox customers list --modified-since "2026-01-01T00:00:00Z"
hs inbox customers list --sort-field firstName --sort-order asc

# Get customer details
hs inbox customers get 11111

# Create a customer
hs inbox customers create \
  --first-name Alice \
  --last-name Smith \
  --email alice@example.com \
  --phone "555-1234" \
  --job-title "Engineer" \
  --location "Paris" \
  --gender female \
  --background "VIP customer" \
  --age "35" \
  --photo-url "https://example.com/photo.jpg" \
  --organization-id 42

# Create with raw JSON (overrides all flags, supports nested objects)
hs inbox customers create \
  --json '{"firstName":"Alice","emails":[{"type":"work","value":"alice@example.com"}]}'

# Update a customer
hs inbox customers update 11111 --first-name "Alicia"

# Overwrite a customer (same flags as create)
hs inbox customers overwrite 11111 --first-name "Alice"

# Delete a customer
hs inbox customers delete 11111

# Delete asynchronously (returns 202)
hs inbox customers delete 11111 --async
```

#### customers list flags

| Flag | Type | Description |
|------|------|-------------|
| `--query` | string | Search query (e.g. email) |
| `--mailbox` | string | Filter by mailbox ID |
| `--first-name` | string | Filter by first name |
| `--last-name` | string | Filter by last name |
| `--modified-since` | string | Filter by last-modified timestamp |
| `--sort-field` | string | Sort field |
| `--sort-order` | string | Sort direction |
| `--embed` | string | Embed sub-resources _(deprecated)_ |

#### customers get flags

| Flag | Type | Description |
|------|------|-------------|
| `--embed` | string | Embed sub-resources _(deprecated)_ |

#### customers create flags

Also used by `overwrite`.

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--first-name` | string | yes | First name |
| `--last-name` | string | | Last name |
| `--email` | string | | Email address |
| `--phone` | string | | Phone number |
| `--job-title` | string | | Job title |
| `--background` | string | | Background info |
| `--location` | string | | Location |
| `--gender` | string | | Gender: male, female, unknown |
| `--age` | string | | Age |
| `--photo-url` | string | | Photo URL |
| `--organization-id` | int | | Organization ID |
| `--json` | string | | Raw JSON body (overrides all flags) |

#### customers update flags

| Flag | Type | Description |
|------|------|-------------|
| `--first-name` | string | First name |
| `--last-name` | string | Last name |
| `--phone` | string | Phone number |

#### customers delete flags

| Flag | Type | Description |
|------|------|-------------|
| `--async` | bool | Delete asynchronously (returns 202) |

### Tags

```bash
# List tags
hs inbox tags list

# Get tag details
hs inbox tags get 123
```

### Users

```bash
# List users
hs inbox users list
hs inbox users list --email agent@example.com --mailbox 12345

# Get user details
hs inbox users get 22222
hs inbox users me
hs inbox users delete 22222

# User statuses
hs inbox users status list
hs inbox users status get 22222
hs inbox users status set 22222 --status away
hs inbox users status set 22222 --json '{"status":"away","autoReply":"I am out of office"}'
```

#### users list flags

| Flag | Type | Description |
|------|------|-------------|
| `--email` | string | Filter by email |
| `--mailbox` | string | Filter by mailbox ID |

#### users status set flags

| Flag | Type | Description |
|------|------|-------------|
| `--status` | string | Status value |
| `--json` | string | Full status payload as JSON |

### Teams

```bash
# List teams and team members
hs inbox teams list
hs inbox teams members 77
```

### Organizations

```bash
# Organization CRUD
hs inbox organizations list
hs inbox organizations list --query "acme"
hs inbox organizations get 12
hs inbox organizations create --name "Acme" --domain "acme.com"
hs inbox organizations create --json '{"name":"Acme","domains":["acme.com","acme.io"]}'
hs inbox organizations update 12 --name "Acme Intl"
hs inbox organizations delete 12

# Related resources
hs inbox organizations conversations list 12
hs inbox organizations customers list 12

# Organization properties
hs inbox organizations properties list
hs inbox organizations properties get 3
hs inbox organizations properties create --name "Tier" --type "text"
hs inbox organizations properties update 3 --name "Tier 2"
hs inbox organizations properties delete 3
```

#### organizations list flags

| Flag | Type | Description |
|------|------|-------------|
| `--query` | string | Search query |

#### organizations create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--name` | string | yes | Organization name |
| `--domain` | string | | Organization domain |
| `--json` | string | | Full payload as JSON (overrides flags) |

#### organizations update flags

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Organization name |
| `--domain` | string | Organization domain |
| `--json` | string | Full payload as JSON (overrides flags) |

#### organizations properties create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--name` | string | yes | Property name |
| `--type` | string | | Property type |
| `--json` | string | | Full payload as JSON (overrides flags) |

#### organizations properties update flags

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Property name |
| `--type` | string | Property type |
| `--json` | string | Full payload as JSON (overrides flags) |

### Properties

```bash
# Customer property definitions
hs inbox properties customers list
hs inbox properties customers get 5

# Conversation property definitions
hs inbox properties conversations list
hs inbox properties conversations get 8
```

### Ratings

```bash
hs inbox ratings get 7
```

### Reports

```bash
# One command per Inbox report family
hs inbox reports chats --start 2026-01-01 --end 2026-01-31
hs inbox reports company --start 2026-01-01 --end 2026-01-31
hs inbox reports conversations --start 2026-01-01 --end 2026-01-31
hs inbox reports customers --start 2026-01-01 --end 2026-01-31
hs inbox reports docs --start 2026-01-01 --end 2026-01-31
hs inbox reports email --start 2026-01-01 --end 2026-01-31
hs inbox reports productivity --start 2026-01-01 --end 2026-01-31
hs inbox reports ratings --start 2026-01-01 --end 2026-01-31
hs inbox reports users --start 2026-01-01 --end 2026-01-31

# Filter by mailbox or view
hs inbox reports conversations --start 2026-01-01 --end 2026-01-31 --mailbox 12345 --view tags

# Pass additional query params
hs inbox reports company --param "view=responses" --param "granularity=day"
```

#### reports flags

Shared across all report families.

| Flag | Type | Description |
|------|------|-------------|
| `--start` | string | Report start date/time |
| `--end` | string | Report end date/time |
| `--mailbox` | string | Filter by mailbox ID |
| `--view` | string | Report view filter |
| `--param` | strings | Additional params as `key=value` (repeatable) |

### Workflows

```bash
# List workflows
hs inbox workflows list
hs inbox wf list                 # alias
hs inbox workflows list --mailbox-id 12345 --type manual

# Activate or deactivate a workflow
hs inbox workflows update-status 33333 --status inactive
hs inbox workflows update-status 33333 --status active

# Run a workflow on specific conversations
hs inbox workflows run 33333 --conversation-ids 100,200,300
```

#### workflows list flags

| Flag | Type | Description |
|------|------|-------------|
| `--mailbox-id` | int | Filter by mailbox ID |
| `--type` | string | Filter by workflow type |

#### workflows update-status flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--status` | string | yes | Status: active or inactive |

#### workflows run flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--conversation-ids` | strings | yes | Conversation IDs (comma-separated) |

### Webhooks

```bash
# List webhooks
hs inbox webhooks list
hs inbox wh list                 # alias

# Get webhook details
hs inbox webhooks get 44444

# Create a webhook
hs inbox webhooks create \
  --url https://example.com/hook \
  --events "convo.created,convo.updated" \
  --secret my-webhook-secret \
  --payload-version V2 \
  --mailbox-ids 12345,67890 \
  --notification \
  --label "Primary Hook"

# Update a webhook
hs inbox webhooks update 44444 \
  --url https://example.com/new-hook \
  --payload-version V1 \
  --mailbox-ids 12345 \
  --notification=false

# Delete a webhook
hs inbox webhooks delete 44444
```

#### webhooks create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--url` | string | yes | Webhook URL |
| `--events` | strings | yes | Events to subscribe to (comma-separated) |
| `--secret` | string | yes | Webhook secret |
| `--payload-version` | string | | Payload version: V1, V2 |
| `--mailbox-ids` | ints | | Mailbox IDs to scope (comma-separated) |
| `--notification` | bool | | Send lightweight notification payloads |
| `--label` | string | | Human-readable label |

#### webhooks update flags

Same flags as create, none required.

### Saved Replies

```bash
# List and get saved replies
hs inbox saved-replies list --mailbox-id 12345
hs inbox saved-replies list --query "welcome"
hs inbox saved-replies get 44

# Create a saved reply
hs inbox saved-replies create --mailbox-id 12345 --name "Welcome" --body "Hi there" --subject "Welcome!" --private

# Update a saved reply
hs inbox saved-replies update 44 --name "Welcome v2" --body "Updated greeting" --subject "Welcome back!"

# Delete
hs inbox saved-replies delete 44
```

#### saved-replies list flags

| Flag | Type | Description |
|------|------|-------------|
| `--mailbox-id` | string | Filter by mailbox ID |
| `--query` | string | Search query |

#### saved-replies create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--mailbox-id` | string | yes | Mailbox ID |
| `--name` | string | yes | Reply name |
| `--body` | string | yes | Reply body text |
| `--subject` | string | | Reply subject |
| `--private` | bool | | Mark as private |
| `--json` | string | | Full payload as JSON (overrides flags) |

#### saved-replies update flags

| Flag | Type | Description |
|------|------|-------------|
| `--mailbox-id` | string | Mailbox ID |
| `--name` | string | Reply name |
| `--body` | string | Reply body text |
| `--subject` | string | Reply subject |
| `--private` | bool | Mark as private |
| `--json` | string | Full payload as JSON (overrides flags) |

### Tools

Workflow commands that aggregate data from the Inbox API. Namespaced under `hs inbox tools ...`.

```bash
# Team overview — conversation counts per agent
hs inbox tools briefing

# Filter by status
hs inbox tools briefing --status pending
hs inbox tools briefing --status all

# Agent summary — list conversations for a specific user
hs inbox tools briefing --assigned-to 531600

# Agent briefing with full thread data
hs inbox tools briefing --assigned-to 531600 --embed threads

# JSON output (clean — no HAL _embedded/_links)
hs inbox tools briefing --assigned-to 531600 --embed threads --format json
```

#### briefing flags

| Flag | Type | Description |
|------|------|-------------|
| `--assigned-to` | string | Filter by assigned user ID |
| `--status` | string | Conversation status filter (default: `active`) |
| `--embed` | string | Embed sub-resources (e.g. `threads`, requires `--assigned-to`) |

#### briefing output modes

| Mode | Command | Description |
|------|---------|-------------|
| Team overview | `briefing` | Agent names, emails, conversation counts |
| Agent summary | `briefing --assigned-to <id>` | Conversation list (same columns as `conversations list`) |
| Agent + threads | `briefing --assigned-to <id> --embed threads` | Conversation list with thread count and last activity |

When `--format json` is used with `--embed threads`, the output is a JSON array where each conversation has a top-level `threads` array with full thread data. HAL envelope (`_embedded`, `_links`) is stripped from both conversation and thread objects.

### Shell completion

```bash
# Bash
hs completion bash > /etc/bash_completion.d/helpscout

# Zsh
hs completion zsh > "${fpath[1]}/_helpscout"

# Fish
hs completion fish > ~/.config/fish/completions/helpscout.fish

# PowerShell
hs completion powershell | Out-String | Invoke-Expression
```

### Version

```bash
hs version
```

## Output formats

### Table (default)

```
ID     NAME       EMAIL               SLUG
─────  ─────────  ──────────────────  ────────
12345  Support    support@acme.com    support
12346  Sales      sales@acme.com      sales

Page 1 of 1 (2 total)
```

### JSON (clean)

```bash
hs inbox mailboxes list --format json
```

Read-optimized output. Compared to the raw API response, `--format json` applies per-resource cleanup:

- Drops HAL noise (`_links`, `_embedded` wrappers)
- Converts HTML bodies to markdown (threads, saved replies)
- Flattens person objects to `"Name (email)"` strings
- Drops sentinel values (`closedBy: 0`, `closedByUser: {id: 0, ...}`)
- Drops empty arrays/strings and default-noise fields (`state: "published"`, `photoUrl`, etc.)
- Hoists embedded sub-resources to top level (e.g. customer `_embedded.emails` → `emails`)
- Renames for clarity (`userUpdatedAt` → `updatedAt`, `threads` count → `threadCount`)

**Caveat:** `--format json` is read-only safe. HTML→markdown conversion and field removal make the output unsuitable as input for write operations (e.g. updating saved replies, composing thread bodies). Use `--format json-full` when you need write-safe data.

### JSON-full (raw)

```bash
hs inbox mailboxes list --format json-full
```

Unfiltered API pass-through. Identical to the raw HelpScout API response, pretty-printed. Use this for debugging, round-tripping data back into write operations, or when you need the complete response including HAL links and embedded wrappers.

### CSV

```bash
hs inbox mailboxes list --format csv
```

Standard RFC 4180 CSV with a header row.

## Pagination

By default, the CLI returns a single page of results (25 items). Use `--page` and `--per-page` to navigate:

```bash
hs inbox conversations list --page 2 --per-page 50
```

Use `--no-paginate` to fetch and combine all pages into a single result set:

```bash
hs inbox conversations list --no-paginate
```

## Configuration

Use `hs inbox config set` to write values, `hs inbox config get` to read them, and `hs inbox config path` to locate the file.

### Config file locations

| OS | Path |
|----|------|
| Linux/macOS | `~/.config/hs/config.yaml` |
| Windows | `%APPDATA%\helpscout\config.yaml` |

### All config options

| Key | Flag | Description |
|-----|------|-------------|
| `inbox_app_id` | `--inbox-app-id` | HelpScout App ID |
| `inbox_app_secret` | `--inbox-app-secret` | HelpScout App Secret |
| `inbox_default_mailbox` | `--inbox-default-mailbox` | Auto-filter conversations to this mailbox |
| `format` | `--format` | Output format: `table`, `json`, `json-full`, or `csv` |
| `inbox_pii_mode` | `--inbox-pii-mode` | PII redaction mode: `off`, `customers`, `all` |
| `inbox_pii_allow_unredacted` | `--inbox-pii-allow-unredacted` | Allow `--unredacted` to bypass redaction per request |
| `docs_api_key` | `--docs-api-key` | HelpScout Docs API key |

### Environment variables

| Variable | Overrides |
|----------|-----------|
| `HS_INBOX_APP_ID` | `inbox_app_id` |
| `HS_INBOX_APP_SECRET` | `inbox_app_secret` |
| `HS_FORMAT` | `format` |
| `HS_INBOX_PII_MODE` | `inbox_pii_mode` |
| `HS_INBOX_PII_ALLOW_UNREDACTED` | `inbox_pii_allow_unredacted` |
| `HS_INBOX_PII_SECRET` | Optional secret salt for deterministic pseudonyms |
| `HS_DOCS_API_KEY` | `docs_api_key` |
| `HS_INBOX_PERMISSIONS` | `inbox_permissions` |
| `HS_NO_UPDATE_CHECK` | Disable daily update check (`1`) |

## Permissions

An allowlist-based permission system that restricts which operations are permitted. When `HS_INBOX_PERMISSIONS` is set, only explicitly granted `resource:operation` pairs are allowed. When unset, everything is allowed (backward compatible).

### Format

Comma-separated `resource:operation` pairs. Wildcards (`*`) supported.

```bash
# Read-only access to conversations and customers
HS_INBOX_PERMISSIONS="conversations:read,customers:read"

# Read-only across all resources
HS_INBOX_PERMISSIONS="*:read"

# Full access to conversations, read-only everything else
HS_INBOX_PERMISSIONS="conversations:*,*:read"

# Unrestricted (explicit)
HS_INBOX_PERMISSIONS="*:*"
```

**Resources**: `conversations`, `customers`, `mailboxes`, `tags`, `users`, `teams`, `organizations`, `properties`, `workflows`, `webhooks`, `saved-replies`, `reports`, `ratings`

**Operations**: `read` (list/get), `write` (create/update/reply/note/run), `delete`

**Source priority**: `HS_INBOX_PERMISSIONS` env var > `inbox_permissions` in config.yaml > unrestricted default

### MCP / LLM setup

Use permissions to restrict what an LLM can do via MCP or shell access:

```json
{
  "env": {
    "HS_INBOX_APP_ID": "...",
    "HS_INBOX_APP_SECRET": "...",
    "HS_INBOX_PERMISSIONS": "conversations:read,customers:read,mailboxes:read,tags:read"
  }
}
```

### Inspect permissions

```bash
# Show current policy, source, and per-command allow/deny table
hs inbox permissions

# With a policy set
HS_INBOX_PERMISSIONS="conversations:read" hs inbox permissions
```

### Denied commands

When a command is denied:

```
Error: permission denied: conversations:write not allowed

Current policy: conversations:read,customers:read
To allow, add conversations:write to HS_INBOX_PERMISSIONS
```

### Namespace scoping

Permissions are scoped per API namespace since each has separate auth/credentials. The `internal/permission` package is namespace-agnostic — it parses and evaluates `resource:operation` strings regardless of which API they apply to. Scoping is a naming convention at the env/config layer.

| Namespace | Env var | Config field | Status |
|-----------|---------|--------------|--------|
| Inbox | `HS_INBOX_PERMISSIONS` | `inbox_permissions` | Implemented |
| Docs | `HS_DOCS_PERMISSIONS` | `docs_permissions` | Planned |

When Docs API support is added, commands will live under `hs docs ...` with separate auth (`HS_DOCS_API_KEY`) and separate permission scoping.

---

## Developer guide

### Prerequisites

- Go 1.25+

### Project structure

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

### Build

```bash
go build -o build/hs ./cmd/hs
```

With version info:

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o build/helpscout ./cmd/helpscout
```

### Run tests

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

### Test architecture

- **api package**: Uses `httptest.Server` with a URL-rewriting transport to test real HTTP round-trips without hitting the HelpScout API
- **cmd package**: Uses a `mockClient` implementing `ClientAPI` with function fields. Global state (`apiClient`, `cfg`, `output.Out`) is swapped per-test. Not `t.Parallel()` safe due to global mutation. E2E tests use `isolateHome` to sandbox HOME/config dirs
- **config package**: Uses `t.TempDir()` for filesystem tests and `t.Setenv()` for env var tests
- **output package**: Formatters write to `bytes.Buffer` for assertion
- **selfupdate package**: Uses `httptest.Server` for GitHub API mocking, `DirOverride`/`InstallDirOverride` for filesystem isolation

### Release

Releases are automated via GitHub Actions. Push a `v*` tag to trigger a draft release with platform binaries:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Create a HelpScout App with Client ID & Client Secret
This is *not* via the `Manage` -> `Apps` flow.

1. Select your profile icon in the top right
2. Then "Your Profile"
3. Select "My Apps" in the left bar.
4. "Create App"
5. Enter an App Name - "Agent CLI"
6. Then a redirection URL (not needed) use a valid `https` url - "https://mysite.com"
7. You have generated an App ID and App Secret - use these in the `hs inbox auth login` command
8. Result should be: `Validating credentials... Authenticated. Found 1 mailboxes.`

## License

[MIT](LICENSE)
