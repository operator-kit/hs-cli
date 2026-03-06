# Docs API Reference

Docs API commands are namespaced under `hs docs ...`. Uses a Docs API key for authentication.

## Authentication

```bash
# Authenticate with Docs API key
hs docs auth login

# Check authentication status
hs docs auth status

# Remove stored Docs API key
hs docs auth logout
```

**Credential resolution order:** `HS_DOCS_API_KEY` env var > OS keyring > config file (`docs_api_key`).

## Environment variables

| Variable | Description |
|----------|-------------|
| `HS_DOCS_API_KEY` | Docs API key (overrides `docs_api_key`) |
| `HS_DOCS_PERMISSIONS` | Docs permission policy |

## Permissions

Same mechanism as Inbox permissions. When `HS_DOCS_PERMISSIONS` is set, only explicitly granted `resource:operation` pairs are allowed. When unset, everything is allowed.

### Format

Comma-separated `resource:operation` pairs. Wildcards (`*`) supported.

```bash
# Read-only access to articles and collections
HS_DOCS_PERMISSIONS="articles:read,collections:read"

# Read-only across all Docs resources
HS_DOCS_PERMISSIONS="*:read"

# Full access to articles, read-only everything else
HS_DOCS_PERMISSIONS="articles:*,*:read"
```

**Resources**: `sites`, `collections`, `categories`, `articles`, `redirects`, `assets`

**Operations**: `read` (list/get), `write` (create/update/upload), `delete`

**Source priority**: `HS_DOCS_PERMISSIONS` env var > `docs_permissions` in config.yaml > unrestricted default

## Commands

### Sites

```bash
# List sites
hs docs sites list

# Get site details
hs docs sites get <id>

# Create a site
hs docs sites create --subdomain mysite --title "My Site" --color "#ff0000"

# Update a site
hs docs sites update <id> --title "New Title" --status active

# Delete a site
hs docs sites delete <id>

# Get site restrictions
hs docs sites restrictions get <site-id>

# Update site restrictions
hs docs sites restrictions update <site-id> --emails user@example.com --domains example.com
```

#### sites create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--subdomain` | string | yes | Subdomain |
| `--title` | string | yes | Site title |
| `--status` | string | | Status |
| `--cname` | string | | Custom CNAME |
| `--has-public-site` | bool | | Has public site |
| `--logo-url` | string | | Logo URL |
| `--favicon-url` | string | | Favicon URL |
| `--color` | string | | Primary color |
| `--contact-email` | string | | Contact email |

#### sites update flags

| Flag | Type | Description |
|------|------|-------------|
| `--subdomain` | string | Subdomain |
| `--title` | string | Site title |
| `--status` | string | Status |
| `--cname` | string | Custom CNAME |
| `--has-public-site` | bool | Has public site |
| `--logo-url` | string | Logo URL |
| `--favicon-url` | string | Favicon URL |
| `--color` | string | Primary color |
| `--contact-email` | string | Contact email |

#### sites restrictions update flags

| Flag | Type | Description |
|------|------|-------------|
| `--emails` | strings | Allowed email addresses (comma-separated) |
| `--domains` | strings | Allowed domains (comma-separated) |

### Collections

```bash
# List collections
hs docs collections list
hs docs collections list --site <site-id> --visibility public

# Get collection details
hs docs collections get <id>

# Create a collection
hs docs collections create --site <site-id> --name "Getting Started" --visibility public

# Update a collection
hs docs collections update <id> --name "New Name" --description "Updated description"

# Delete a collection
hs docs collections delete <id>
```

#### collections list flags

| Flag | Type | Description |
|------|------|-------------|
| `--site` | string | Filter by site ID |
| `--visibility` | string | Filter by visibility (public\|private) |
| `--sort` | string | Sort field |
| `--order` | string | Sort order (asc\|desc) |

#### collections create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--site` | string | yes | Site ID |
| `--name` | string | yes | Collection name |
| `--visibility` | string | | Visibility (public\|private) |
| `--order` | int | | Display order |
| `--description` | string | | Collection description |

#### collections update flags

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Collection name |
| `--visibility` | string | Visibility (public\|private) |
| `--order` | int | Display order |
| `--description` | string | Collection description |

### Categories

```bash
# List categories in a collection
hs docs categories list <collection-id>

# Get category details
hs docs categories get <id>

# Create a category
hs docs categories create --collection <collection-id> --name "FAQs"

# Update a category
hs docs categories update <id> --name "General FAQs" --visibility private

# Reorder categories in a collection
hs docs categories reorder <collection-id> --categories id1,id2,id3

# Delete a category
hs docs categories delete <id>
```

