# Inbox API Reference

Inbox API commands are namespaced under `hs inbox ...`. Uses OAuth2 client credentials for authentication.

## Authentication

The CLI uses HelpScout's OAuth2 client credentials flow. You'll need an App ID and App Secret from your HelpScout app settings (**Your Profile** > My Apps > Create App).

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

### Check status

```bash
hs inbox auth status
```

### Logout

```bash
hs inbox auth logout
```

## Environment variables

| Variable | Description |
|----------|-------------|
| `HS_INBOX_APP_ID` | HelpScout App ID (overrides `inbox_app_id`) |
| `HS_INBOX_APP_SECRET` | HelpScout App Secret (overrides `inbox_app_secret`) |
| `HS_FORMAT` | Output format (overrides `format`) |
| `HS_INBOX_PII_MODE` | PII redaction mode (overrides `inbox_pii_mode`) |
| `HS_INBOX_PII_ALLOW_UNREDACTED` | Allow `--unredacted` bypass (overrides `inbox_pii_allow_unredacted`) |
| `HS_INBOX_PII_SECRET` | Optional secret salt for deterministic pseudonyms |
| `HS_INBOX_PERMISSIONS` | Inbox permission policy |
| `HS_NO_UPDATE_CHECK` | Disable daily update check (`1`) |

## Permissions

An allowlist-based permission system that restricts which Inbox operations are permitted. When `HS_INBOX_PERMISSIONS` is set, only explicitly granted `resource:operation` pairs are allowed. When unset, everything is allowed (backward compatible).

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

## Commands

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
