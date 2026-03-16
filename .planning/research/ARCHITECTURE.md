# Architecture Patterns

**Domain:** Mattermost Plugin -- Chat-to-Task Bridge (Plane + Obsidian)
**Researched:** 2026-03-16

## Recommended Architecture

### High-Level: Monolithic Plugin with Internal Service Layer

The entire system lives as a **single Mattermost plugin** (Go binary). No separate bridge service. The Mattermost plugin SDK provides: custom HTTP endpoints via `ServeHTTP`, outbound HTTP calls, KV store for persistence, bot accounts for posting, slash commands, interactive dialogs, and webapp components for post action menus. Everything needed is built in.

**Architecture decision: Plugin-only, no external bridge service.**

```
+----------------------------------------------------------+
|                    Mattermost Server                      |
|                                                          |
|  +----------------------------------------------------+  |
|  |          Command Center Plugin (Go binary)          |  |
|  |                                                     |  |
|  |  +-----------+  +-----------+  +----------------+  |  |
|  |  | Command   |  | Action    |  | Webhook        |  |  |
|  |  | Router    |  | Handler   |  | Receiver       |  |  |
|  |  +-----+-----+  +-----+-----+  +-------+--------+  |  |
|  |        |               |                |            |  |
|  |  +-----v---------------v----------------v--------+  |  |
|  |  |              Core Orchestrator                 |  |  |
|  |  |  (command dispatch, context resolution,        |  |  |
|  |  |   channel-project mapping, user preferences)   |  |  |
|  |  +-----+------------------+----------------------+  |  |
|  |        |                  |                         |  |
|  |  +-----v------+    +-----v--------+                |  |
|  |  | Plane      |    | Obsidian     |                |  |
|  |  | Client     |    | Client       |                |  |
|  |  +-----+------+    +------+-------+                |  |
|  |        |                  |                         |  |
|  |  +-----v------+    +-----v--------+                |  |
|  |  | KV Store   |    | Bot Account  |                |  |
|  |  | (mappings, |    | (posts,      |                |  |
|  |  |  tokens,   |    |  notifs,     |                |  |
|  |  |  cache)    |    |  errors)     |                |  |
|  |  +------------+    +--------------+                |  |
|  +----------------------------------------------------+  |
+----------------------------------------------------------+
         |                              |
         v                              v
  +--------------+            +-------------------+
  | Plane API    |            | Obsidian Local    |
  | (self-hosted)|            | REST API (per     |
  | Port varies  |            | user, port 27124) |
  +--------------+            +-------------------+
```

### Component Boundaries

| Component | Responsibility | Communicates With | Go File(s) |
|-----------|---------------|-------------------|-------------|
| **Plugin Core** | Lifecycle (OnActivate/OnDeactivate), configuration management, bot account setup | Mattermost Server via RPC | `plugin.go`, `configuration.go` |
| **Command Router** | Registers `/task` slash command, dispatches subcommands (plane/obsidian/help) | Core Orchestrator | `command.go`, `command_plane.go`, `command_obsidian.go` |
| **Action Handler** | Processes interactive message button clicks and dialog form submissions | Core Orchestrator | `action_handlers.go`, `dialog_handlers.go` |
| **Webhook Receiver** | Accepts inbound Plane webhooks via ServeHTTP, verifies HMAC signatures | Core Orchestrator, Bot Account | `api.go`, `webhook_plane.go` |
| **Core Orchestrator** | Business logic: resolves channel context, dispatches to correct API client, formats responses | All components | Embedded in handler functions |
| **Plane Client** | HTTP client wrapper for Plane REST API v1 (work items, projects) | Plane API server | `plane/client.go`, `plane/types.go`, `plane/work_items.go`, `plane/projects.go` |
| **Obsidian Client** | HTTP client wrapper for Obsidian Local REST API (per-user routing) | Obsidian instances (per user) | `obsidian/client.go`, `obsidian/types.go`, `obsidian/router.go` |
| **KV Store Layer** | Abstraction over Mattermost KV store: channel-project mappings, user configs, cache with TTL | Mattermost KV API | `store/store.go`, `store/channel_mapping.go`, `store/user_config.go`, `store/keys.go` |
| **Bot Account** | Posts confirmations, notifications, ephemeral responses, error messages | Mattermost Post API | `bot.go` |
| **Config/Settings** | Admin console settings struct, validation, thread-safe access | Mattermost Config API | `configuration.go` |

---

## Data Flow

