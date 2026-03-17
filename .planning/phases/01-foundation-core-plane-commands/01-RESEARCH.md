# Phase 1: Foundation + Core Plane Commands - Research

**Researched:** 2026-03-17
**Domain:** Mattermost Plugin SDK (Go) + Plane API integration
**Confidence:** HIGH

## Summary

This phase builds a Mattermost server plugin in Go from scratch, implementing slash commands that interact with the Plane project management API for task creation and querying. The plugin uses the official Mattermost Plugin Starter Template as its foundation, the `pluginapi` wrapper for cleaner API access, and communicates with Plane via its REST API v1 using the `/work-items/` endpoints (the `/issues/` endpoints are deprecated and will be removed March 31, 2026).

The core technical challenges are: (1) building a well-structured command routing system with nested subcommands and autocomplete, (2) implementing interactive dialogs with dynamic selectors that fetch data from Plane, (3) managing user-to-Plane mapping via KV store, and (4) handling Plane API rate limits (60 req/min) with appropriate caching.

**Primary recommendation:** Clone the mattermost-plugin-starter-template, use `pluginapi.Client` as the API wrapper, build a Plane API client as a separate internal package, and implement command routing with a handler-map pattern inspired by the Jira plugin.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Comando raiz `/task` con subcomandos: `/task plane create`, `/task plane mine`, `/task plane status`, `/task connect`, `/task obsidian setup`, `/task help`
- Alias cortos: `/task p c` = `/task plane create`, `/task p m` = `/task plane mine`
- Autocompletado completo con hints de argumentos
- Si comando incorrecto: sugerir comando correcto ("Did you mean /task plane create?")
- Dialogo de creacion: Titulo (obligatorio), Descripcion (textarea), Proyecto (selector dinamico), Prioridad (selector), Asignado (selector), Labels (multi-select dinamico)
- Smart defaults: Asignado = quien crea la tarea, Prioridad = ninguna/media
- Modo rapido inline: `/task p c "Titulo de la tarea"` crea directamente con smart defaults sin dialogo
- Dialogo completo cuando se usa `/task plane create` sin argumentos
- `/task plane mine`: Lista compacta, 1 linea por tarea, emoji de estado + titulo + proyecto + prioridad + estado. Max 10 tareas. Link "Abrir en Plane"
- `/task plane status`: Contadores por estado (Open/In Progress/Done) + barra de progreso + ultima actividad + link
- Confirmacion de creacion efimera: "Tarea creada: [Titulo] -- [Proyecto] [link]"
- Todas las respuestas de consultas personales son efimeras

