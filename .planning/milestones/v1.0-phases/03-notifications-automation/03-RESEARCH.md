# Phase 3: Notifications + Automation - Research

**Researched:** 2026-03-17
**Domain:** Plane webhooks, Mattermost plugin HTTP receivers, scheduled background jobs, KV store patterns
**Confidence:** HIGH

## Summary

Phase 3 closes the feedback loop: changes made in Plane (state transitions, assignee changes, new comments) are automatically published to the linked Mattermost channel, and a periodic digest summarizes project health. This requires three new capabilities: (1) an HTTP webhook receiver endpoint in the plugin, (2) event routing from Plane project to Mattermost channel via the existing channel-project binding, and (3) a cluster-safe scheduled job for periodic digests.

Plane supports webhooks for Issue and Issue Comment events, sending HMAC-signed JSON payloads to an HTTP endpoint. The plugin already has a gorilla/mux router, a bot account, SlackAttachment card builders, and channel-project binding CRUD. The main new work is: webhook handler + HMAC verification, event-to-notification formatter, digest scheduler via `pluginapi/cluster.Schedule()`, two new slash commands (`/task plane notifications on|off` and `/task plane digest daily|weekly|off`), and new KV store types for notification/digest configuration.

**Primary recommendation:** Use Plane's native webhook system (configured via Plane UI by the admin) to push Issue and Issue Comment events to the plugin's `/api/v1/webhook/plane` endpoint. Verify HMAC-SHA256 signatures, deduplicate with `X-Plane-Delivery` UUID, resolve project-to-channel via existing `store.GetChannelBinding()` reverse lookup, and post rich SlackAttachment cards reusing the `buildWorkItemAttachment()` pattern. For digests, use `cluster.Schedule()` with a 1-minute tick that checks per-channel digest configs from KV store.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Three event types: state changes, assignee changes, new comments
- NO task creation notifications (already have ephemeral confirmation)
- Only notify in channels with 1:1 binding from Phase 2
- All changes visible to all -- no user-specific filtering
- Only notify changes made directly in Plane (not changes originated from the plugin)
- Card rica (SlackAttachment) reusing link unfurling pattern from Phase 2
- Title includes action + task name (e.g., "Estado cambiado: Fix login bug")
- State changes show before -> after transition
- Comment notifications include ~200 char truncated text
- Each card includes link to task in Plane
- Digest configurable per channel via `/task plane digest daily|weekly|off`
- Customizable publish hour per channel
- Frequencies: daily, weekly, off
- Dashboard content: counters by state, completed tasks in period, new tasks, state changes, project link
- Digest posts visible to entire channel (not ephemeral)
- No temporal grouping: each change = one notification
- All-or-nothing: all 3 event types when on, no per-type filtering
- Command `/task plane notifications on|off` to pause/resume without unbinding
- Independent posts: no threading per task