### Flow 1: Slash Command Creates Task in Plane

```
User types: /task plane create "Fix login bug" --priority high

1. Mattermost Server --> ExecuteCommand hook --> Command Router
2. Command Router parses: ["plane", "create", "Fix login bug", "--priority", "high"]
3. If channel has project binding:
   a. Read KV: "cp:{channel_id}" -> {workspace, project_id, project_name}
   b. Open interactive dialog pre-filled with project + title + priority
4. If no binding:
   a. Open dialog with project dropdown (fetched from Plane API, cached)
5. User reviews and submits dialog
6. Dialog submission POST to /plugins/{plugin_id}/dialogs/submit-task
7. Core Orchestrator --> Plane Client.CreateWorkItem(workspace, project, {
     name: "Fix login bug",
     priority: "high",
     description_html: "<p>Created from Mattermost by @user in #channel</p><p><a href='permalink'>Original message</a></p>"
   })
8. Plane API returns work item with ID
9. Bot posts ephemeral to user: "Created PROJ-42: Fix login bug [Open in Plane]"
```

### Flow 2: Message Context Menu Creates Task

```
User clicks "..." on message --> "Create Task in Plane"

1. Webapp calls plugin action URL via POST
2. Plugin receives PostActionIntegrationRequest with: user_id, post_id, channel_id
3. Plugin fetches original post content via API
4. Opens interactive dialog with:
   - Title: first line of message text (truncated to 100 chars)
   - Description: full message text + "\n\n---\n[View in Mattermost](permalink)"
   - Project: from channel binding, or dropdown
   - Priority: default "none"
   - Assignee: default to message author
5. User edits and submits
6. (Same as Flow 1, steps 6-9)
7. Additionally: Bot replies to original message thread with
   "Task created: PROJ-42 [Open in Plane]"
```

### Flow 3: Plane Webhook Notifies Channel

```
Work item status changes in Plane (e.g., "In Progress" -> "Done")

1. Plane sends POST to https://<mattermost>/plugins/{plugin_id}/webhook/plane
   Headers: X-Plane-Event: "issue", X-Plane-Signature: hmac, X-Plane-Delivery: uuid
   Body: {event: "issue", action: "update", data: {work_item_object}}
2. Webhook Receiver verifies HMAC-SHA256 signature against stored secret
3. Checks dedup: KV get "wd:{delivery_uuid}" -- if exists, return 200 (already processed)
4. Stores dedup key: KV set "wd:{delivery_uuid}" with 1-hour expiry
5. Parses event: extract project_id from data
6. Queries KV reverse index: "pc:{project_id}" -> [channel_id_1, channel_id_2]
7. For each linked channel, Bot posts:
   "[PROJ-42] Fix login bug: In Progress -> Done (by @user) [Open in Plane]"
8. Returns HTTP 200 immediately to avoid Plane retry
```

### Flow 4: Obsidian Note Creation (Per-User Routing)

```
User clicks "..." on message --> "Save to Obsidian"

1. Plugin receives action with user_id, post_id
2. Queries KV: "uo:{user_id}" -> {endpoint, api_key, vault_path}
   - If not configured: ephemeral error "Run /task obsidian setup first"
3. Fetches original post content
4. Constructs markdown note:
   ---
   source: mattermost
   channel: #general
   author: @username
   date: 2026-03-16
   mattermost_link: https://mm.example.com/team/pl/post_id
   ---

   # {message first line}

   {full message text}

   ---
   *Saved from Mattermost on 2026-03-16*
5. Obsidian Client: PUT /vault/{vault_path}/{sanitized_title}.md
   Headers: Authorization: Bearer {user_api_key}, Content-Type: text/markdown
   TLS: InsecureSkipVerify (self-signed cert)
6. On success: ephemeral "Note saved to Obsidian: Tasks/{title}.md"
7. On connection error: ephemeral "Cannot reach your Obsidian. Is it running with Local REST API enabled?"
```

### Flow 5: View My Tasks

```
User types: /task plane mine

1. Command Router --> handlePlaneMyTasks
2. Resolve user's Plane identity (stored in KV or mapped from Mattermost email)
3. Plane Client: GET /workspaces/{slug}/projects/{project_id}/work-items/
     ?assignee={plane_user_id}&expand=state,labels&limit=10
   (If in linked channel, scope to that project. If not, query across projects.)
4. Format as ephemeral message with table:
   | # | Title | Status | Priority |
   |---|-------|--------|----------|
   | PROJ-42 | Fix login bug | Done | High |
   | PROJ-43 | Add search | In Progress | Medium |
5. Bot sends ephemeral post to user
```

