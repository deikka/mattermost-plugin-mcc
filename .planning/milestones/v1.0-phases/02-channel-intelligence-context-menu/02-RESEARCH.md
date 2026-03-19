# Phase 2: Channel Intelligence + Context Menu - Research

**Researched:** 2026-03-17
**Domain:** Mattermost Plugin Hooks (MessageHasBeenPosted), Webapp Plugin (Post Dropdown Menu), KV Store bindings, Plane API (GetWorkItem)
**Confidence:** HIGH

## Summary

This phase transforms Mattermost channels into project-aware workspaces by implementing four capabilities: (1) channel-project binding via `/task plane link`/`unlink` stored in KV, (2) a "Create Task" context menu item in the post dropdown that pre-populates a creation dialog with message text, (3) auto-selection of the bound project in all commands within linked channels, and (4) link unfurling that renders Plane work item previews when URLs are pasted in chat.

The most significant architectural finding is that the context menu ("Create Task from Message") requires a **webapp plugin component**. The `registerPostDropdownMenuAction` API is a webapp-only feature -- there is no way to add items to the post "..." dropdown menu from server-side code alone. The current plugin is server-only, so Phase 2 must add a minimal webapp bundle (one JS file, ~50 lines) that registers the menu action and makes an HTTP POST to the server to open a dialog. This is a small but structurally important change. For link unfurling, the server-side `MessageHasBeenPosted` hook intercepts messages, detects Plane URLs via regex, fetches work item details, and creates a reply post with a `SlackAttachment` card.

**Primary recommendation:** Add a minimal webapp component for the context menu, implement `MessageHasBeenPosted` for link unfurling, store channel-project bindings in KV with a `channel_project_` prefix, and modify all existing command handlers to check for bindings when resolving project context.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Relacion 1:1: un canal solo puede estar vinculado a un proyecto de Plane
- `/task plane link` crea/reemplaza el binding; `/task plane unlink` lo elimina
- Cualquier usuario conectado (que haya hecho `/task connect`) puede vincular/desvincular
- Al vincular, el bot publica un mensaje visible para todo el canal (no efimero): "Canal vinculado al proyecto X"
- Al desvincular, mismo patron: post visible al canal
- Menu contextual se abre el dialog de creacion pre-poblado (no creacion directa)
- Titulo pre-poblado con las primeras ~80 caracteres del mensaje
- Descripcion pre-poblada con el texto completo del mensaje + permalink al mensaje original
- Tras crear la tarea, el bot anade emoji reaction :memo: al mensaje original como indicador visual
- Confirmacion efimera al usuario con link a la tarea creada (patron existente de formatTaskCreatedMessage)
- Link unfurling: al pegar URL de tarea de Plane en chat, se muestra attachment debajo del mensaje
- Info mostrada: titulo, estado (con emoji), asignado, prioridad, nombre del proyecto
- Usa la API key global del admin (no requiere que el usuario tenga /task connect)
- Funciona en cualquier canal, no solo en canales vinculados
- Patron de URL a detectar: URLs que matcheen el PlaneURL configurado + path de work item
- En canal vinculado, el dialog de creacion pre-selecciona el proyecto vinculado pero es editable
- En inline create (`/task plane create titulo`), usa el proyecto vinculado automaticamente
- `/task plane mine` en canal vinculado filtra solo tareas del proyecto vinculado
- `/task plane status` en canal vinculado muestra el estado del proyecto vinculado sin pedir nombre
- Las respuestas efimeras incluyen "(Proyecto: X)" cuando se usa auto-seleccion
- En canal sin binding, comportamiento actual sin cambios

### Claude's Discretion
- Longitud exacta del texto del mensaje usado como descripcion (truncamiento si necesario por limites de API)
- Formato exacto del attachment de link unfurling (colores, layout del card)
- Manejo de URLs de Plane que no correspondan a work items validos o accesibles
- Pattern matching para detectar URLs de Plane (regex vs string match)

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| CREA-02 | Create task from context menu "..." of any message, with text pre-populated as description and permalink to original | Webapp `registerPostDropdownMenuAction` + server HTTP endpoint that opens dialog via `OpenInteractiveDialog` with pre-populated fields |
| CREA-03 | When creating task in bound channel, project pre-selected automatically | KV store lookup of `channel_project_` binding + `Default` field in dialog project select element |
| BIND-01 | User can bind channel to Plane project via `/task plane link` | New command handlers `plane/link` and `plane/unlink` + KV store CRUD with `channel_project_` prefix |
| BIND-02 | Commands in bound channel auto-use associated project | Modify `handlePlaneCreate`, `handlePlaneMine`, `handlePlaneStatus` to check binding before defaulting |
| NOTF-03 | Pasting Plane task URL shows inline preview with title, state, assignee | `MessageHasBeenPosted` hook + regex URL detection + `GetWorkItem` API call + bot reply with `SlackAttachment` |
</phase_requirements>