### Claude's Discretion
- Webhook reception mechanism (endpoint HTTP, polling, or adapter per Plane capabilities)
- Exact SlackAttachment field layout for notifications
- KV store schema for digest config (prefixes, structure)
- Scheduler implementation for periodic digests (goroutine with ticker, cron plugin, etc.)
- Error handling when Plane sends incomplete webhook data
- Default publish hour if user doesn't specify

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| NOTF-01 | Cambios en tareas de Plane (estado, asignacion, comentarios) se publican automaticamente en el canal vinculado via webhooks | Plane webhook system supports Issue + Issue Comment events with create/update/delete actions; plugin's existing HTTP router + channel binding store enables routing; SlackAttachment pattern proven in Phase 2 |
| NOTF-02 | Bot publica resumen periodico (configurable: diario/semanal) del estado del proyecto en el canal vinculado | `pluginapi/cluster.Schedule()` provides HA-safe background jobs; existing `ListProjectWorkItems()` + state grouping logic from `handlePlaneStatus` is directly reusable; KV store supports per-channel config with TTL |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `mattermost/server/public/pluginapi/cluster` | v0.1.21 (monorepo) | HA-safe scheduled background jobs | Built into plugin SDK; ensures only one instance runs digest job across cluster nodes |
| `crypto/hmac` + `crypto/sha256` | Go stdlib | HMAC-SHA256 webhook signature verification | Plane signs payloads with HMAC-SHA256; Go stdlib is the correct tool |
| `gorilla/mux` | v1.8.1 | HTTP routing for webhook endpoint | Already in use; new route for `/api/v1/webhook/plane` |
| `encoding/json` | Go stdlib | Webhook payload parsing | Already in use throughout the plugin |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `mattermost/server/public/model.PluginKVSetOptions` | v0.1.21 | TTL-based KV storage for dedup keys | Webhook deduplication with `ExpireInSeconds` |
| `mattermost/server/public/model.SlackAttachment` | v0.1.21 | Rich notification cards | Already used in link unfurling; reuse for webhook notifications |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Plane webhooks (push) | Polling Plane API periodically | Polling wastes rate limit budget (60 req/min); push is the correct model |
| `cluster.Schedule()` | Raw goroutine + ticker | Raw goroutine runs on ALL nodes in HA; cluster.Schedule ensures single execution |
| HMAC verification | No verification | Security risk; must verify to prevent spoofed webhooks |

## Architecture Patterns

### Recommended New Files
```
server/
  webhook_plane.go           # Webhook HTTP handler, HMAC verification, event routing
  command_handlers_notify.go  # handlePlaneNotifications, handlePlaneDigest commands
  digest.go                  # Digest scheduler, digest content builder
```

### Pattern 1: Plane Webhook Receiver
**What:** HTTP POST endpoint that receives Plane webhook events, verifies HMAC signature, deduplicates, and routes to the correct channel.
**When to use:** For all inbound Plane events (issue updates, comment creates).

```go
// webhook_plane.go

// handlePlaneWebhook processes incoming Plane webhook events.
// Route: POST /api/v1/webhook/plane
func (p *Plugin) handlePlaneWebhook(w http.ResponseWriter, r *http.Request) {
    // 1. Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        writeError(w, http.StatusBadRequest, "cannot read body")
        return
    }

    // 2. Verify HMAC-SHA256 signature
    signature := r.Header.Get("X-Plane-Signature")
    if !p.verifyWebhookSignature(body, signature) {
        writeError(w, http.StatusForbidden, "invalid signature")
        return
    }

    // 3. Deduplicate via X-Plane-Delivery UUID
    deliveryID := r.Header.Get("X-Plane-Delivery")
    if p.isWebhookDuplicate(deliveryID) {
        writeJSON(w, http.StatusOK, map[string]string{"status": "already processed"})
        return
    }
    p.markWebhookProcessed(deliveryID)

    // 4. Parse event
    var event PlaneWebhookEvent
    if err := json.Unmarshal(body, &event); err != nil {
        writeError(w, http.StatusBadRequest, "invalid JSON")
        return
    }

    // 5. Route based on event type
    switch event.Event {
    case "issue":
        p.handleIssueWebhook(&event)
    case "issue_comment":
        p.handleIssueCommentWebhook(&event)
    }

    // 6. Return 200 immediately (Plane retries on non-200)
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) verifyWebhookSignature(body []byte, signature string) bool {
    cfg := p.getConfiguration()
    if cfg.PlaneWebhookSecret == "" {
        return true // No secret configured = skip verification (log warning)
    }
    mac := hmac.New(sha256.New, []byte(cfg.PlaneWebhookSecret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```

### Pattern 2: Event-to-Channel Routing via Reverse Lookup
**What:** Given a project ID from a webhook event, find all channels bound to that project and post a notification to each.
**When to use:** Every webhook event that should produce a notification.