---

## Patterns to Follow

### Pattern 1: Plugin Struct with Service Dependencies

**What:** Central Plugin struct holding all service references, initialized in OnActivate.

**When:** Always. Standard Mattermost plugin pattern.

```go
// server/plugin.go
type Plugin struct {
    plugin.MattermostPlugin

    client      *pluginapi.Client   // Higher-level API wrapper
    botUserID   string              // Bot account for posting
    router      *mux.Router         // HTTP router for ServeHTTP
    planeClient *plane.Client       // Plane API client
    store       *store.Store        // KV store abstraction

    // Thread-safe configuration access
    configurationLock sync.RWMutex
    configuration     *configuration
}

func (p *Plugin) OnActivate() error {
    p.client = pluginapi.NewClient(p.API, p.Driver)

    botUserID, err := p.client.Bot.EnsureBot(&model.Bot{
        Username:    "taskbot",
        DisplayName: "Command Center",
        Description: "Creates and tracks tasks in Plane and Obsidian.",
    })
    if err != nil {
        return fmt.Errorf("failed to ensure bot: %w", err)
    }
    p.botUserID = botUserID

    p.store = store.New(p.client)

    if err := p.registerCommands(); err != nil {
        return fmt.Errorf("failed to register commands: %w", err)
    }

    p.router = p.initAPI()
    p.planeClient = plane.NewClient(p.getConfiguration())

    return nil
}
```

### Pattern 2: Command Dispatch with Subcommands

**What:** Single root command (`/task`) with clean subcommand routing.

**When:** Multiple related actions under one namespace.

```go
// server/command.go
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
    split := strings.Fields(args.Command)
    if len(split) < 2 {
        return p.respondEphemeral(args, p.helpText()), nil
    }

    switch split[1] {
    case "plane":
        return p.handlePlaneCommand(args, split[2:])
    case "obsidian":
        return p.handleObsidianCommand(args, split[2:])
    case "help":
        return p.respondEphemeral(args, p.helpText()), nil
    default:
        return p.respondEphemeral(args, "Unknown subcommand. Try `/task help`."), nil
    }
}

func (p *Plugin) registerCommands() error {
    return p.client.SlashCommand.Register(&model.Command{
        Trigger:          "task",
        AutoComplete:     true,
        AutoCompleteDesc: "Manage tasks in Plane and Obsidian",
        AutoCompleteHint: "[plane|obsidian|help]",
    })
}
```

### Pattern 3: HTTP Router for Webhooks and Actions

**What:** Use `gorilla/mux` for clean routing of inbound HTTP requests.

**When:** Plugin needs to receive external HTTP calls or handle interactive message callbacks.

```go
// server/api.go
func (p *Plugin) initAPI() *mux.Router {
    r := mux.NewRouter()

    // Plane webhook receiver (external -> plugin)
    r.HandleFunc("/webhook/plane", p.handlePlaneWebhook).Methods("POST")

    // Interactive message action handlers (Mattermost -> plugin)
    r.HandleFunc("/actions/create-plane-task", p.handleCreatePlaneTaskAction).Methods("POST")
    r.HandleFunc("/actions/save-to-obsidian", p.handleSaveToObsidianAction).Methods("POST")

    // Dialog submission handlers
    r.HandleFunc("/dialogs/submit-plane-task", p.handlePlaneTaskDialogSubmit).Methods("POST")
    r.HandleFunc("/dialogs/submit-plane-link", p.handlePlaneLinkDialogSubmit).Methods("POST")
    r.HandleFunc("/dialogs/submit-obsidian-setup", p.handleObsidianSetupDialogSubmit).Methods("POST")

    return r
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
    p.router.ServeHTTP(w, r)
}
```

### Pattern 4: KV Store with Typed Keys, Prefixes, and Reverse Indexes

**What:** Structured key patterns for the KV store with forward and reverse lookups.

**When:** Storing any relational data (channel-project mappings, user configurations).