#### categories create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--collection` | string | yes | Collection ID |
| `--name` | string | yes | Category name |
| `--slug` | string | | URL slug |
| `--visibility` | string | | Visibility (public\|private) |
| `--order` | int | | Display order |
| `--default-sort` | string | | Default sort (name\|number\|popularity\|manual) |

#### categories update flags

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Category name |
| `--slug` | string | URL slug |
| `--visibility` | string | Visibility (public\|private) |
| `--order` | int | Display order |
| `--default-sort` | string | Default sort (name\|number\|popularity\|manual) |

#### categories reorder flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--categories` | strings | yes | Ordered category IDs (comma-separated) |

### Articles

```bash
# List articles by collection or category
hs docs articles list --collection <collection-id>
hs docs articles list --category <category-id>
hs docs articles list --collection <collection-id> --status published

# Search articles
hs docs articles search --query "password reset"
hs docs articles search --query "billing" --collection <collection-id> --status published

# Get article details
hs docs articles get <id>
hs docs articles get <id> --draft

# List related articles
hs docs articles related <id>

# Create an article
hs docs articles create \
  --collection <collection-id> \
  --name "How to reset your password" \
  --text "<p>Follow these steps...</p>" \
  --status published \
  --categories cat1,cat2 \
  --keywords "password,reset,login"

# Update an article
hs docs articles update <id> --name "Updated title" --status notpublished

# Delete an article
hs docs articles delete <id>

# Upload an asset to an article
hs docs articles upload <id> --file ./screenshot.png

# Update article view count
hs docs articles views <id> --count 500

# Save a draft
hs docs articles draft save <article-id> --text "<p>Draft content</p>"

# Delete a draft
hs docs articles draft delete <article-id>

# List article revisions
hs docs articles revisions list <article-id>

# Get a specific revision
hs docs articles revisions get <article-id> <revision-id>
```

#### articles list flags

| Flag | Type | Description |
|------|------|-------------|
| `--collection` | string | Collection ID (required if no `--category`) |
| `--category` | string | Category ID (required if no `--collection`) |
| `--status` | string | Filter by status (published\|notpublished\|all) |
| `--sort` | string | Sort field |
| `--order` | string | Sort order (asc\|desc) |

#### articles search flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--query` | string | yes | Search query |
| `--collection` | string | | Filter by collection ID |
| `--site` | string | | Filter by site ID |
| `--status` | string | | Filter by status |
| `--visibility` | string | | Filter by visibility |

#### articles get flags

| Flag | Type | Description |
|------|------|-------------|
| `--draft` | bool | Retrieve draft version |

#### articles create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--collection` | string | yes | Collection ID |
| `--name` | string | yes | Article title |
| `--text` | string | yes | Article body HTML |
| `--status` | string | | Status (published\|notpublished) |
| `--slug` | string | | URL slug |
| `--categories` | strings | | Category IDs (comma-separated) |
| `--related` | strings | | Related article IDs (comma-separated) |
| `--keywords` | strings | | SEO keywords (comma-separated) |

#### articles update flags

| Flag | Type | Description |
|------|------|-------------|
| `--name` | string | Article title |
| `--text` | string | Article body HTML |
| `--status` | string | Status (published\|notpublished) |
| `--slug` | string | URL slug |
| `--categories` | strings | Category IDs (comma-separated) |
| `--related` | strings | Related article IDs (comma-separated) |
| `--keywords` | strings | SEO keywords (comma-separated) |

#### articles upload flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--file` | string | yes | File path to upload |

#### articles views flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--count` | int | yes | View count to set |

#### articles draft save flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--text` | string | yes | Draft body HTML |

### Redirects

```bash
# List redirects for a site
hs docs redirects list <site-id>

# Get redirect details
hs docs redirects get <id>

# Find a redirect by site and URL
hs docs redirects find --site <site-id> --url /old-path

# Create a redirect
hs docs redirects create --site <site-id> --url-mapping /old-path --redirect /new-path

# Update a redirect
hs docs redirects update <id> --redirect /updated-path

# Delete a redirect
hs docs redirects delete <id>
```

#### redirects find flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--site` | string | yes | Site ID |
| `--url` | string | yes | URL to find redirect for |

#### redirects create flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--site` | string | yes | Site ID |
| `--url-mapping` | string | yes | Source URL path |
| `--redirect` | string | yes | Destination URL path |

#### redirects update flags

| Flag | Type | Description |
|------|------|-------------|
| `--url-mapping` | string | Source URL path |
| `--redirect` | string | Destination URL path |

### Assets

```bash
# Upload an article asset
hs docs assets article upload --file ./image.png

# Upload a settings asset (logo, favicon, etc.)
hs docs assets settings upload --file ./logo.png
```

#### assets article upload flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--file` | string | yes | File path to upload |

#### assets settings upload flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--file` | string | yes | File path to upload |