## Standard Stack

### Core (existing -- no new Go dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/mattermost/mattermost/server/public` | v0.1.21 | Plugin hooks (MessageHasBeenPosted), model types (Post, Reaction, SlackAttachment) | Already in go.mod |
| `github.com/gorilla/mux` | v1.8.1 | HTTP routing for new endpoints | Already in go.mod |
| `regexp` | stdlib | URL pattern matching for link unfurling | No new dependency needed |
| `strings` | stdlib | URL parsing, text truncation | Already used |

### New (webapp component)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| webpack | ^5.x | Bundle webapp JS into main.js | Build-time only |
| @babel/core + presets | ^7.x | Transpile JSX/ES6 | Build-time only |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Webapp component for context menu | Server-only interactive buttons on every post | Impractical -- would require modifying every post or creating a wrapper; registerPostDropdownMenuAction is the standard Mattermost pattern |
| Bot reply for link unfurling | UpdatePost to add attachment to original post | Reply is less intrusive, avoids permission issues with modifying other users' posts |
| Regex for URL matching | strings.Contains | Regex is more precise, handles edge cases (URLs in markdown, multiple URLs in one message) |

**No new Go dependencies required.** The webapp component needs npm dev dependencies for building but these don't ship in the plugin bundle.

## Architecture Patterns

### New File Structure
```
webapp/                          # NEW: Minimal webapp component
  src/
    index.js                     # Plugin entry point (~50 lines)
  package.json                   # Dev dependencies only
  webpack.config.js              # Standard webpack config
server/
  plugin.go                      # ADD: MessageHasBeenPosted hook
  command_router.go              # ADD: plane/link, plane/unlink handlers
  command_handlers.go            # MODIFY: Add binding-aware logic to existing handlers
  command_handlers_binding.go    # NEW: handlePlaneLink, handlePlaneUnlink
  command_handlers_context.go    # NEW: handleContextMenuAction (HTTP handler)
  dialog.go                      # MODIFY: Accept pre-populated title/description/project
  api.go                         # ADD: /api/v1/action/create-task-from-message endpoint
  link_unfurl.go                 # NEW: MessageHasBeenPosted + URL parsing + attachment building
  store/
    store.go                     # ADD: ChannelProjectBinding type + CRUD methods
plugin.json                      # ADD: webapp.bundle_path
Makefile                         # MODIFY: Add webapp build step
```

### Pattern 1: Channel-Project Binding (KV Store)
**What:** Store 1:1 mapping between Mattermost channel IDs and Plane project IDs
**When to use:** Every command handler that needs project context
**Key format:** `channel_project_{channelID}` -> JSON `ChannelProjectBinding`

```go
// Source: Existing store pattern in store/store.go
const prefixChannelProject = "channel_project_"

type ChannelProjectBinding struct {
    ProjectID   string `json:"project_id"`
    ProjectName string `json:"project_name"`
    BoundBy     string `json:"bound_by"`      // Mattermost user ID who created binding
    BoundAt     int64  `json:"bound_at"`       // Unix timestamp
}

func (s *Store) GetChannelBinding(channelID string) (*ChannelProjectBinding, error) {
    data, appErr := s.api.KVGet(prefixChannelProject + channelID)
    if appErr != nil {
        return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
    }
    if data == nil {
        return nil, nil
    }
    var binding ChannelProjectBinding
    if err := json.Unmarshal(data, &binding); err != nil {
        return nil, fmt.Errorf("unmarshal ChannelProjectBinding: %w", err)
    }
    return &binding, nil
}

func (s *Store) SaveChannelBinding(channelID string, binding *ChannelProjectBinding) error {
    data, err := json.Marshal(binding)
    if err != nil {
        return fmt.Errorf("marshal ChannelProjectBinding: %w", err)
    }
    if appErr := s.api.KVSet(prefixChannelProject+channelID, data); appErr != nil {
        return fmt.Errorf("KVSet failed: %s", appErr.Error())
    }
    return nil
}

func (s *Store) DeleteChannelBinding(channelID string) error {
    if appErr := s.api.KVDelete(prefixChannelProject + channelID); appErr != nil {
        return fmt.Errorf("KVDelete failed: %s", appErr.Error())
    }
    return nil
}
```