```go
// server/store/keys.go
const (
    // Forward: channel -> project
    // Key: "cp:{channel_id}" | Value: ProjectMapping JSON
    prefixChannelProject = "cp:"

    // Reverse: project -> channels (for webhook routing)
    // Key: "pc:{project_id}" | Value: []string of channel_ids JSON
    prefixProjectChannels = "pc:"

    // User -> Obsidian config
    // Key: "uo:{user_id}" | Value: ObsidianConfig JSON
    prefixUserObsidian = "uo:"

    // User -> Plane identity
    // Key: "up:{mm_user_id}" | Value: PlaneUserMapping JSON
    prefixUserPlane = "up:"

    // Webhook dedup
    // Key: "wd:{delivery_uuid}" | Value: "1" with TTL
    prefixWebhookDedup = "wd:"

    // Cache: project list
    // Key: "cache:projects" | Value: []Project JSON with TTL
    prefixCache = "cache:"
)
```

### Pattern 5: External API Client with Timeouts and Error Wrapping

**What:** Dedicated HTTP client per external service with proper timeouts and Go error wrapping.

```go
// server/plane/client.go
type Client struct {
    baseURL    string
    apiKey     string
    workspace  string
    httpClient *http.Client
}

func NewClient(cfg *Config) *Client {
    return &Client{
        baseURL:   strings.TrimRight(cfg.PlaneURL, "/"),
        apiKey:    cfg.PlaneAPIKey,
        workspace: cfg.PlaneWorkspace,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
    url := fmt.Sprintf("%s/api/v1/workspaces/%s%s", c.baseURL, c.workspace, path)

    var bodyReader io.Reader
    if body != nil {
        b, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("marshal request body: %w", err)
        }
        bodyReader = bytes.NewBuffer(b)
    }

    req, err := http.NewRequest(method, url, bodyReader)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("X-API-Key", c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("plane API request %s %s: %w", method, path, err)
    }
    return resp, nil
}
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Separate Bridge Service

**What:** Running a separate microservice outside Mattermost to handle API bridging.
**Why bad:** Adds deployment complexity, separate process management, network latency, additional failure points. The Mattermost plugin SDK already provides HTTP endpoints, outbound HTTP, KV persistence, and bot accounts.
**Instead:** Keep all logic inside the plugin. The plugin IS the bridge.

### Anti-Pattern 2: In-Memory State for Persistent Data

**What:** Storing channel-project mappings, user preferences, or cached responses in Go variables (maps, structs).
**Why bad:** Lost on plugin restart. Inconsistent across nodes in HA deployments. Works in dev, breaks in production.
**Instead:** Use KV store (`KVSet`/`KVGet`) for ALL persistent state from day one. Use `KVCompareAndSet` for concurrent modifications.

### Anti-Pattern 3: Flat KV Keys Without Prefixes

**What:** Keys like "config", "mapping1", "user_data".
**Why bad:** No way to list by type, collision risk, impossible to debug. KV store has 50-char key limit and no native prefix scanning.
**Instead:** Short structured prefixes (`cp:`, `uo:`, `up:`) and maintain index keys for listing.

### Anti-Pattern 4: Storing Secrets Unencrypted in KV Store

**What:** Plain-text Obsidian API keys or user tokens in KV store.
**Why bad:** KV values are readable by server admins and potentially in database backups.
**Instead:** Use `settings_schema` with `secret: true` for admin-level secrets. For per-user secrets, implement plugin-level AES encryption before KVSet.

### Anti-Pattern 5: One Giant plugin.go File

**What:** All command handling, API clients, webhook processing in one file.
**Why bad:** Unmaintainable beyond ~500 lines. Impossible to test individual components.
**Instead:** Split by responsibility as shown in directory structure below.

### Anti-Pattern 6: Direct Plane API Calls Without Caching

**What:** Fetching project list, states, labels, members on every command execution.
**Why bad:** 60 req/min rate limit means a team of 10 making queries will exhaust the budget quickly.
**Instead:** Cache project metadata in KV store with 5-10 minute TTL. States and labels change rarely.

---

## Recommended Directory Structure

```
mattermost-plugin-command-center/
|-- plugin.json                    # Plugin manifest (ID, name, settings_schema)
|-- server/
|   |-- main.go                    # Entry point: plugin.ClientMain(&Plugin{})
|   |-- plugin.go                  # Plugin struct, OnActivate, OnDeactivate
|   |-- configuration.go           # Settings struct, OnConfigurationChange, validation
|   |-- command.go                 # ExecuteCommand hook, subcommand dispatch
|   |-- command_plane.go           # /task plane create|mine|search|link|status
|   |-- command_obsidian.go        # /task obsidian create|setup
|   |-- api.go                     # ServeHTTP, mux router init
|   |-- action_handlers.go         # Post action button callbacks
|   |-- dialog_handlers.go         # Interactive dialog submissions
|   |-- webhook_plane.go           # Inbound Plane webhook handler + HMAC verify
|   |-- bot.go                     # Bot posting helpers, message formatting
|   |-- plane/
|   |   |-- client.go              # Plane API HTTP client (base methods)
|   |   |-- types.go               # Plane API request/response Go structs
|   |   |-- work_items.go          # CreateWorkItem, ListWorkItems, GetWorkItem
|   |   |-- projects.go            # ListProjects, GetProject
|   |-- obsidian/
|   |   |-- client.go              # Obsidian Local REST API HTTP client
|   |   |-- types.go               # Obsidian API types
|   |   |-- router.go              # Per-user endpoint routing (read config from KV)
|   |-- store/
|   |   |-- store.go               # KV store abstraction layer
|   |   |-- channel_mapping.go     # Channel-project CRUD + reverse index
|   |   |-- user_config.go         # Per-user Obsidian/Plane config + encryption
|   |   |-- cache.go               # TTL-based caching for project metadata
|   |   |-- keys.go                # Key prefix constants
|-- webapp/
|   |-- src/
|   |   |-- index.tsx              # Plugin registration
|   |   |-- plugin.ts              # Register post menu actions
|-- assets/
|   |-- icon.svg                   # Plugin icon for marketplace
|-- build/                         # Build scripts (from starter template)
|-- Makefile                       # Build: make, make dist, make deploy, make test
|-- go.mod
|-- go.sum
```

---

## Scalability Considerations

| Concern | At 2-5 users | At 10 users | At 50+ users |
|---------|--------------|-------------|--------------|
| Plane API rate limits | 60 req/min shared key is plenty | Still fine with caching | Per-user API keys or request queue needed |
| Obsidian connectivity | Users on same LAN, direct access | Works with proper network config | Architecture breaks down -- VPN/tunnel per user |
| KV store size | Trivial (<100 keys) | Trivial (<500 keys) | Cleanup strategy for stale entries |
| Webhook volume | Low | Low-moderate | Debounce notifications to avoid channel spam |
| Plugin memory | Minimal (Go subprocess) | Minimal | Fine -- Go plugins are lightweight |
| Concurrent commands | No contention | Minimal contention | Use KVCompareAndSet for all shared state |

**Explicit scope:** Designed for 2-10 users. The Obsidian integration inherently limits to users whose machines are network-accessible from Mattermost. This is a LAN/VPN constraint, not a software constraint.

---

## Sources

- [Mattermost Plugin API and Hooks (DeepWiki)](https://deepwiki.com/mattermost/mattermost/4.2-plugin-api-and-hooks) - HIGH confidence
- [Mattermost Server Plugin SDK Reference](https://developers.mattermost.com/integrate/reference/server/server-reference/) - HIGH confidence
- [Mattermost Interactive Messages](https://developers.mattermost.com/integrate/plugins/interactive-messages/) - HIGH confidence
- [Mattermost Interactive Dialogs](https://developers.mattermost.com/integrate/plugins/interactive-dialogs/) - HIGH confidence
- [Mattermost Server Plugins Guide](https://developers.mattermost.com/integrate/plugins/components/server/) - HIGH confidence
- [Mattermost Plugin Starter Template](https://github.com/mattermost/mattermost-plugin-starter-template) - HIGH confidence
- [mattermost-plugin-api Go Package](https://pkg.go.dev/github.com/mattermost/mattermost-plugin-api) - HIGH confidence
- [Mattermost plugin.Hooks Interface](https://pkg.go.dev/github.com/mattermost/mattermost/server/public/plugin) - HIGH confidence
- [Mattermost Plugin Demo (command hooks)](https://github.com/mattermost/mattermost-plugin-demo/blob/master/server/command_hooks.go) - HIGH confidence
- [Mattermost Plugin Demo (HTTP hooks)](https://github.com/mattermost/mattermost-plugin-demo/blob/master/server/http_hooks.go) - HIGH confidence
- [Plane API Reference](https://developers.plane.so/api-reference/introduction) - HIGH confidence
- [Plane Webhooks](https://developers.plane.so/dev-tools/intro-webhooks) - HIGH confidence
- [Obsidian Local REST API (GitHub)](https://github.com/coddingtonbear/obsidian-local-rest-api) - HIGH confidence
- [Obsidian Local REST API (DeepWiki)](https://deepwiki.com/coddingtonbear/obsidian-local-rest-api) - MEDIUM confidence
