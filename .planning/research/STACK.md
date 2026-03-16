# Technology Stack

**Project:** Mattermost Command Center
**Researched:** 2026-03-16

## Recommended Stack

### Core: Mattermost Plugin (Server - Go)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.22+ | Plugin server language | Mattermost plugins must be Go -- not optional. Use 1.22+ for range-over-func and improved standard library. Local system has 1.25.5 available. Mattermost dev docs require Go 1.21+; 1.22 is safe. | HIGH |
| `github.com/mattermost/mattermost/server/public` | latest (v0.2.x) | Plugin SDK (plugin + model packages) | Current canonical import path since Mattermost v10+. Replaces the old `mattermost-server/v6` path. Contains `plugin.MattermostPlugin`, `plugin.API`, all `model.*` types (Post, Channel, User, CommandArgs, etc.). Published Mar 10, 2026. | HIGH |
| `github.com/mattermost/mattermost-plugin-api` | latest (v0.1.x) | Higher-level plugin API wrapper | Provides service-organized methods (`client.Post.Create`, `client.KV.Set`, `client.Bot.EnsureBot`, `client.SlashCommand.Register`), proper Go `error` returns instead of `*model.AppError`, functional options pattern for KV operations. Vastly better DX than raw API. | HIGH |
| `github.com/gorilla/mux` | v1.8+ | HTTP routing | Needed for `ServeHTTP` hook to cleanly route inbound webhook and action handler HTTP requests. Standard pattern in official Mattermost plugins (Jira, GitHub, ServiceNow). | HIGH |
| Plugin Starter Template | master | Project scaffold | Official template with Makefile, manifest, server/webapp structure, deploy scripts, golangci-lint config. Clone and customize. Replace module path in go.mod. | HIGH |

### Core: Mattermost Plugin (Webapp - TypeScript/React)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| React | (Mattermost-bundled) | UI components | Mattermost bundles React; plugins declare it as webpack external. Do NOT bundle your own React -- it causes version conflicts. | HIGH |
| TypeScript | 5.x | Type safety for webapp | Supported by the plugin build system. Interactive dialogs and post type components benefit from type safety. | MEDIUM |
| Webpack | 5.x | Bundle webapp plugin | Standard build tool for Mattermost webapp plugins. Configured in the starter template with babel-loader, @babel/preset-react, @babel/preset-env. | HIGH |

**Note on webapp scope:** For this plugin, the webapp component is minimal. It registers post dropdown menu actions ("Create Task in Plane", "Save to Obsidian") and potentially a channel header button. Most logic lives server-side. The webapp is a thin registration layer, not a full React app.

### External API Clients (Go)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `net/http` (stdlib) | Go stdlib | HTTP client for Plane and Obsidian APIs | Both Plane and Obsidian REST APIs are simple REST/JSON. No Go SDK exists for either. Standard library `net/http` with custom client wrappers is the right approach -- no heavy third-party HTTP clients needed. | HIGH |
| `encoding/json` (stdlib) | Go stdlib | JSON marshal/unmarshal | Define Go structs matching API response shapes. Use `json:"fieldName,omitempty"` tags. | HIGH |
| `crypto/hmac` + `crypto/sha256` (stdlib) | Go stdlib | Plane webhook signature verification | Plane sends HMAC-SHA256 signatures in `X-Plane-Signature` header. Standard library crypto handles this. | HIGH |
| `crypto/tls` (stdlib) | Go stdlib | Obsidian TLS skip-verify | Obsidian Local REST API uses self-signed certificates. Configure `tls.Config{InsecureSkipVerify: true}` specifically for Obsidian HTTP client instances. | HIGH |

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `testing` (stdlib) | Go stdlib | Unit tests | Standard Go testing. Table-driven tests for command parsing and API client methods. | HIGH |
| `github.com/stretchr/testify` | v1.9+ | Assertions and mocks | Industry standard for Go. Mattermost's own `plugintest` package is built on testify mocks. Use `assert`, `require`, and `mock` subpackages. | HIGH |
| `plugintest` (Mattermost SDK) | (bundled with server/public) | Mock plugin API | Official mock implementations for `plugin.API`. Pattern: `api := &plugintest.API{}; api.On("GetUser", id).Return(user, nil)`. | HIGH |
| `net/http/httptest` (stdlib) | Go stdlib | Test HTTP handlers | Test the plugin's `ServeHTTP` hook, webhook receivers, and action handler endpoints. | HIGH |