### Pattern 2: Webapp Post Dropdown Menu Action
**What:** Register a "Create Task in Plane" item in the post "..." dropdown menu
**When to use:** Allows creating tasks from any message with one click
**Architecture:** Webapp JS registers menu action -> makes HTTP POST to server -> server opens dialog

```javascript
// Source: Mattermost webapp plugin pattern (registerPostDropdownMenuAction)
// webapp/src/index.js

class MccPlugin {
    initialize(registry, store) {
        registry.registerPostDropdownMenuAction(
            'Create Task in Plane',
            (postId) => {
                // POST to our server endpoint which opens the dialog
                fetch(`/plugins/com.klab.mattermost-command-center/api/v1/action/create-task-from-message`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ post_id: postId }),
                });
            },
        );
    }
}

window.registerPlugin('com.klab.mattermost-command-center', new MccPlugin());
```

**Server-side handler** receives the POST, fetches the post content, builds permalink, and opens dialog:

```go
// Source: Mattermost plugin API pattern
func (p *Plugin) handleCreateTaskFromMessage(w http.ResponseWriter, r *http.Request) {
    userID := r.Header.Get("Mattermost-User-Id")

    var req struct {
        PostID string `json:"post_id"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Fetch the original post
    post, appErr := p.API.GetPost(req.PostID)
    // Get channel for team info (needed for permalink)
    channel, _ := p.API.GetChannel(post.ChannelId)
    team, _ := p.API.GetTeam(channel.TeamId)

    // Build permalink: {siteURL}/{teamName}/pl/{postID}
    siteURL := *p.API.GetConfig().ServiceSettings.SiteURL
    permalink := fmt.Sprintf("%s/%s/pl/%s", siteURL, team.Name, post.Id)

    // Truncate message for title (~80 chars)
    title := truncateTitle(post.Message, 80)

    // Full message + permalink for description
    description := post.Message + "\n\n---\n[Original message](" + permalink + ")"

    // Check channel binding for project pre-selection
    binding, _ := p.store.GetChannelBinding(post.ChannelId)

    // Open dialog with pre-populated fields
    openCreateTaskDialogWithContext(p, triggerID, post.ChannelId, userID, title, description, binding, post.Id)
}
```

### Pattern 3: Link Unfurling via MessageHasBeenPosted
**What:** Detect Plane work item URLs in messages and add an attachment preview
**When to use:** Automatically when any user posts a message containing a Plane URL
**Implementation:** Hook -> regex extract -> Plane API call -> bot creates reply with SlackAttachment

```go
// Source: Mattermost Plugin Hook pattern
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
    // Skip bot posts to avoid loops
    if post.UserId == p.botUserID {
        return
    }

    cfg := p.getConfiguration()
    if cfg.PlaneURL == "" {
        return
    }

    // Extract Plane work item URLs from message
    urls := extractPlaneWorkItemURLs(post.Message, cfg.PlaneURL, cfg.PlaneWorkspace)
    if len(urls) == 0 {
        return
    }

    // Process first URL only (avoid spam)
    projectID, workItemID := urls[0].ProjectID, urls[0].WorkItemID

    // Fetch work item details (uses global API key)
    workItem, err := p.planeClient.GetWorkItem(projectID, workItemID)
    if err != nil {
        p.API.LogWarn("Failed to fetch work item for unfurl", "error", err.Error())
        return
    }

    // Build attachment
    attachment := buildWorkItemAttachment(workItem, cfg.PlaneURL, cfg.PlaneWorkspace, projectID)

    // Create bot reply with attachment
    replyPost := &model.Post{
        UserId:    p.botUserID,
        ChannelId: post.ChannelId,
        RootId:    post.Id,  // Reply to original
        Message:   "",
    }
    model.ParseSlackAttachment(replyPost, []*model.SlackAttachment{attachment})

    p.API.CreatePost(replyPost)
}
```

### Pattern 4: Binding-Aware Command Handlers
**What:** Modify existing handlers to check channel binding before asking for project
**When to use:** Every handler that works with project context
**Example: handlePlaneStatus modification**

```go
// In handlePlaneStatus, before the "which project?" prompt:
binding, _ := p.store.GetChannelBinding(args.ChannelId)
if binding != nil && len(subArgs) == 0 {
    // Use bound project automatically
    project = findProjectByNameOrID(projects, binding.ProjectName)
    // Append indicator to response
    suffix = fmt.Sprintf(" (Proyecto: %s)", binding.ProjectName)
}
```

### Anti-Patterns to Avoid
- **Modifying other users' posts for unfurling:** Use bot reply instead of `UpdatePost` on the original message. Modifying posts changes their edit history and may cause permission issues.
- **Opening dialogs without trigger_id:** The `OpenInteractiveDialog` API requires a valid trigger_id. For the context menu, the webapp must pass one via an HTTP POST to the server, not try to open the dialog directly from server-side hooks.
- **Processing bot messages in MessageHasBeenPosted:** Always check `post.UserId == p.botUserID` to avoid infinite loops where the bot processes its own unfurl replies.
- **Heavy API calls in MessageHasBeenPosted:** This hook fires for EVERY message. Keep it fast: regex check first, then API call only if URL found. Consider short-lived cache for repeated URLs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Post dropdown menu item | Server-side post interception | `registerPostDropdownMenuAction` (webapp) | Only supported way to add items to the "..." menu |
| Message attachment cards | Custom message formatting | `model.SlackAttachment` + `model.ParseSlackAttachment` | Mattermost renders these as rich cards natively |
| Post permalink URLs | Manual URL construction | `{siteURL}/{teamName}/pl/{postID}` pattern | Standard Mattermost permalink format |
| Emoji reactions | Custom post props | `p.API.AddReaction(&model.Reaction{...})` | Native API method since v5.3 |
| Trigger ID for dialogs | Server-side generation | Let webapp pass it via fetch to server endpoint | Trigger IDs are browser-scoped and cannot be generated server-side |

**Key insight:** The context menu (post dropdown) is exclusively a webapp feature. There is no server-side API to register menu items in the post "..." dropdown. This is by design -- the webapp owns the UI, the server owns the data.

## Common Pitfalls

### Pitfall 1: Infinite Loop in MessageHasBeenPosted
**What goes wrong:** Bot posts unfurl reply -> hook fires again for bot's post -> bot tries to unfurl its own attachment text
**Why it happens:** MessageHasBeenPosted fires for ALL posts including bot posts
**How to avoid:** First line of the hook must be `if post.UserId == p.botUserID { return }`
**Warning signs:** Plugin crashes with stack overflow or high CPU usage

### Pitfall 2: Missing Trigger ID for Context Menu Dialog
**What goes wrong:** Server tries to open dialog but has no valid trigger_id
**Why it happens:** Trigger IDs are generated client-side when user interacts with UI. Server-side hooks (MessageHasBeenPosted) don't have trigger_ids. Interactive message button trigger_ids historically had issues with dialog opening.
**How to avoid:** The webapp `fetch()` call to the server endpoint inherently carries the user's session. The server endpoint should use `p.API.OpenInteractiveDialog()` with the proper trigger_id. The webapp must include credentials in the fetch call.
**Warning signs:** Dialog doesn't open, 400 errors from dialog/open endpoint

### Pitfall 3: Webapp Bundle Not Found After Deploy
**What goes wrong:** Plugin activates but context menu item doesn't appear
**Why it happens:** plugin.json has `webapp.bundle_path` but the built main.js is not in the correct location in the tarball
**How to avoid:** Ensure Makefile bundles `webapp/dist/main.js` and plugin.json `webapp.bundle_path` points to `webapp/dist/main.js`
**Warning signs:** Check System Console > Plugins, webapp portion shows as not loaded

### Pitfall 4: Race Condition in Link Unfurling
**What goes wrong:** Multiple Plane API calls for the same URL when a message is edited
**Why it happens:** MessageHasBeenPosted may fire on edits or retries
**How to avoid:** Only process URLs in the original post content, check if bot already replied to this post before creating a new reply
**Warning signs:** Duplicate unfurl cards under the same message

### Pitfall 5: Permalink Construction for DMs/Group Messages
**What goes wrong:** DMs and group messages don't belong to a team, so `channel.TeamId` is empty
**Why it happens:** TeamId is only set for public/private channels, not DMs
**How to avoid:** For DMs, use the special `_redirect` URL format or skip permalink for now. Context menu creation from DMs is an edge case -- the primary use case is channels.
**Warning signs:** Permalink URL has empty team name like `https://mm.example.com//pl/abc123`