**Key insight:** The current store has forward lookup only (`channel_project_{channelID}` -> binding). For webhook routing we need reverse lookup: `project_id -> [channelIDs]`. Two approaches:

- **Option A (recommended):** Maintain a reverse index key `project_channels_{projectID}` -> `[]string{channelIDs}`. Update on bind/unbind.
- **Option B:** Use `KVList` to scan all `channel_project_` keys and filter. Works but O(n) on every webhook -- unacceptable at scale.

**Recommendation:** Option A. Add reverse index maintenance to `SaveChannelBinding()` and `DeleteChannelBinding()`.

```go
// store/store.go additions

const prefixProjectChannels = "project_channels_"

// GetProjectChannels returns all channel IDs bound to a project.
func (s *Store) GetProjectChannels(projectID string) ([]string, error) {
    data, appErr := s.api.KVGet(prefixProjectChannels + projectID)
    if appErr != nil {
        return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
    }
    if data == nil {
        return nil, nil
    }
    var channels []string
    if err := json.Unmarshal(data, &channels); err != nil {
        return nil, fmt.Errorf("unmarshal channels: %w", err)
    }
    return channels, nil
}
```

### Pattern 3: Cluster-Safe Digest Scheduler
**What:** Background job that runs every 1 minute, checks all configured digests, and posts when due.
**When to use:** For the periodic project digest feature (NOTF-02).

```go
// digest.go

func (p *Plugin) startDigestScheduler() error {
    job, err := cluster.Schedule(
        p.API,
        "DigestScheduler",
        cluster.MakeWaitForRoundedInterval(1*time.Minute),
        p.runDigestCheck,
    )
    if err != nil {
        return fmt.Errorf("failed to schedule digest job: %w", err)
    }
    p.digestJob = job
    return nil
}

func (p *Plugin) stopDigestScheduler() {
    if p.digestJob != nil {
        _ = p.digestJob.Close()
    }
}

func (p *Plugin) runDigestCheck() {
    // 1. List all digest configs from KV store
    // 2. For each config, check if current time matches scheduled hour
    // 3. If due, build digest content and post to channel
    // 4. Update last-posted timestamp to prevent re-posting
}
```

### Pattern 4: Notification Card Builder (Reusing Link Unfurl Pattern)
**What:** Build SlackAttachment cards for webhook events, reusing `stateGroupEmoji()`, `priorityLabel()`, and the visual style from `buildWorkItemAttachment()`.

```go
// Example state change notification card
func buildStateChangeAttachment(taskName, oldState, newState, actorName, taskURL string) *model.SlackAttachment {
    emoji := stateGroupEmoji(newStateGroup)
    return &model.SlackAttachment{
        Color:  "#3f76ff",
        Title:  fmt.Sprintf("%s Estado cambiado: %s", emoji, taskName),
        TitleLink: taskURL,
        Fields: []*model.SlackAttachmentField{
            {Title: "Cambio", Value: fmt.Sprintf("%s -> %s", oldState, newState), Short: true},
            {Title: "Por", Value: actorName, Short: true},
        },
        Footer: "Plane",
    }
}
```

### Anti-Patterns to Avoid
- **Don't poll Plane API for changes:** Wastes the 60 req/min rate limit. Use webhooks (push model).
- **Don't store digest state in memory:** Use KV store. In-memory state is lost on restart and diverges across HA nodes.
- **Don't process webhooks synchronously with long operations:** Return HTTP 200 immediately, then process async if needed. Plane retries on timeout.
- **Don't skip HMAC verification:** Even in dev. The webhook secret should be in plugin configuration (settings_schema).
- **Don't use raw goroutines for periodic work:** Use `cluster.Schedule()` to prevent duplicate execution across HA nodes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| HA-safe periodic execution | Custom goroutine + ticker + KV locking | `pluginapi/cluster.Schedule()` | Handles mutex, metadata, jitter, single-execution guarantee across nodes |
| HMAC-SHA256 verification | Custom byte comparison | `crypto/hmac.Equal()` | Constant-time comparison prevents timing attacks |
| Webhook dedup | In-memory map of processed IDs | `KVSetWithOptions` with `ExpireInSeconds` | Survives restarts, works across HA nodes |
| Rich message formatting | Custom markdown strings | `model.SlackAttachment` with `ParseSlackAttachment` | Standard Mattermost pattern, consistent rendering, proven in Phase 2 |
| Reverse index maintenance | Manual on every bind/unbind | Centralized in `SaveChannelBinding`/`DeleteChannelBinding` | Single point of truth, prevents index drift |