### Build & Deploy

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Make | system | Build automation | Starter template uses Makefile for: `make build` (compile), `make dist` (package .tar.gz), `make deploy` (upload to server), `make test`, `make check-style`. Follow convention. | HIGH |
| `golangci-lint` | latest | Go linting | Configured in starter template via `.golangci.yml`. Catches common Go issues. Includes goimports, unused, errcheck, staticcheck. | HIGH |
| ESLint | (template default) | JS/TS linting | For webapp plugin code. Template includes basic configuration. | MEDIUM |

### Configuration & Storage

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Mattermost KV Store | (plugin API) | Persist all plugin state | Built-in key-value store scoped per plugin. Use for: channel-to-project mappings, user API token storage (encrypted), webhook dedup, cached data with TTL. No external database needed at 2-10 users. Key limit: 50 chars. Value limit: per-key size limit exists (check docs for exact number). | HIGH |
| Mattermost System Console | (plugin manifest) | Admin configuration UI | Define `settings_schema` in `plugin.json` for admin-configurable settings. Supports `text`, `bool`, `dropdown`, `radio`, `generated` types. Fields marked `secret: true` are masked in UI. | HIGH |
| Bot Account | (plugin API) | Post notifications and ephemeral messages | Created via `client.Bot.EnsureBot()` in OnActivate. Bot posts task confirmations, webhook notifications, and error messages. Uses a dedicated bot identity (not system message). | HIGH |

## Architecture Decision: No Separate Bridge Service

The PROJECT.md mentions a "servicio bridge" as a pending decision. **Recommendation: Do NOT build a separate bridge service.** Keep everything inside the Mattermost plugin.

**Rationale:**
- The Mattermost plugin can make outbound HTTP requests to Plane and Obsidian directly via `net/http`
- The plugin's `ServeHTTP` hook receives external HTTP at `/plugins/{plugin_id}/webhook/plane`
- A separate service adds deployment complexity, network configuration, and a second codebase for zero benefit at this scale
- For 2-10 users, the single plugin process handles all load easily
- Obsidian connections are user-specific (each user configures their endpoint in KV store)

**When a bridge WOULD make sense:** If the plugin needed persistent connections, heavy background workers, or if Mattermost plugin sandboxing blocked outbound HTTP. None of these apply.

**Confidence:** HIGH -- this pattern is used by all official Mattermost integration plugins (GitHub, Jira, GitLab, ServiceNow) that connect to external APIs.

## External APIs Summary

### Plane REST API v1

| Aspect | Detail |
|--------|--------|
| Base URL | `https://{your-plane-domain}/api/v1/` (self-hosted URL varies) |
| Auth | `X-API-Key: {key}` header (Personal Access Tokens from Profile Settings) |
| Alt Auth | `Authorization: Bearer {oauth-token}` (OAuth2 for user-authorized apps) |
| Rate limit | 60 requests/minute per client. Headers: `X-RateLimit-Remaining`, `X-RateLimit-Reset` |
| **Key endpoints** | |
| List projects | `GET /workspaces/{slug}/projects/` -- scope: `projects:read` |
| List work items | `GET /workspaces/{slug}/projects/{id}/work-items/?state={id}&assignee={id}&expand=state,assignees,labels` |
| Create work item | `POST /workspaces/{slug}/projects/{id}/work-items/` -- scope: `projects.work_items:write` |
| Get work item | `GET /workspaces/{slug}/projects/{id}/work-items/{id}` |
| Pagination | `?limit=N&offset=N` |
| Expand param | `?expand=type,module,labels,assignees,state,project` -- reduces API calls |
| **Webhooks** | HTTP POST callbacks for project, issue, cycle, module, comment events |
| Webhook headers | `X-Plane-Delivery` (UUID), `X-Plane-Event`, `X-Plane-Signature` (HMAC-SHA256) |
| Webhook payload | `{event, action, webhook_id, workspace_id, data}` |
| **CRITICAL** | `/issues/` endpoints deprecated March 31, 2026. Use `/work-items/` exclusively from day one. |

### Obsidian Local REST API