### Pitfall 6: Plane URL Matching False Positives
**What goes wrong:** Regex matches partial URLs or similar-looking text
**Why it happens:** URL regex is too greedy or doesn't account for surrounding context
**How to avoid:** Match the full path structure: `{PlaneURL}/{workspace}/projects/{uuid}/work-items/{uuid}`. UUID format is predictable (hyphenated hex). Anchor the regex with word boundaries.
**Warning signs:** Bot tries to unfurl non-existent work items

## Code Examples

### Adding a Reaction to a Post
```go
// Source: Mattermost Plugin API (AddReaction, min server v5.3)
reaction := &model.Reaction{
    UserId:    p.botUserID,
    PostId:    postID,
    EmojiName: "memo",  // Maps to the memo emoji
}
if _, appErr := p.API.AddReaction(reaction); appErr != nil {
    p.API.LogWarn("Failed to add reaction", "error", appErr.Error())
}
```

### Building a SlackAttachment for Link Unfurling
```go
// Source: Mattermost model.SlackAttachment fields
func buildWorkItemAttachment(item *WorkItem, planeURL, workspace, projectID string) *model.SlackAttachment {
    stateEmoji := stateGroupEmoji(item.StateGroup)
    pLabel := priorityLabel(item.Priority)

    workItemURL := fmt.Sprintf("%s/%s/projects/%s/work-items/%s",
        strings.TrimRight(planeURL, "/"), workspace, projectID, item.ID)

    fields := []*model.SlackAttachmentField{
        {Title: "Status", Value: stateEmoji + " " + item.StateName, Short: true},
        {Title: "Priority", Value: pLabel, Short: true},
    }

    // Add assignee field if available
    if item.AssigneeName != "" {
        fields = append(fields, &model.SlackAttachmentField{
            Title: "Assigned", Value: item.AssigneeName, Short: true,
        })
    }

    return &model.SlackAttachment{
        Color:      "#3f76ff",  // Plane brand blue
        Title:      item.Name,
        TitleLink:  workItemURL,
        Text:       item.ProjectName,
        Fields:     fields,
        Footer:     "Plane",
        FooterIcon: "", // Optional: Plane icon URL
    }
}
```