**Key insight:** The cluster.Schedule package handles the hardest part (ensuring only one node executes the digest job) with KV-based mutex and metadata. Don't attempt to build this manually.

## Common Pitfalls

### Pitfall 1: Plane Webhook Payload Has No "Before" State
**What goes wrong:** The webhook payload contains the CURRENT state of the issue, not a diff. For state change notifications showing "Open -> In Progress", you don't get the old state from Plane.
**Why it happens:** Plane sends the complete entity on update, not a change delta.
**How to avoid:** Two strategies:
  1. **Accept "current state only"** -- show "Estado: In Progress" instead of "Open -> In Progress". Simpler but less informative.
  2. **Cache previous state** -- store `last_state_{workItemID}` in KV with short TTL. On webhook, compare current vs cached. Risk: cache miss on first event or after restart.
**Recommendation:** Strategy 2 for state changes (cache the state group per work item). Fall back to "Estado: {current}" when no previous state is cached. The webhook `data.state.group` field provides the current state group.
**Warning signs:** Notifications always showing current state without transition context.

### Pitfall 2: Webhook Duplicate Events
**What goes wrong:** Plane may send multiple webhook deliveries for a single change (reported as known issue in Plane GitHub).
**Why it happens:** Bug in Plane's webhook dispatch or retry on timeout.
**How to avoid:** Deduplicate using the `X-Plane-Delivery` header UUID. Store `webhook_dedup_{deliveryID}` in KV with 1-hour TTL via `KVSetWithOptions` + `ExpireInSeconds: 3600`. Check before processing.
**Warning signs:** Duplicate notification posts in channel.

### Pitfall 3: Plugin-Originated Changes Trigger Self-Notifications
**What goes wrong:** User creates a task via `/task plane create` in Mattermost. Plane fires a webhook for the new issue. Plugin receives it and posts a notification -- duplicating the already-shown ephemeral confirmation.
**Why it happens:** Plane webhooks fire for ALL changes, including those made via API.
**How to avoid:** The CONTEXT.md decision says "Solo notificar cambios hechos directamente en Plane". Implementation: track recent plugin-originated work item IDs in KV with short TTL (e.g., `plugin_action_{workItemID}` for 5 minutes). On webhook receipt, check if the work item ID is in this "recently created by plugin" set. If yes, skip notification.
**Warning signs:** Duplicate notifications when creating tasks from Mattermost.

### Pitfall 4: Digest Job Fires on All Nodes in HA
**What goes wrong:** Without cluster coordination, each Mattermost node runs its own digest goroutine, posting duplicate daily summaries.
**Why it happens:** Plugins run as separate processes per node.
**How to avoid:** Use `cluster.Schedule()` which acquires a KV-based mutex before executing. Only one node wins the lock and executes the callback. Already addressed in architecture above.
**Warning signs:** Multiple identical digest posts appearing.

### Pitfall 5: Rate Limit Exhaustion from Digest Jobs
**What goes wrong:** Digest job calls `ListProjectWorkItems()` for every channel with a digest configured. If 10 channels are bound, that's 10 API calls in one execution cycle, on top of regular user commands.
**Why it happens:** 60 req/min rate limit shared across all plugin operations.
**How to avoid:** Digest queries use the existing cached `ListProjectWorkItems()`. The cache TTL is already 5 minutes for projects. For digest-specific data, consider a longer cache window since digest content doesn't need to be real-time. Stagger digest execution if multiple channels fire at the same minute.
**Warning signs:** 429 errors in logs during digest execution windows.