### Claude's Discretion
- Mecanismo exacto de `/task connect` (auto email match vs API key personal vs hibrido)
- Formato de input para `/task obsidian setup` (host+port+key vs URL completa+key)
- Health check al activar plugin (validar conexion Plane)
- Esquema de KV store (prefijos de keys, indices)
- Rate limiting y caching strategy para Plane API (60 req/min)
- Manejo de errores de red/API con mensajes accionables

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CONF-01 | Admin configura URL, API key y workspace de Plane desde System Console | settings_schema en plugin.json con campos text + secret |
| CONF-02 | Usuario vincula cuenta Mattermost con Plane via `/task connect` | KV store para mapeo usuario, Plane members API para validacion |
| CONF-03 | Usuario configura endpoint Obsidian REST API via `/task obsidian setup` | KV store per-user, solo almacenamiento (no llamadas a Obsidian en esta fase) |
| CONF-04 | Usuario ve comandos disponibles via `/task help` | Respuesta efimera con lista formateada de comandos |
| CONF-05 | Plugin crea bot account al activarse | `pluginapi.BotService.EnsureBot()` en `OnActivate` |
| CREA-01 | Crear tarea via `/task plane create` con dialogo interactivo | Interactive dialog con dynamic selects + Plane POST /work-items/ |
| CREA-04 | Confirmacion efimera con link a tarea creada | `SendEphemeralPost` con formato definido por usuario |
| QERY-01 | Ver tareas asignadas via `/task plane mine` | Plane GET /work-items/?assignee={id}&expand=state,labels,project |
| QERY-02 | Ver resumen de proyecto via `/task plane status` | Plane GET /work-items/ con agrupacion por estado + states API |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/mattermost/mattermost/server/public` | v0.1.21 | Mattermost plugin interface, model types, hooks | Official SDK - required for plugin development |
| `github.com/mattermost/mattermost-plugin-api` | latest | Streamlined wrapper (`pluginapi.Client`) over plugin API | Eliminates `model.AppError`, organizes by service, recommended by Mattermost |
| `github.com/gorilla/mux` | v1.8.1 | HTTP routing for plugin endpoints (dialogs, lookups) | Included in starter template, standard for Go HTTP |
| `github.com/pkg/errors` | v0.9.1 | Error wrapping with context | Included in starter template |
| `github.com/stretchr/testify` | v1.11.1 | Testing assertions and mocks | Standard Go testing, included in starter template |
| `go.uber.org/mock` | v0.6.0 | Mock generation for interfaces | Included in starter template for unit testing |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | JSON marshal/unmarshal for Plane API | All API communication |
| `net/http` | stdlib | HTTP client for Plane API calls | Plane API client implementation |
| `sync` | stdlib | RWMutex for configuration, cache locking | Thread-safe config access, cache |
| `time` | stdlib | Cache TTL, rate limit tracking | Caching layer, API throttling |
| `fmt`, `strings` | stdlib | Command parsing, message formatting | Command routing, response building |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `pluginapi.Client` | Raw `plugin.API` | pluginapi is cleaner, eliminates AppError, better organized -- use pluginapi |
| `gorilla/mux` | `net/http` ServeMux | mux comes with template, supports path params -- keep mux |
| Custom HTTP client | `go-resty` or similar | stdlib is sufficient for simple REST calls, fewer dependencies -- use stdlib |

**Installation:**
```bash
# Clone starter template
git clone --depth 1 https://github.com/mattermost/mattermost-plugin-starter-template.git mattermost-plugin-mcc
cd mattermost-plugin-mcc
# Remove webapp/ directory (server-only plugin)
rm -rf webapp/
# Update plugin.json, go.mod with project identity
# go mod tidy
```

## Architecture Patterns

### Recommended Project Structure
```
mattermost-plugin-mcc/
├── plugin.json              # Manifest: id, settings_schema, server executables
├── go.mod                   # Go 1.25, mattermost deps
├── go.sum
├── Makefile                 # Build, deploy, release targets
├── assets/
│   └── icon.svg             # Plugin icon for System Console
├── server/
│   ├── plugin.go            # Plugin struct, OnActivate, ServeHTTP
│   ├── configuration.go     # Thread-safe config with RWMutex
│   ├── command.go           # Command registration + AutocompleteData
│   ├── command_router.go    # ExecuteCommand routing + handler map
│   ├── command_handlers.go  # Individual handler functions
│   ├── dialog.go            # Interactive dialog creation + submission
│   ├── api.go               # HTTP routes (gorilla/mux) for dialogs/lookups
│   ├── plane/
│   │   ├── client.go        # Plane API HTTP client with auth
│   │   ├── types.go         # Plane API request/response structs
│   │   ├── work_items.go    # Work item CRUD operations
│   │   ├── projects.go      # Project listing, members, states
│   │   └── cache.go         # In-memory cache with TTL for API data
│   ├── store/
│   │   ├── store.go         # KV store interface + implementation
│   │   ├── user.go          # User mapping (MM <-> Plane)
│   │   └── obsidian.go      # Obsidian config per user
│   └── bot.go               # Bot account management
└── build/                   # Build scripts (from starter template)
```

### Pattern 1: Command Handler Map
**What:** Route slash commands via a map of `subcommand -> handler function` instead of nested switch/case.
**When to use:** Always for plugins with multiple subcommands.
**Example:**
```go
// Source: Adapted from mattermost-plugin-jira command routing pattern
type CommandHandlerFunc func(p *Plugin, c *plugin.Context, header *model.CommandArgs, args []string) *model.CommandResponse

var commandHandlers = map[string]CommandHandlerFunc{
    "plane/create": handlePlaneCreate,
    "plane/mine":   handlePlaneMine,
    "plane/status": handlePlaneStatus,
    "connect":      handleConnect,
    "obsidian/setup": handleObsidianSetup,
    "help":         handleHelp,
}

// Alias map for short forms
var commandAliases = map[string]string{
    "p/c": "plane/create",
    "p/m": "plane/mine",
    "p/s": "plane/status",
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
    split := strings.Fields(args.Command)
    // split[0] = "/task", rest = subcommand path
    command := buildCommandKey(split[1:])

    // Check aliases
    if alias, ok := commandAliases[command]; ok {
        command = alias
    }

    if handler, ok := commandHandlers[command]; ok {
        return handler(p, c, args, split[1:]), nil
    }

    return suggestCommand(command), nil // "Did you mean...?"
}
```

### Pattern 2: Thread-Safe Configuration
**What:** Guard config pointer with RWMutex, clone on change.
**When to use:** Always -- configuration can change at any time via System Console.
**Example:**
```go
// Source: mattermost-plugin-starter-template/server/configuration.go
type configuration struct {
    PlaneURL       string
    PlaneAPIKey    string
    PlaneWorkspace string
}