### Constructing a Permalink
```go
// Source: Mattermost permalink format: {siteURL}/{teamName}/pl/{postID}
func (p *Plugin) buildPermalink(postID, channelID string) string {
    siteURL := ""
    if cfg := p.API.GetConfig(); cfg != nil && cfg.ServiceSettings.SiteURL != nil {
        siteURL = strings.TrimRight(*cfg.ServiceSettings.SiteURL, "/")
    }
    if siteURL == "" {
        return "" // Cannot construct permalink without SiteURL
    }

    channel, appErr := p.API.GetChannel(channelID)
    if appErr != nil || channel.TeamId == "" {
        return "" // DMs/group messages don't have a team
    }

    team, appErr := p.API.GetTeam(channel.TeamId)
    if appErr != nil {
        return ""
    }

    return fmt.Sprintf("%s/%s/pl/%s", siteURL, team.Name, postID)
}
```

### Extracting Plane URLs from Message Text
```go
// Source: Custom regex based on Plane URL format from GetWorkItemURL
// URL format: {PlaneURL}/{workspace}/projects/{projectID}/work-items/{workItemID}
type planeURLMatch struct {
    ProjectID  string
    WorkItemID string
}

func extractPlaneWorkItemURLs(message, planeURL, workspace string) []planeURLMatch {
    escapedBase := regexp.QuoteMeta(strings.TrimRight(planeURL, "/"))
    escapedWS := regexp.QuoteMeta(workspace)

    // UUID pattern: 8-4-4-4-12 hex chars
    uuidPattern := `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

    pattern := fmt.Sprintf(`%s/%s/projects/(%s)/work-items/(%s)`,
        escapedBase, escapedWS, uuidPattern, uuidPattern)

    re := regexp.MustCompile(pattern)
    matches := re.FindAllStringSubmatch(message, -1)

    var results []planeURLMatch
    for _, m := range matches {
        if len(m) >= 3 {
            results = append(results, planeURLMatch{
                ProjectID:  m[1],
                WorkItemID: m[2],
            })
        }
    }
    return results
}
```

### Fetching a Single Work Item (New Plane Client Method)
```go
// Source: Plane API docs - GET /api/v1/workspaces/{ws}/projects/{pid}/work-items/{wid}/
func (c *Client) GetWorkItem(projectID, workItemID string) (*WorkItem, error) {
    path := fmt.Sprintf("/projects/%s/work-items/%s/", projectID, workItemID)
    resp, err := c.doRequest("GET", path, nil)
    if err != nil {
        return nil, fmt.Errorf("get work item: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, parseAPIError(resp)
    }

    var workItem WorkItem
    if err := json.NewDecoder(resp.Body).Decode(&workItem); err != nil {
        return nil, fmt.Errorf("decode work item response: %w", err)
    }
    return &workItem, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `registerPostDropdownMenuComponent` | `registerPostDropdownMenuAction` | Mattermost v7.10 (supported), v11 (required) | Use `registerPostDropdownMenuAction` -- the component variant is deprecated |
| `/issues/` endpoints in Plane | `/work-items/` endpoints | March 2026 | Already using work-items in Phase 1 |
| `MessageWillBePosted` for modification | `MessageHasBeenPosted` for observation | Original design | Use HasBeenPosted since we don't need to modify the message, only react to it |

**Deprecated/outdated:**
- `registerPostDropdownMenuComponent`: Removed in Mattermost v11 (Oct 2025). Use `registerPostDropdownMenuAction` instead.
- `MessageWillBePosted` for link unfurling: Don't use this hook for unfurling -- it blocks post creation and cannot create additional posts during execution.

## Open Questions

1. **Webapp trigger_id propagation**
   - What we know: The webapp `fetch()` call to the server endpoint carries user auth via Mattermost-User-Id header. But interactive dialogs need a trigger_id.
   - What's unclear: Whether the webapp can extract and pass a trigger_id to the server. The standard pattern in Jira plugin uses the webapp to directly call the Mattermost REST API `/api/v4/actions/dialogs/open` with the dialog config, rather than having the server call `OpenInteractiveDialog`.
   - Recommendation: Have the webapp make a GET request to the server to fetch pre-populated dialog data (title, description, project), then have the webapp directly call `/api/v4/actions/dialogs/open` client-side with that data and a locally available trigger_id. This avoids the trigger_id propagation problem entirely. **Alternative simpler approach:** The webapp can call `window.openInteractiveDialog()` if available in the Mattermost webapp plugin API, or construct the dialog request client-side.

2. **Unfurl for edited messages**
   - What we know: `MessageHasBeenPosted` fires for new posts. Mattermost also has `MessageHasBeenUpdated` hook.
   - What's unclear: Whether we should also unfurl on edits (user pastes a Plane URL while editing a message).
   - Recommendation: Only unfurl on new posts in v1. Edits are an edge case. If needed later, implement `MessageHasBeenUpdated` with deduplication.

3. **Assignee name resolution for unfurling**
   - What we know: Work item response includes `assignees` as a list of user IDs. We need display names for the unfurl card.
   - What's unclear: Whether the `expand=assignees` parameter returns full user objects inline.
   - Recommendation: Use `expand=assignees,state,project` on the GetWorkItem call. If assignee data is not inline, fall back to the workspace members cache to resolve IDs to names.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 (same as Phase 1) |
| Config file | None -- Go standard `_test.go` convention |
| Quick run command | `go test ./server/... -count=1 -short` |
| Full suite command | `make test` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BIND-01 | `/task plane link` creates binding, `/task plane unlink` removes it | unit | `go test ./server/ -run TestPlaneLink -count=1` | Wave 0 |
| BIND-01 | Store CRUD for ChannelProjectBinding | unit | `go test ./server/store/ -run TestChannelBinding -count=1` | Wave 0 |
| BIND-02 | Commands in bound channel auto-use project | unit | `go test ./server/ -run TestBindingAware -count=1` | Wave 0 |
| CREA-02 | Context menu action opens pre-populated dialog | integration | `go test ./server/ -run TestContextMenuAction -count=1` | Wave 0 |
| CREA-03 | Dialog pre-selects bound project | unit | `go test ./server/ -run TestDialogPreselect -count=1` | Wave 0 |
| NOTF-03 | Plane URL in message triggers unfurl attachment | unit | `go test ./server/ -run TestLinkUnfurl -count=1` | Wave 0 |
| NOTF-03 | URL extraction regex | unit | `go test ./server/ -run TestExtractPlaneURLs -count=1` | Wave 0 |
| NOTF-03 | GetWorkItem Plane API method | unit | `go test ./server/plane/ -run TestGetWorkItem -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./server/... -count=1 -short`
- **Per wave merge:** `make test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `server/store/store_test.go` -- add tests for ChannelProjectBinding CRUD
- [ ] `server/link_unfurl_test.go` -- test URL extraction, attachment building, MessageHasBeenPosted logic
- [ ] `server/command_test.go` -- add tests for plane link/unlink, binding-aware handlers
- [ ] `server/plane/client_test.go` -- add test for GetWorkItem
- [ ] `webapp/` -- no automated testing needed for ~50 lines of JS; manual verification in browser

## Sources

### Primary (HIGH confidence)
- [Mattermost Plugin API Reference](https://developers.mattermost.com/integrate/reference/server/server-reference/) - AddReaction, GetPost, CreatePost, GetChannel, GetTeam, GetConfig, MessageHasBeenPosted, OpenInteractiveDialog
- [Mattermost Webapp Plugin API](https://developers.mattermost.com/integrate/reference/webapp/webapp-reference/) - registerPostDropdownMenuAction signature and usage
- [Mattermost Interactive Messages](https://developers.mattermost.com/integrate/plugins/interactive-messages/) - PostAction payload format, integration URL, response format
- [Mattermost Interactive Dialogs](https://developers.mattermost.com/integrate/plugins/interactive-dialogs/) - Dialog open flow, trigger_id requirements
- [Mattermost Message Attachments](https://developers.mattermost.com/integrate/reference/message-attachments/) - SlackAttachment field definitions (color, title, title_link, text, fields, footer)
- [Mattermost Manifest Reference](https://developers.mattermost.com/integrate/plugins/manifest-reference/) - webapp.bundle_path configuration
- [Mattermost Webapp Quick Start](https://developers.mattermost.com/integrate/plugins/components/webapp/hello-world/) - Minimal webapp setup, webpack config, build process
- [Plane API - Get Work Item](https://developers.plane.so/api-reference/issue/get-issue-detail) - GET /work-items/{id} endpoint, expand parameter, response fields
- Existing codebase analysis: `server/plugin.go`, `server/command_handlers.go`, `server/dialog.go`, `server/api.go`, `server/store/store.go`, `server/plane/client.go`

### Secondary (MEDIUM confidence)
- [Mattermost Forum: Deprecating PostDropdownMenuComponent](https://forum.mattermost.com/t/deprecating-a-post-dropdown-menu-component-plugin-api-v11/25001) - Confirmed registerPostDropdownMenuAction is the current standard
- [Mattermost Permalink Format](https://docs.mattermost.com/end-user-guide/collaborate/share-links.html) - Format: `{siteURL}/{teamName}/pl/{postID}`
- [Mattermost Plugin DeepWiki](https://deepwiki.com/mattermost/mattermost/4.2-plugin-api-and-hooks) - Hook execution order, API method overview

### Tertiary (LOW confidence)
- [Forum: Interactive Dialog from Button](https://forum.mattermost.com/t/interactive-dialog-cannot-call-from-interactive-message-button/11850) - Historical issues with trigger_id from interactive messages (may be resolved in newer versions)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - No new Go dependencies, all APIs verified in official docs
- Architecture (store, unfurling, commands): HIGH - Follows existing patterns from Phase 1, APIs well-documented
- Architecture (webapp context menu): MEDIUM - Pattern is clear from docs and existing plugins (Jira), but trigger_id propagation may need experimentation during implementation
- Pitfalls: HIGH - Based on official docs warnings, existing codebase patterns, and community reports

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (stable APIs, no breaking changes expected)