| Aspect | Detail |
|--------|--------|
| Default ports | HTTPS: 27124 (self-signed cert), HTTP: 27123 (optional, enable in settings) |
| Auth | `Authorization: Bearer {api-key}` -- 64-char hex string, auto-generated on first load |
| Auth header name | Configurable via plugin settings (default: `Authorization`) |
| SSL caveat | Self-signed cert causes TLS errors. Either use HTTP (27123) or set `InsecureSkipVerify: true` |
| **Key endpoints** | |
| Create/replace note | `PUT /vault/{path}` (Content-Type: `text/markdown`) |
| Append to note | `POST /vault/{path}` |
| Read note | `GET /vault/{path}` |
| Delete note | `DELETE /vault/{path}` |
| List vault | `GET /vault/` |
| Simple search | `POST /search/simple/{query}` |
| Dataview query | `POST /search/` (Content-Type: `text/vnd.dataview.dql`) |
| Status check | `GET /` (no auth required -- use for connectivity test) |
| **Content types** | `text/markdown` (default), `application/json`, `application/vnd.olrapi.note+json` (with metadata) |
| Network constraint | Each user runs their own instance. Plugin stores per-user endpoint + API key in KV store. |
| Availability | Only works when Obsidian desktop app is running with the plugin enabled |
| Latest version | v3.4.6 (March 7, 2026) |

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Plugin language | Go (Mattermost plugin) | Python/Node webhook bot | Plugins provide post action buttons, slash commands, dialogs, KV store, admin UI. A webhook bot only gets incoming webhooks -- no rich UI integration. |
| Architecture | Plugin-only (no bridge) | Go microservice bridge | Adds deployment complexity, network config, second codebase. Plugin does outbound HTTP and receives webhooks natively via ServeHTTP. |
| Plane Go SDK | Custom HTTP client | None exists | No official Go SDK for Plane. Thin client wrappers around `net/http` with typed request/response structs. Straightforward. |
| Obsidian Go SDK | Custom HTTP client | None exists | No official Go SDK for Obsidian Local REST API. Simple REST endpoints -- wrapper is ~200 lines of code. |
| Plugin API style | `mattermost-plugin-api` wrapper | Raw `plugin.API` | Raw API returns `*model.AppError` and has flat method namespace. Wrapper provides service organization (`client.Post.Create`), proper Go errors, functional options. |
| Webapp framework | React (Mattermost-provided) | Custom framework | Cannot choose -- Mattermost webapp plugins must use the bundled React. |
| Database | Mattermost KV Store | SQLite/PostgreSQL | KV store is free, built-in, no deployment burden. For key-value mappings and config at 2-10 users, it is sufficient. Plugin API exposes `Store` service for direct DB access if ever needed. |
| HTTP router | gorilla/mux | stdlib ServeMux | Mux provides cleaner routing with method matching, path variables. All official plugins use it. Minimal dependency. |
| Task template format | Markdown strings in config | YAML/JSON config files | Plugin config lives in manifest settings_schema and KV store. Keep task templates as markdown format strings. |

## Installation

```bash
# Clone the starter template
git clone https://github.com/mattermost/mattermost-plugin-starter-template.git mattermost-plugin-command-center
cd mattermost-plugin-command-center

# Update go.mod module path
# Replace: module github.com/mattermost/mattermost-plugin-starter-template
# With:    module github.com/klab/mattermost-plugin-command-center

# Also update .golangci.yml local-prefixes

# Key Go dependencies (in go.mod)
# require (
#     github.com/mattermost/mattermost/server/public v0.2.x
#     github.com/mattermost/mattermost-plugin-api v0.1.x
#     github.com/gorilla/mux v1.8.x
#     github.com/stretchr/testify v1.9.x
# )

# Build
make dist

# Deploy to Mattermost (with admin credentials configured in env)
# MM_SERVICESETTINGS_SITEURL=https://mattermost.example.com
# MM_ADMIN_TOKEN=your-admin-token
make deploy

# Or manually upload the .tar.gz from dist/ via System Console > Plugins
```

## Plugin Manifest Structure (plugin.json)