func (p *Plugin) getConfiguration() *configuration {
    p.configurationLock.RLock()
    defer p.configurationLock.RUnlock()
    if p.configuration == nil {
        return &configuration{}
    }
    return p.configuration
}

func (p *Plugin) OnConfigurationChange() error {
    var cfg configuration
    if err := p.API.LoadPluginConfiguration(&cfg); err != nil {
        return errors.Wrap(err, "failed to load plugin configuration")
    }
    p.setConfiguration(&cfg)
    return nil
}
```

### Pattern 3: Interactive Dialog with Dynamic Selects
**What:** Open dialog from slash command, fetch options from Plane API via plugin HTTP endpoint.
**When to use:** `/task plane create` without arguments.
**Example:**
```go
// Source: Mattermost interactive dialogs docs
func (p *Plugin) openCreateTaskDialog(triggerID, channelID string) error {
    dialog := model.OpenDialogRequest{
        TriggerId: triggerID,
        URL:       fmt.Sprintf("/plugins/%s/api/v1/dialog/create-task", manifest.Id),
        Dialog: model.Dialog{
            CallbackId: "create_task",
            Title:      "Create Task in Plane",
            Elements: []model.DialogElement{
                {
                    DisplayName: "Title",
                    Name:        "title",
                    Type:        "text",
                    Placeholder: "Task title",
                },
                {
                    DisplayName: "Description",
                    Name:        "description",
                    Type:        "textarea",
                    Optional:    true,
                },
                {
                    DisplayName: "Project",
                    Name:        "project",
                    Type:        "select",
                    DataSource:  "dynamic",
                    // Fetches from plugin HTTP endpoint
                },
                {
                    DisplayName: "Priority",
                    Name:        "priority",
                    Type:        "select",
                    Options: []*model.PostActionOptions{
                        {Text: "None", Value: "none"},
                        {Text: "Low", Value: "low"},
                        {Text: "Medium", Value: "medium"},
                        {Text: "High", Value: "high"},
                        {Text: "Urgent", Value: "urgent"},
                    },
                    Default: "none",
                },
                {
                    DisplayName: "Assignee",
                    Name:        "assignee",
                    Type:        "select",
                    DataSource:  "dynamic",
                },
                {
                    DisplayName: "Labels",
                    Name:        "labels",
                    Type:        "select",
                    DataSource:  "dynamic",
                    // Note: multi-select via custom handling
                },
            },
            SubmitLabel:    "Create Task",
            NotifyOnCancel: false,
        },
    }
    return p.API.OpenInteractiveDialog(dialog)
}
```

### Pattern 4: Ephemeral Response Helper
**What:** Utility to send ephemeral posts visible only to the requesting user.
**When to use:** All query responses and confirmations.
**Example:**
```go
// Source: Mattermost plugin API docs
func (p *Plugin) sendEphemeral(userID, channelID, message string) {
    post := &model.Post{
        UserId:    p.botUserID,
        ChannelId: channelID,
        Message:   message,
    }
    p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) respondEphemeral(args *model.CommandArgs, message string) *model.CommandResponse {
    p.sendEphemeral(args.UserId, args.ChannelId, message)
    return &model.CommandResponse{}
}
```

### Anti-Patterns to Avoid
- **Monolithic ExecuteCommand:** Do not put all command logic in a single switch statement. Use handler map + separate functions.
- **Raw `plugin.API` calls:** Always use `pluginapi.Client` wrapper -- it eliminates `model.AppError` boilerplate.
- **Storing secrets in KV store unencrypted:** Plane user tokens should be encrypted. Use `pluginapi.KVService` with appropriate key prefixes.
- **Blocking OnActivate:** Health checks to Plane should be non-blocking or have short timeouts; don't block plugin activation on external service availability.
- **Using `/issues/` endpoints:** Plane deprecated `/issues/` -- ONLY use `/work-items/`. End of support: March 31, 2026.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bot account creation | Manual bot user creation | `pluginapi.BotService.EnsureBot()` | Handles idempotency, profile image, display name |
| Thread-safe config | Custom sync mechanism | Starter template `configuration.go` pattern | Battle-tested, handles OnConfigurationChange hook |
| Command autocomplete | Manual string matching | `model.AutocompleteData` with `AddCommand`, `AddTextArgument` | Native UI integration, subcommand tree, type hints |
| KV store access | Direct `p.API.KVGet/KVSet` | `pluginapi.KVService` wrapper | Better error handling, typed access |
| Plugin manifest | Hand-written JSON | Starter template `plugin.json` | Correct structure, multi-platform executables |
| HTTP routing | Manual path matching in ServeHTTP | `gorilla/mux` router | Path params, method matching, middleware |
| JSON API responses | Manual marshal + write | Helper functions wrapping `json.NewEncoder` | Consistent error format, content-type headers |

**Key insight:** The Mattermost Plugin Starter Template provides production-ready boilerplate for configuration, KV store, commands, and HTTP routing. Start from it, don't recreate these patterns.

## Common Pitfalls

### Pitfall 1: Interactive Dialog Dynamic Select URL
**What goes wrong:** Dynamic select `data_source_url` fails because URL is not within `/plugins/{plugin-id}/` path or uses HTTP instead of HTTPS.
**Why it happens:** Mattermost enforces that dynamic select URLs are plugin-scoped for security.
**How to avoid:** Always use `fmt.Sprintf("/plugins/%s/api/v1/...", manifest.Id)` for dialog data source URLs. The plugin's ServeHTTP handles these requests internally.
**Warning signs:** Dialog opens but selectors show empty or error.

### Pitfall 2: Trigger ID Expiration
**What goes wrong:** Dialog fails to open because trigger ID from slash command has expired.
**Why it happens:** Trigger IDs must be used immediately; they cannot be stored for later use.
**How to avoid:** Open the dialog synchronously within the ExecuteCommand handler, before returning the response.
**Warning signs:** "Invalid or expired trigger ID" error.

### Pitfall 3: Plane API Rate Limiting (60 req/min)
**What goes wrong:** Plugin gets 429 errors when multiple users query simultaneously.
**Why it happens:** Plane API rate limit is 60 requests per minute per API key (the admin API key is shared).
**How to avoid:** Implement in-memory cache with TTL for: projects list (5 min), states list (10 min), labels list (5 min), members list (5 min). Only work-item creation and per-user queries (`/mine`) hit the API directly.
**Warning signs:** `X-RateLimit-Remaining` header approaches 0.

### Pitfall 4: KV Store Key Conflicts
**What goes wrong:** Data corruption when different data types use overlapping key patterns.
**Why it happens:** KV store is a flat namespace -- no built-in structure.
**How to avoid:** Use explicit prefixes: `user_plane_{mmUserID}` for Plane mappings, `user_obsidian_{mmUserID}` for Obsidian config, `cache_projects` for cached data. Never use `mmi_` prefix (reserved by Mattermost internals).
**Warning signs:** Unexpected data when reading from KV store.

### Pitfall 5: Configuration Not Available in OnActivate
**What goes wrong:** Plugin tries to validate Plane connection in OnActivate but configuration hasn't loaded yet.
**Why it happens:** `OnConfigurationChange` may not have been called yet when OnActivate runs.
**How to avoid:** Call `p.OnConfigurationChange()` explicitly at the start of OnActivate, or load config manually via `p.API.LoadPluginConfiguration`.
**Warning signs:** Empty config values during activation.

### Pitfall 6: Plane User ID Resolution
**What goes wrong:** Cannot create work items with correct assignee because Mattermost user ID != Plane user ID.
**Why it happens:** These are separate systems with different user identity spaces.
**How to avoid:** The `/task connect` flow must resolve and store the Plane user UUID. Use the workspace members API to find the matching user, then store the mapping in KV store.
**Warning signs:** Work items created with wrong assignee or "user not found" errors.

## Code Examples

### Plugin Main Entry Point
```go
// Source: mattermost-plugin-starter-template + pluginapi docs
package main