### Pitfall 6: Network Reachability for Webhooks
**What goes wrong:** Plane cannot reach the Mattermost plugin's webhook endpoint. Self-hosted Plane and Mattermost may be on different networks.
**Why it happens:** Docker networking, firewalls, reverse proxy configuration.
**How to avoid:** Document the webhook URL format clearly: `{mattermost_site_url}/plugins/com.klab.mattermost-command-center/api/v1/webhook/plane`. Admin must configure this URL in Plane's webhook settings manually. Add a test endpoint or log entry to confirm webhook receipt works.
**Warning signs:** No webhook events arriving; Plane admin panel shows failed deliveries.

## Code Examples

### Plane Webhook Payload Structure (Issue Update)
```json
{
  "event": "issue",
  "action": "updated",
  "webhook_id": "4af07fdc-12b2-4861-9c1b-0e585780045f",
  "workspace_id": "e18e76fd-8ebb-43f8-ba15-d54bb788f9ef",
  "data": {
    "id": "96f534cc-1f97-4ee8-8d42-2c5b3e619d1f",
    "name": "Fix login bug",
    "labels": [
      {"id": "...", "name": "bug", "color": "#ff0000"}
    ],
    "assignees": [
      {
        "id": "cc054819-...",
        "first_name": "Alice",
        "last_name": "Smith",
        "email": "alice@example.com",
        "display_name": "Alice"
      }
    ],
    "state": {
      "id": "4cf15b53-...",
      "name": "In Progress",
      "color": "#f59e0b",
      "group": "started"
    },
    "sequence_id": 42,
    "project": "proj-uuid-001",
    "created_at": "2024-12-19T12:54:39.857651Z",
    "updated_at": "2024-12-19T15:31:32.602996Z"
  }
}
```
Source: [Plane GitHub Issue #6235](https://github.com/makeplane/plane/issues/6235) - verified payload from real webhook delivery.

**Important:** Webhook `data` fields use NESTED OBJECTS for `state` and `assignees` (not flat `state__name` fields like the API expand query). The webhook types need DIFFERENT Go structs from the existing `plane.WorkItem`.

### Plane Webhook Headers
```
Content-Type: application/json
User-Agent: Autopilot
X-Plane-Delivery: <UUID>
X-Plane-Event: issue | issue_comment
X-Plane-Signature: <HMAC-SHA256 hex digest>
```
Source: [Plane Webhooks Documentation](https://developers.plane.so/dev-tools/intro-webhooks)

### HMAC Verification in Go
```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
)

func verifyHMAC(body []byte, signature, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(expected), []byte(signature))
}
```
Source: Plane docs Python example translated to Go; Go stdlib `crypto/hmac` docs.

### cluster.Schedule Usage
```go
import "github.com/mattermost/mattermost/server/public/pluginapi/cluster"

// In Plugin struct:
type Plugin struct {
    // ... existing fields ...
    digestJob *cluster.Job
}

// In OnActivate:
job, err := cluster.Schedule(
    p.API,
    "DigestScheduler",
    cluster.MakeWaitForRoundedInterval(1*time.Minute),
    p.runDigestCheck,
)
p.digestJob = job

// In OnDeactivate:
func (p *Plugin) OnDeactivate() error {
    if p.digestJob != nil {
        return p.digestJob.Close()
    }
    return nil
}
```
Source: `pluginapi/cluster/job.go` from `mattermost/server/public` v0.1.21 (verified in module cache).

### KV Set with TTL (for webhook dedup)
```go
// Store delivery ID with 1-hour expiration
_, appErr := p.API.KVSetWithOptions(
    "webhook_dedup_"+deliveryID,
    []byte("1"),
    model.PluginKVSetOptions{
        ExpireInSeconds: 3600,
    },
)
```
Source: `model.PluginKVSetOptions` from `mattermost/server/public` v0.1.21.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Plane `/issues/` endpoints | `/work-items/` endpoints | March 2026 deprecation | Webhook events still use `"event": "issue"` but payload uses work-item fields |
| Separate bridge service for webhooks | Plugin's ServeHTTP handles webhooks directly | Mattermost plugin SDK v5+ | No external service needed |
| Raw goroutine for background jobs | `cluster.Schedule()` | pluginapi v0.1.x | HA-safe, single-execution, KV-based mutex |

**Deprecated/outdated:**
- Plane webhook event names use "issue" and "issue_comment" (not "work_item"). This is a naming artifact; the payload data uses work-item structure.
- The `mattermost-plugin-api` separate repo is NOT used by this project; use `mattermost/server/public/pluginapi` monorepo path instead.

## Open Questions

1. **Plane webhook payload for Issue Comment events**
   - What we know: Plane supports `issue_comment` as a webhook event type. Payload includes `event: "issue_comment"` and `action: "created"/"updated"/"deleted"`.
   - What's unclear: Exact field names for comment text content (`comment_html`? `body`? `body_html`?). Also unclear if the comment payload includes the parent issue details (project ID, issue name).
   - Recommendation: Build the handler with defensive parsing. Log the full payload on first receipt for field discovery. Use `json.RawMessage` for the data field initially and refine types after observing real payloads.

2. **Plane webhook "activity" field with actor info**
   - What we know: Some webhook payloads include an `activity` field with `actor` containing `id` and `display_name`.
   - What's unclear: Whether this is present on ALL events or only some. Whether it identifies who made the change.
   - Recommendation: Parse `activity.actor` if present, fall back to examining the `assignees` field or omitting actor info.

3. **No "previous state" in Plane webhook payload**
   - What we know: Plane sends the complete current entity, not a diff.
   - What's unclear: Whether there's an `updated_fields` or `changes` key for PATCH events.
   - Recommendation: Implement state caching (store last known state group per work item) to detect transitions. Accept graceful degradation to "current state only" display when cache misses.

4. **Webhook creation is manual (UI-only)**
   - What we know: Plane webhooks are configured through the Plane workspace settings UI. No documented API endpoint for programmatic webhook CRUD.
   - What's unclear: Whether an undocumented API exists for creating webhooks.
   - Recommendation: Document the setup steps clearly for the admin. The webhook URL is: `{mattermost_site_url}/plugins/com.klab.mattermost-command-center/api/v1/webhook/plane`. Admin must create the webhook in Plane and paste the secret into the Mattermost plugin's System Console settings.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + testify v1.11.1 |
| Config file | None (Go convention, `go test ./...`) |
| Quick run command | `cd server && go test ./... -count=1 -short` |
| Full suite command | `cd server && go test ./... -count=1 -v` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| NOTF-01a | Webhook endpoint accepts POST, verifies HMAC, returns 200 | unit | `cd server && go test -run TestHandlePlaneWebhook -count=1` | Wave 0 |
| NOTF-01b | Issue update webhook posts state change card to bound channel | unit | `cd server && go test -run TestWebhookIssueStateChange -count=1` | Wave 0 |
| NOTF-01c | Issue update webhook posts assignee change card to bound channel | unit | `cd server && go test -run TestWebhookAssigneeChange -count=1` | Wave 0 |
| NOTF-01d | Issue comment webhook posts truncated comment card to bound channel | unit | `cd server && go test -run TestWebhookIssueComment -count=1` | Wave 0 |
| NOTF-01e | Webhook deduplication skips already-processed delivery IDs | unit | `cd server && go test -run TestWebhookDedup -count=1` | Wave 0 |
| NOTF-01f | Notifications command toggles on/off in KV store | unit | `cd server && go test -run TestHandlePlaneNotifications -count=1` | Wave 0 |
| NOTF-02a | Digest command saves config to KV store | unit | `cd server && go test -run TestHandlePlaneDigest -count=1` | Wave 0 |
| NOTF-02b | Digest scheduler posts summary when due | unit | `cd server && go test -run TestDigestExecution -count=1` | Wave 0 |
| NOTF-02c | Reverse index updated on bind/unbind | unit | `cd server && go test -run TestReverseIndex -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `cd server && go test ./... -count=1 -short`
- **Per wave merge:** `cd server && go test ./... -count=1 -v`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `server/webhook_plane_test.go` -- covers NOTF-01a through NOTF-01e
- [ ] `server/command_handlers_notify_test.go` -- covers NOTF-01f, NOTF-02a
- [ ] `server/digest_test.go` -- covers NOTF-02b
- [ ] `server/store/store_test.go` (additions) -- covers NOTF-02c reverse index tests

## KV Store Schema (Claude's Discretion Decision)

### New Prefixes
| Prefix | Key Pattern | Value | Purpose |
|--------|------------|-------|---------|
| `notify_config_` | `notify_config_{channelID}` | `NotificationConfig` JSON | Per-channel notification on/off |
| `digest_config_` | `digest_config_{channelID}` | `DigestConfig` JSON | Per-channel digest frequency + hour |
| `project_channels_` | `project_channels_{projectID}` | `[]string` (channelIDs) JSON | Reverse index for webhook routing |
| `webhook_dedup_` | `webhook_dedup_{deliveryID}` | `"1"` with TTL 3600s | Webhook delivery deduplication |
| `work_item_state_` | `work_item_state_{workItemID}` | `WorkItemStateCache` JSON with TTL | Previous state cache for transition detection |
| `plugin_action_` | `plugin_action_{workItemID}` | `"1"` with TTL 300s | Suppress self-notifications for plugin-originated changes |
| `digest_last_` | `digest_last_{channelID}` | Unix timestamp string | Last digest execution time |

### New Types
```go
type NotificationConfig struct {
    Enabled   bool  `json:"enabled"`
    UpdatedBy string `json:"updated_by"`
    UpdatedAt int64  `json:"updated_at"`
}

type DigestConfig struct {
    Frequency string `json:"frequency"` // "daily", "weekly", "off"
    Hour      int    `json:"hour"`      // 0-23, default 9
    Weekday   int    `json:"weekday"`   // 0=Sunday..6=Saturday (only for weekly)
    UpdatedBy string `json:"updated_by"`
    UpdatedAt int64  `json:"updated_at"`
}

type WorkItemStateCache struct {
    StateGroup string `json:"state_group"`
    StateName  string `json:"state_name"`
    CachedAt   int64  `json:"cached_at"`
}
```

## Scheduler Decision (Claude's Discretion)

**Recommendation:** `cluster.Schedule()` with 1-minute tick interval.

**Rationale:**
- `cluster.Schedule(p.API, "DigestScheduler", cluster.MakeWaitForRoundedInterval(1*time.Minute), p.runDigestCheck)` runs every minute, checks KV store for channels with digest configs, evaluates if any are due based on configured hour, and posts if needed.
- 1-minute resolution is sufficient -- digests post at a configured hour, not to the second.
- `MakeWaitForRoundedInterval` aligns to clock boundaries (e.g., 09:00, 09:01) which is desirable for "daily at 9 AM" semantics.
- The callback reads all `digest_config_` entries, compares current hour to configured hour, and checks `digest_last_{channelID}` to prevent re-posting within the same period.

**Default hour:** 9 (09:00 local server time) -- reasonable standup time.

## Plugin Configuration Addition

The `PlaneWebhookSecret` field must be added to the plugin's settings_schema in `plugin.json` and to the `configuration` struct:

```go
type configuration struct {
    PlaneURL           string `json:"PlaneURL"`
    PlaneAPIKey        string `json:"PlaneAPIKey"`
    PlaneWorkspace     string `json:"PlaneWorkspace"`
    PlaneWebhookSecret string `json:"PlaneWebhookSecret"` // NEW: HMAC secret from Plane webhook setup
}
```

## Webhook Types (Distinct from API Types)

Plane webhook payloads use NESTED objects (not the `__` flat notation from the query API). New types are needed:

```go
// PlaneWebhookEvent is the top-level webhook payload.
type PlaneWebhookEvent struct {
    Event       string          `json:"event"`       // "issue", "issue_comment"
    Action      string          `json:"action"`      // "created", "updated", "deleted"
    WebhookID   string          `json:"webhook_id"`
    WorkspaceID string          `json:"workspace_id"`
    Data        json.RawMessage `json:"data"`        // Parse based on Event type
}

// WebhookIssueData represents the data field for issue events.
type WebhookIssueData struct {
    ID         string                `json:"id"`
    Name       string                `json:"name"`
    State      WebhookStateDetail    `json:"state"`
    Assignees  []WebhookAssignee     `json:"assignees"`
    Labels     []WebhookLabel        `json:"labels"`
    Priority   string                `json:"priority"`
    Project    string                `json:"project"`
    SequenceID int                   `json:"sequence_id"`
    CreatedAt  string                `json:"created_at"`
    UpdatedAt  string                `json:"updated_at"`
}

type WebhookStateDetail struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Color string `json:"color"`
    Group string `json:"group"`
}

type WebhookAssignee struct {
    ID          string `json:"id"`
    FirstName   string `json:"first_name"`
    LastName    string `json:"last_name"`
    Email       string `json:"email"`
    DisplayName string `json:"display_name"`
}
```

## Sources

### Primary (HIGH confidence)
- [Plane Webhooks Documentation](https://developers.plane.so/dev-tools/intro-webhooks) - Event types, headers, HMAC signature, payload structure
- [Plane GitHub Issue #6235](https://github.com/makeplane/plane/issues/6235) - Real webhook payload example with nested state/assignee objects
- `pluginapi/cluster/job.go` from `mattermost/server/public` v0.1.21 - Schedule API, Job lifecycle, MakeWaitForRoundedInterval
- `model.PluginKVSetOptions` from `mattermost/server/public` v0.1.21 - ExpireInSeconds for TTL-based KV storage
- Existing codebase: `link_unfurl.go`, `command_handlers.go`, `store/store.go`, `api.go` - Verified patterns for SlackAttachment, channel bindings, HTTP routing

### Secondary (MEDIUM confidence)
- [Plane GitHub Issue #6746](https://github.com/makeplane/plane/issues/6746) - Webhooks may not fire for API-originated changes (open issue, unresolved)
- [Plane GitHub Issue #6848](https://github.com/makeplane/plane/issues/6848) - Duplicate webhook deliveries (known bug)
- [Mattermost HA Plugin Documentation](https://developers.mattermost.com/integrate/plugins/components/server/ha/) - cluster.Schedule pattern for background jobs

### Tertiary (LOW confidence)
- Issue comment webhook payload structure: not fully verified. The `event: "issue_comment"` type is documented but exact field names for comment text content are unconfirmed from official sources. Will need runtime validation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all libraries are already in use or part of the existing module dependency tree
- Architecture: HIGH - webhook receiver pattern is well-documented in Mattermost plugin ecosystem; cluster.Schedule is the blessed approach for background jobs
- Pitfalls: HIGH - Plane webhook limitations (no diff, duplicates, manual setup) are confirmed from official docs and GitHub issues
- Webhook payload structure: MEDIUM - real payload verified from GitHub issue but not from official schema documentation; comment event payload is LOW confidence

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (30 days - stable domain, Plane webhook API is unlikely to change significantly)

---
*Phase: 03-notifications-automation*
*Research completed: 2026-03-17*