```json
{
    "id": "com.klab.command-center",
    "name": "Command Center",
    "description": "Turn Mattermost conversations into Plane tasks and Obsidian notes",
    "version": "0.1.0",
    "min_server_version": "10.0.0",
    "server": {
        "executables": {
            "linux-amd64": "server/dist/plugin-linux-amd64",
            "darwin-amd64": "server/dist/plugin-darwin-amd64",
            "darwin-arm64": "server/dist/plugin-darwin-arm64"
        }
    },
    "webapp": {
        "bundle_path": "webapp/dist/main.js"
    },
    "settings_schema": {
        "header": "Configure the Command Center plugin for Plane and Obsidian integration.",
        "settings": [
            {
                "key": "PlaneBaseURL",
                "display_name": "Plane Base URL",
                "type": "text",
                "help_text": "URL of your self-hosted Plane instance (e.g., https://plane.example.com)",
                "placeholder": "https://plane.example.com"
            },
            {
                "key": "PlaneAPIKey",
                "display_name": "Plane API Key",
                "type": "text",
                "help_text": "Personal Access Token for Plane API. Generate in Plane > Profile > API Tokens.",
                "secret": true
            },
            {
                "key": "PlaneWorkspaceSlug",
                "display_name": "Plane Workspace Slug",
                "type": "text",
                "help_text": "Workspace slug from your Plane URL (e.g., 'my-team' from plane.example.com/my-team)"
            },
            {
                "key": "PlaneWebhookSecret",
                "display_name": "Plane Webhook Secret",
                "type": "text",
                "help_text": "Secret key from Plane webhook configuration, used to verify inbound webhooks.",
                "secret": true
            },
            {
                "key": "EnableObsidian",
                "display_name": "Enable Obsidian Integration",
                "type": "bool",
                "help_text": "Allow users to connect their Obsidian vaults via the Local REST API plugin.",
                "default": true
            }
        ]
    }
}
```

## Key Go Module Paths Reference

```
Plugin interface (MattermostPlugin, Hooks, API):
  github.com/mattermost/mattermost/server/public/plugin

Model types (Post, Channel, User, CommandArgs, PostAction, etc.):
  github.com/mattermost/mattermost/server/public/model

Higher-level API wrapper (service-organized client):
  github.com/mattermost/mattermost-plugin-api  (package: pluginapi)

Test mocks (MockAPI with testify):
  github.com/mattermost/mattermost/server/public/plugin/plugintest

HTTP routing:
  github.com/gorilla/mux
```

## Key Mattermost Plugin Hooks Used

| Hook | Purpose in This Plugin |
|------|----------------------|
| `OnActivate` | Initialize plugin: create bot, register commands, set up HTTP router, initialize API clients |
| `OnDeactivate` | Cleanup: stop background goroutines |
| `OnConfigurationChange` | Reload Plane URL/key/workspace when admin changes settings |
| `ExecuteCommand` | Handle all `/task` slash commands |
| `ServeHTTP` | Route inbound HTTP: Plane webhooks, post action callbacks, dialog submissions |
| `MessageHasBeenPosted` | (Future) Link unfurling when Plane URLs are pasted |

## Sources

- [Mattermost Plugin Quick Start](https://developers.mattermost.com/integrate/plugins/components/server/hello-world/) - HIGH confidence
- [Mattermost Server Plugin SDK Reference](https://developers.mattermost.com/integrate/reference/server/server-reference/) - HIGH confidence
- [Mattermost Plugin Best Practices](https://developers.mattermost.com/integrate/plugins/best-practices/) - HIGH confidence
- [Mattermost Plugin API (pluginapi) - Go Packages](https://pkg.go.dev/github.com/mattermost/mattermost-plugin-api) - HIGH confidence
- [Mattermost Plugin Starter Template](https://github.com/mattermost/mattermost-plugin-starter-template) - HIGH confidence
- [Mattermost Interactive Messages](https://developers.mattermost.com/integrate/plugins/interactive-messages/) - HIGH confidence
- [Mattermost plugin.Hooks Interface](https://pkg.go.dev/github.com/mattermost/mattermost/server/public/plugin) - HIGH confidence
- [Mattermost model types](https://pkg.go.dev/github.com/mattermost/mattermost/server/public/model) - HIGH confidence
- [Mattermost Developer Setup](https://developers.mattermost.com/contribute/developer-setup/) - HIGH confidence
- [Plane API Documentation](https://developers.plane.so/api-reference/introduction) - HIGH confidence
- [Plane Create Work Item](https://developers.plane.so/api-reference/issue/add-issue) - HIGH confidence
- [Plane List Work Items](https://developers.plane.so/api-reference/issue/list-issues) - HIGH confidence
- [Plane List Projects](https://developers.plane.so/api-reference/project/list-projects) - HIGH confidence
- [Plane Webhooks](https://developers.plane.so/dev-tools/intro-webhooks) - HIGH confidence
- [Obsidian Local REST API - GitHub](https://github.com/coddingtonbear/obsidian-local-rest-api) - HIGH confidence
- [Obsidian Local REST API - Interactive Docs](https://coddingtonbear.github.io/obsidian-local-rest-api/) - HIGH confidence
- [Obsidian Local REST API - DeepWiki](https://deepwiki.com/coddingtonbear/obsidian-local-rest-api) - MEDIUM confidence
- [Mattermost Releases](https://endoflife.date/mattermost) - HIGH confidence