import (
    "sync"
    "github.com/gorilla/mux"
    "github.com/mattermost/mattermost/server/public/plugin"
    pluginapi "github.com/mattermost/mattermost-plugin-api"
)

type Plugin struct {
    plugin.MattermostPlugin
    configurationLock sync.RWMutex
    configuration     *configuration
    client            *pluginapi.Client
    botUserID         string
    router            *mux.Router
    planeClient       *plane.Client // Custom Plane API client
}

func (p *Plugin) OnActivate() error {
    p.client = pluginapi.NewClient(p.API, p.Driver)

    // Load configuration
    if err := p.OnConfigurationChange(); err != nil {
        return err
    }

    // Create bot account
    botID, err := p.client.Bot.EnsureBot(&model.Bot{
        Username:    "task-bot",
        DisplayName: "Task Bot",
        Description: "Mattermost Command Center bot",
    })
    if err != nil {
        return errors.Wrap(err, "failed to ensure bot")
    }
    p.botUserID = botID

    // Register slash command
    if err := p.registerCommands(); err != nil {
        return errors.Wrap(err, "failed to register commands")
    }

    // Initialize Plane client
    cfg := p.getConfiguration()
    p.planeClient = plane.NewClient(cfg.PlaneURL, cfg.PlaneAPIKey, cfg.PlaneWorkspace)

    // Initialize HTTP router
    p.router = mux.NewRouter()
    p.initAPI()

    // Optional: Health check (non-blocking)
    go p.validatePlaneConnection()

    return nil
}

func main() {
    plugin.ClientMain(&Plugin{})
}
```

### Slash Command Registration with Autocomplete
```go
// Source: mattermost model.AutocompleteData docs + todo plugin pattern
func (p *Plugin) registerCommands() error {
    // Root command
    task := model.NewAutocompleteData("task", "[command]", "Task management commands")

    // /task plane subcommands
    plane := model.NewAutocompleteData("plane", "[subcommand]", "Plane task management")
    // Alias: /task p
    planeAlias := model.NewAutocompleteData("p", "[subcommand]", "Plane task management (alias)")

    create := model.NewAutocompleteData("create", "[title]", "Create a new task in Plane")
    create.AddTextArgument("Quick create with title", "[title]", "")
    createAlias := model.NewAutocompleteData("c", "[title]", "Create task (alias)")
    createAlias.AddTextArgument("Quick create with title", "[title]", "")

    mine := model.NewAutocompleteData("mine", "", "Show your assigned tasks")
    mineAlias := model.NewAutocompleteData("m", "", "Your tasks (alias)")

    status := model.NewAutocompleteData("status", "[project]", "Show project status")
    statusAlias := model.NewAutocompleteData("s", "[project]", "Project status (alias)")

    plane.AddCommand(create)
    plane.AddCommand(mine)
    plane.AddCommand(status)
    planeAlias.AddCommand(createAlias)
    planeAlias.AddCommand(mineAlias)
    planeAlias.AddCommand(statusAlias)

    // /task connect, /task help, /task obsidian setup
    connect := model.NewAutocompleteData("connect", "", "Link your Mattermost account with Plane")
    help := model.NewAutocompleteData("help", "", "Show available commands")
    obsidian := model.NewAutocompleteData("obsidian", "[subcommand]", "Obsidian integration")
    obsidianSetup := model.NewAutocompleteData("setup", "", "Configure Obsidian REST API endpoint")
    obsidian.AddCommand(obsidianSetup)

    task.AddCommand(plane)
    task.AddCommand(planeAlias)
    task.AddCommand(connect)
    task.AddCommand(help)
    task.AddCommand(obsidian)

    return p.API.RegisterCommand(&model.Command{
        Trigger:          "task",
        DisplayName:      "Task Management",
        Description:      "Create and manage tasks in Plane and Obsidian",
        AutoComplete:     true,
        AutoCompleteDesc: "Task management commands",
        AutocompleteData: task,
    })
}
```

### Plane API Client
```go
// Source: Plane API docs (developers.plane.so)
package plane

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type Client struct {
    baseURL    string
    apiKey     string
    workspace  string
    httpClient *http.Client
}

func NewClient(baseURL, apiKey, workspace string) *Client {
    return &Client{
        baseURL:   strings.TrimRight(baseURL, "/"),
        apiKey:    apiKey,
        workspace: workspace,
        httpClient: &http.Client{
            Timeout: 10 * time.Second,
        },
    }
}

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
    var reqBody io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("marshal request body: %w", err)
        }
        reqBody = bytes.NewReader(jsonBody)
    }

    url := fmt.Sprintf("%s/api/v1/workspaces/%s%s", c.baseURL, c.workspace, path)
    req, err := http.NewRequest(method, url, reqBody)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("X-API-Key", c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    return c.httpClient.Do(req)
}

// CreateWorkItem creates a new work item in a Plane project
func (c *Client) CreateWorkItem(projectID string, item *CreateWorkItemRequest) (*WorkItem, error) {
    resp, err := c.doRequest("POST", fmt.Sprintf("/projects/%s/work-items/", projectID), item)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        return nil, parseAPIError(resp)
    }

    var workItem WorkItem
    if err := json.NewDecoder(resp.Body).Decode(&workItem); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }
    return &workItem, nil
}

// ListMyWorkItems returns work items assigned to a user across all projects
func (c *Client) ListWorkItems(projectID, assigneeID string, expand []string) ([]WorkItem, error) {
    path := fmt.Sprintf("/projects/%s/work-items/?assignee=%s&expand=%s&per_page=10",
        projectID, assigneeID, strings.Join(expand, ","))
    resp, err := c.doRequest("GET", path, nil)
    // ... decode paginated response
}
```

### KV Store Schema
```go
// Source: Research recommendation (Claude's discretion)
package store

const (
    // User Plane mapping: key = "user_plane_{mmUserID}", value = PlaneUserMapping JSON
    prefixUserPlane = "user_plane_"

    // User Obsidian config: key = "user_obsidian_{mmUserID}", value = ObsidianConfig JSON
    prefixUserObsidian = "user_obsidian_"

    // Cache: key = "cache_{resource}_{id}", value = cached JSON with TTL
    prefixCache = "cache_"
)

type PlaneUserMapping struct {
    PlaneUserID    string `json:"plane_user_id"`
    PlaneEmail     string `json:"plane_email"`
    PlaneDisplayName string `json:"plane_display_name"`
    ConnectedAt    int64  `json:"connected_at"`
}

type ObsidianConfig struct {
    Host   string `json:"host"`
    Port   int    `json:"port"`
    APIKey string `json:"api_key"`
    SetupAt int64 `json:"setup_at"`
}
```

## Claude's Discretion Recommendations

### `/task connect` Mechanism: Hybrid Email Match + Manual Override
**Recommendation:** Try automatic email match first, fall back to interactive dialog.
1. When user runs `/task connect`, plugin gets Mattermost user's email via `p.API.GetUser(userID)`
2. Fetch workspace members from Plane API: `GET /api/v1/workspaces/{slug}/members/`
3. If email matches exactly one Plane member: auto-link and confirm
4. If no match or multiple matches: show interactive dialog asking user to select their Plane account or enter their Plane email
5. Store mapping in KV store: `user_plane_{mmUserID} -> {planeUserID, email, displayName}`

**Rationale:** Most small teams (2-10 people) use the same email across tools. Auto-match is frictionless for the common case while dialog handles edge cases.

### `/task obsidian setup` Format: Host + Port + API Key
**Recommendation:** Use three separate fields: host (default: `127.0.0.1`), port (default: `27124`), API key.
- The Obsidian Local REST API plugin has a well-known default port (27124)
- Separate fields avoid URL parsing issues
- Dialog with defaults pre-filled minimizes user input

### Health Check: Non-blocking with Admin Notification
**Recommendation:** On plugin activation, spawn a goroutine that validates the Plane connection. If it fails, post a DM to the admin via the bot account instead of failing activation.
- Plugin should still activate even if Plane is temporarily unreachable
- Admin gets actionable feedback: "Could not connect to Plane at {url}. Please verify your configuration in System Console."

### KV Store Schema: Prefixed Keys
**Recommendation:** Use the schema defined in Code Examples above.
- `user_plane_{mmUserID}`: Plane user mapping (PlaneUserMapping struct)
- `user_obsidian_{mmUserID}`: Obsidian config (ObsidianConfig struct)
- `cache_projects`: Cached project list with TTL
- `cache_states_{projectID}`: Cached states per project
- `cache_labels_{projectID}`: Cached labels per project
- `cache_members_{projectID}`: Cached members per project

### Caching Strategy: In-Memory with TTL
**Recommendation:** Simple in-memory cache (Go map + sync.RWMutex + TTL), NOT KV store-based cache.
- Projects list: cache 5 minutes
- States per project: cache 10 minutes (rarely change)
- Labels per project: cache 5 minutes
- Members per project: cache 5 minutes
- Work items: NO cache (always fresh for `/mine` and `/status`)

**Rationale:** KV store is persistent across plugin restarts but adds latency. For a team of 2-10, an in-memory map is sufficient. Cache invalidates naturally on restart.

### Error Messages: Actionable with Next Step
**Recommendation:** Every error message includes what went wrong + what to do next.
- Connection error: "Could not reach Plane. Check your network and Plane URL in System Console."
- Auth error: "Plane API key is invalid. Ask your admin to update it in System Console > Plugins > Task Management."
- Not connected: "You haven't linked your Plane account yet. Run `/task connect` to get started."
- Rate limited: "Plane API is busy. Please try again in a few seconds."

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `mattermost-server/v5` or `v6` modules | `mattermost/mattermost/server/public` monorepo | 2023-2024 | Import paths changed, unified versioning |
| `plugin.API` direct calls | `pluginapi.Client` wrapper | Stable since 2022 | Cleaner errors, service-organized API |
| Plane `/issues/` endpoints | Plane `/work-items/` endpoints | Deprecation announced, EOL March 31, 2026 | MUST use `/work-items/` exclusively |
| Go 1.21 for plugins | Go 1.25 | 2025-2026 | Starter template uses Go 1.25 |
| `min_server_version: 6.2.1` | Should target v9.0+ or v10.0+ | 2024-2025 | Newer APIs, better plugin support |

**Deprecated/outdated:**
- `github.com/mattermost/mattermost-server/v5` and `/v6`: Old module paths, do not use
- Plane `/api/v1/.../issues/` endpoints: Being removed March 31, 2026
- `model.AppError` pattern: `pluginapi.Client` returns standard Go errors instead

## Plane API Reference

### Key Endpoints Needed for Phase 1

| Operation | Method | Path | Auth | Notes |
|-----------|--------|------|------|-------|
| List projects | GET | `/api/v1/workspaces/{slug}/projects/` | X-API-Key | Cache 5min |
| List states | GET | `/api/v1/workspaces/{slug}/projects/{pid}/states/` | X-API-Key | Cache 10min, returned in sequence order |
| List labels | GET | `/api/v1/workspaces/{slug}/projects/{pid}/labels/` | X-API-Key | Cache 5min |
| List project members | GET | `/api/v1/workspaces/{slug}/projects/{pid}/members/` | X-API-Key | Cache 5min |
| List workspace members | GET | `/api/v1/workspaces/{slug}/members/` | X-API-Key | For `/task connect` email matching |
| Create work item | POST | `/api/v1/workspaces/{slug}/projects/{pid}/work-items/` | X-API-Key | Required: `name`. Optional: `description_html`, `state`, `assignees[]`, `priority`, `labels[]` |
| List work items | GET | `/api/v1/workspaces/{slug}/projects/{pid}/work-items/` | X-API-Key | Params: `assignee`, `state`, `expand`, `per_page`, `cursor` |

### Plane Priority Values
`none`, `urgent`, `high`, `medium`, `low`

### Plane API Response Headers for Rate Limiting
- `X-RateLimit-Remaining`: Requests left in current window
- `X-RateLimit-Reset`: UTC epoch seconds when limit resets

### Pagination Format
Cursor-based: `?cursor=value:offset:is_prev` where value=page_size, offset=page_number (0-indexed), is_prev=direction.
Default per_page: 100, max: 100.

## Open Questions

1. **Plane workspace members response format**
   - What we know: Endpoint exists at `GET /workspaces/{slug}/members/`, returns member objects with `id` and `created_at`
   - What's unclear: Whether response includes `email` and `display_name` fields (docs are sparse on response schema for members)
   - Recommendation: Test against actual Plane instance early in Phase 1 Wave 0; if email not in response, may need to use a different matching strategy for `/task connect`

2. **Plane self-hosted API version compatibility**
   - What we know: `/work-items/` endpoints are the current standard, `/issues/` deprecated March 31, 2026
   - What's unclear: Which minimum Plane version supports `/work-items/` endpoints on self-hosted instances
   - Recommendation: Add a health check that validates `/work-items/` endpoint availability on activation; if 404, warn admin that their Plane version may be too old

3. **Mattermost dialog multi-select for labels**
   - What we know: Dialogs support `select` elements with `data_source: "dynamic"`, but native `multiselect` support in dialogs is not well-documented
   - What's unclear: Whether `multiselect: true` works on dialog select elements (it works on post action menus)
   - Recommendation: If multi-select not supported in dialogs, use a comma-separated text input for labels or implement a two-step flow. Test early.

4. **`/task plane mine` across multiple projects**
   - What we know: The Plane API requires a `project_id` in the path for listing work items
   - What's unclear: Whether there's a workspace-level endpoint to list all work items assigned to a user across projects
   - Recommendation: Fetch user's projects first, then query work items per project. Cache project list. Limit to 10 results total. If no cross-project endpoint exists, iterate projects (cache results).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 + go.uber.org/mock v0.6.0 |
| Config file | None -- Go uses standard `_test.go` convention |
| Quick run command | `go test ./server/... -run {TestName} -count=1` |
| Full suite command | `make test` (or `go test ./server/... -v -count=1`) |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONF-01 | Admin config loads from plugin.json settings | unit | `go test ./server/ -run TestConfiguration -count=1` | Wave 0 |
| CONF-02 | /task connect maps MM user to Plane user | unit | `go test ./server/ -run TestConnectCommand -count=1` | Wave 0 |
| CONF-03 | /task obsidian setup stores config in KV | unit | `go test ./server/ -run TestObsidianSetup -count=1` | Wave 0 |
| CONF-04 | /task help returns command list | unit | `go test ./server/ -run TestHelpCommand -count=1` | Wave 0 |
| CONF-05 | Bot account created on activate | unit | `go test ./server/ -run TestOnActivate -count=1` | Wave 0 |
| CREA-01 | Dialog opens and submits to Plane API | unit+integration | `go test ./server/ -run TestCreateTask -count=1` | Wave 0 |
| CREA-04 | Ephemeral confirmation after create | unit | `go test ./server/ -run TestCreateTaskConfirmation -count=1` | Wave 0 |
| QERY-01 | /task plane mine returns assigned tasks | unit | `go test ./server/ -run TestPlaneMine -count=1` | Wave 0 |
| QERY-02 | /task plane status returns project summary | unit | `go test ./server/ -run TestPlaneStatus -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./server/... -count=1 -short`
- **Per wave merge:** `make test` (full suite)
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `server/plugin_test.go` -- covers CONF-01, CONF-05 (OnActivate, config loading)
- [ ] `server/command_test.go` -- covers CONF-02, CONF-03, CONF-04, QERY-01, QERY-02 (command routing + handlers)
- [ ] `server/dialog_test.go` -- covers CREA-01, CREA-04 (dialog open + submission)
- [ ] `server/plane/client_test.go` -- covers Plane API client unit tests with HTTP mocks
- [ ] `server/store/store_test.go` -- covers KV store operations
- [ ] `server/testutil/` -- shared test helpers (mock Plane server, mock API)
- [ ] Mattermost `plugintest.API` mock setup in test helpers

## Sources

### Primary (HIGH confidence)
- [Mattermost Plugin Quick Start](https://developers.mattermost.com/integrate/plugins/components/server/hello-world/) - Plugin structure, manifest format, Go code pattern
- [Mattermost Plugin Manifest Reference](https://developers.mattermost.com/integrate/plugins/manifest-reference/) - settings_schema, setting types, all manifest fields
- [Mattermost Interactive Dialogs](https://developers.mattermost.com/integrate/plugins/interactive-dialogs/) - Dialog elements, dynamic selects, submission callback, validation
- [Plane API Introduction](https://developers.plane.so/api-reference/introduction) - Auth, pagination, rate limits, base URL
- [Plane Create Work Item](https://developers.plane.so/api-reference/issue/add-issue) - POST body fields, required fields, response format
- [Plane List Work Items](https://developers.plane.so/api-reference/issue/list-issues) - GET params, expand options, filters
- [Plane List States](https://developers.plane.so/api-reference/state/list-states) - State endpoint, sequence ordering
- [Plane List Labels](https://developers.plane.so/api-reference/label/list-labels) - Labels endpoint
- [Plane Project Members](https://developers.plane.so/api-reference/members/get-project-members) - Members per project
- [Plane Workspace Members](https://developers.plane.so/api-reference/members/get-workspace-members) - For email matching
- [pluginapi Go Package](https://pkg.go.dev/github.com/mattermost/mattermost-plugin-api) - Client wrapper, KV, Bot, Post services
- [Plugin Starter Template](https://github.com/mattermost/mattermost-plugin-starter-template) - Project structure, go.mod, configuration pattern
- [Mattermost Plugin API](https://pkg.go.dev/github.com/mattermost/mattermost/server/public/plugin) - Core plugin interface, hooks

### Secondary (MEDIUM confidence)
- [Mattermost Jira Plugin command.go](https://github.com/mattermost/mattermost-plugin-jira/blob/master/server/command.go) - Command routing pattern with handler map (verified pattern from production plugin)
- [Mattermost Todo Plugin command.go](https://github.com/mattermost/mattermost-plugin-todo/blob/master/server/command.go) - AutocompleteData example with subcommands
- [Mattermost Plugin Best Practices](https://developers.mattermost.com/integrate/plugins/best-practices/) - General guidance

### Tertiary (LOW confidence)
- Plane workspace members response schema: Official docs are sparse on exact fields returned. Test needed against real instance.
- Mattermost dialog `multiselect` support: Not fully documented for interactive dialogs (confirmed for post action menus). Needs testing.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Verified via official starter template go.mod, pkg.go.dev, and Plane official docs
- Architecture: HIGH - Based on production Mattermost plugins (Jira, Todo) and official starter template
- Pitfalls: HIGH - Documented in official Mattermost docs (trigger ID, dialog URLs) and Plane docs (rate limits, deprecation)
- Plane API details: MEDIUM - Endpoints verified but some response schemas (members) are sparse in docs
- Dialog multiselect: LOW - Needs testing against actual Mattermost instance

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (30 days - stable domain, Plane deprecation deadline March 31 is notable)
