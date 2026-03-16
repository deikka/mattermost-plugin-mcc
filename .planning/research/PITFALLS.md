# Domain Pitfalls

**Domain:** Mattermost plugin (Go) for chat-to-task integration (Plane API + Obsidian Local REST API)
**Researched:** 2026-03-16

---

## Critical Pitfalls

Mistakes that cause rewrites, data loss, or fundamental architecture failures.

---

### Pitfall 1: Plane API `/issues/` Endpoint Deprecation (Deadline: March 31, 2026)

**What goes wrong:** You build the Plane integration using `/api/v1/.../issues/` endpoints. Plane is actively deprecating these in favor of `/api/v1/.../work-items/`. End of support is **March 31, 2026** -- effectively NOW.

**Why it happens:** Older Plane documentation, blog posts, and community examples still reference `/issues/`. Self-hosted instances on older versions may only have `/issues/`, creating confusion.

**Consequences:** Integration breaks silently when Plane removes legacy endpoints, or works on old self-hosted instance but breaks after Plane upgrade.

**Prevention:**
- Target `/api/v1/.../work-items/` endpoints exclusively from day one
- During development, verify which endpoints your specific self-hosted Plane version exposes
- Build Go struct types with `json:` tags matching `work-items` response shapes
- Add a startup health check that validates the Plane API responds to a `/work-items/` call

**Detection:** 404 responses from Plane API; mismatched field names in responses.

**Confidence:** HIGH (official Plane developer docs confirm deprecation timeline)

**Sources:**
- [Plane API Documentation](https://developers.plane.so/api-reference/introduction)
- [Plane Create Work Item](https://developers.plane.so/api-reference/issue/add-issue) -- note URL still says "issue" but docs say "work-items"

---

### Pitfall 2: Obsidian Local REST API Is Per-User, Per-Machine, and Requires Obsidian Running

**What goes wrong:** You design the integration assuming a single, stable Obsidian endpoint. In reality, each team member runs their own Obsidian instance on their own machine, with their own API key, their own port (default 27124), and their own self-signed TLS certificate. If Obsidian is closed, the API is completely unreachable.

**Why it happens:** Developers coming from server-to-server integration assume the target API is always available. The Obsidian Local REST API is a desktop-app-embedded server that only exists while Obsidian runs.

**Consequences:**
- Plugin must store per-user configuration (host, port, API key) for every team member
- Requests fail unpredictably when users close Obsidian, restart machines, or are offline
- Self-signed HTTPS certificates cause TLS verification failures
- If Mattermost runs on a server, it must reach each user's machine over the network (firewall/NAT/VPN issues)

**Prevention:**
- Design Obsidian integration as "best-effort, user-present" from the start. NEVER assume endpoint is available.
- Store per-user config (host:port + API key) in KV store with key `uo:{user_id}`
- Implement graceful degradation: return helpful ephemeral error "Cannot reach your Obsidian. Is it running with Local REST API enabled?"
- For TLS: configure `crypto/tls` with `InsecureSkipVerify: true` specifically for Obsidian HTTP client instances
- Use the no-auth `GET /` endpoint as a health check before attempting operations
- Consider recommending HTTP mode (port 27123) over HTTPS to avoid certificate complexity

**Detection:** Connection refused, TLS handshake failures, timeouts.

**Confidence:** HIGH (confirmed by Obsidian Local REST API docs and GitHub)

**Sources:**
- [Obsidian Local REST API GitHub](https://github.com/coddingtonbear/obsidian-local-rest-api)
- [DeepWiki - Installation and Configuration](https://deepwiki.com/coddingtonbear/obsidian-local-rest-api)

---

### Pitfall 3: Mattermost Plugin In-Memory State in HA/Multi-Node Deployments

**What goes wrong:** You store channel-project mappings, user preferences, or cached data in Go variables (maps, structs). This works in single-server development. In production with multiple Mattermost nodes (or after plugin restart), all in-memory state vanishes or diverges.

**Why it happens:** Go plugin development feels like a regular Go app where package-level variables are safe. But Mattermost plugins run as subprocesses -- one per server node. Each node's copy is isolated.

**Consequences:**
- Channel-project link set on Node A invisible on Node B
- Cached data diverges between nodes
- Plugin restart wipes all in-memory state

**Prevention:**
- Use KV Store (`KVSet`/`KVGet`) for ALL persistent state from day one, even in single-server dev
- Use `KVCompareAndSet` for state modified concurrently (channel mappings updated by multiple admins)
- For background jobs (periodic summaries), use `pluginapi/cluster.Schedule()` to ensure single-node execution
- Design KV key schema upfront with naming conventions

**Detection:** Works in dev, breaks in production. Intermittent "missing configuration" errors. Background jobs run multiple times.

**Confidence:** HIGH (official Mattermost HA plugin documentation warns about this)

**Sources:**
- [Mattermost Plugin HA Documentation](https://developers.mattermost.com/integrate/plugins/components/server/ha/)
- [Plugin API KV store](https://github.com/mattermost/mattermost-plugin-api/blob/master/kv.go)

---

### Pitfall 4: Plane API Rate Limit of 60 Requests/Minute

**What goes wrong:** Features like "list my tasks," "search tasks," or "project summary" make multiple API calls per user request (fetch project -> fetch work items -> fetch states). A team of 10 using slash commands in a busy channel exhausts 60 req/min, causing 429 errors.

**Why it happens:** 60 req/min is extremely low by API standards. Developers don't plan for aggressive rate management. A single API key shared across the plugin shares one rate limit window.

**Consequences:**
- Users see rate limited errors during peak usage
- Periodic summary jobs consume entire rate budget
- Webhook-triggered callbacks that query Plane API amplify the problem

**Prevention:**
- Implement response caching with TTL: cache project states, labels, member lists for 5-10 minutes (rarely change)
- Use the `?expand=state,assignees,labels` parameter to get related data in a single request instead of multiple round-trips
- Monitor `X-RateLimit-Remaining` and `X-RateLimit-Reset` headers from every Plane response
- For periodic summaries, stagger execution and cache aggressively
- Build all Plane Client methods to handle 429 responses gracefully: log, wait, and optionally retry once

**Detection:** HTTP 429 responses. `X-RateLimit-Remaining` header approaching zero.

**Confidence:** HIGH (confirmed 60/min from official Plane API docs)

**Sources:**
- [Plane API Rate Limits](https://developers.plane.so/api-reference/introduction)

---

## Moderate Pitfalls

Mistakes that cause significant rework, user frustration, or degraded functionality.

---

### Pitfall 5: Plane Webhooks Require Network Reachability from Plane Server

**What goes wrong:** Plane webhooks need the consumer endpoint to be reachable from the Plane server. In a private-network setup where both Mattermost and Plane are self-hosted, the Mattermost plugin's HTTP endpoint must be accessible by the Plane server.

**Why it happens:** Self-hosted doesn't mean same machine. Plane and Mattermost may be on different subnets, Docker networks, or behind different reverse proxies.

**Prevention:**
- Verify network connectivity between Plane server and Mattermost's plugin HTTP endpoint EARLY (before building webhook handlers)
- The webhook URL will be: `https://{mattermost-url}/plugins/com.klab.command-center/webhook/plane`
- Test with `curl` from the Plane server to the Mattermost plugin endpoint
- If using Docker, ensure containers share a network or use proper host mapping
- Implement webhook HMAC-SHA256 signature verification from day one
- Handle retries: Plane retries with exponential backoff. Respond HTTP 200 quickly.

**Detection:** Webhook events never arrive. Check Plane admin panel for failed deliveries.

**Confidence:** HIGH (official Plane webhook documentation)

**Sources:**
- [Plane Webhooks Documentation](https://developers.plane.so/dev-tools/intro-webhooks)

---

### Pitfall 6: Webapp Plugin Features (Context Menus, Buttons) Don't Work on Mobile

**What goes wrong:** You invest effort building React-based post context menus ("Create task from this message"), channel header buttons, and custom post types. None appear on Mattermost mobile clients.

**Why it happens:** The webapp plugin loads only in desktop/browser clients. Mobile clients cannot execute custom JavaScript from plugins.

**Consequences:**
- Mobile users cannot access "Create task from message" -- the most valuable feature
- Must maintain parallel UX path using slash commands for mobile

**Prevention:**
- Design **slash command + interactive dialog** flow as the PRIMARY UX, not an afterthought. This works on ALL clients.
- Treat webapp enhancements (context menu, custom post types) as progressive enhancement for desktop.
- Interactive message buttons (embedded in bot posts) DO work cross-platform -- use them for confirmations and quick actions.
- Test on mobile early.

**Detection:** Any feature relying on `registerPostDropdownMenuAction` or `registerChannelHeaderButtonAction` is desktop-only.

**Confidence:** HIGH (official Mattermost mobile plugin docs)

**Sources:**
- [Mattermost Mobile Plugins](https://developers.mattermost.com/integrate/plugins/components/mobile/)

---

### Pitfall 7: Lost Conversation Context When Creating Tasks

**What goes wrong:** A task created from a chat message contains only the message text, losing surrounding conversation thread, participants, decisions, and related messages. The Plane task becomes a context-free orphan.

**Why it happens:** Natural tendency to map one message to one task. But in chat, context is distributed across multiple messages, threads, and reactions.

**Consequences:**
- Tasks lack context, requiring re-explanation
- Core value proposition ("conversation becomes task without losing context") fails
- Users stop using the integration

**Prevention:**
- Always include a **permalink** back to the original Mattermost message in the task description
- When creating from a thread reply, include the root message + recent thread context
- Include metadata: creator, channel, timestamp, thread participants
- For Plane: use `description_html` to include formatted context block + permanent link
- For Obsidian: include YAML frontmatter with source metadata + link

**Detection:** Tasks consistently lack context compared to manually created ones.

**Confidence:** MEDIUM (derived from chat-to-task integration UX patterns)

---

### Pitfall 8: Obsidian Local REST API + Docker Networking Issues

**What goes wrong:** When Mattermost runs in a Docker container and tries to reach Obsidian Local REST API on the host machine, TCP connects but HTTP responses are empty or hang.

**Why it happens:** Known issue with the Obsidian HTTP server and Docker bridge networking on macOS. TCP layer succeeds but HTTP layer fails silently.

**Consequences:** Integration appears broken with no error message. Extremely difficult to debug.

**Prevention:**
- Test Obsidian connectivity early if Mattermost is containerized
- Use Docker's `host.docker.internal` (macOS/Windows) or `--network host` (Linux)
- Implement connection health checks that validate a FULL HTTP request/response cycle (not just TCP connect)
- Recommend HTTP mode (port 27123) over HTTPS to reduce variables

**Detection:** `curl` from inside container gets "empty reply from server" despite `nc` connecting.

**Confidence:** MEDIUM (confirmed by GitHub discussion, may be OS/Docker-version specific)

**Sources:**
- [Docker connectivity discussion](https://github.com/coddingtonbear/obsidian-local-rest-api/discussions/166)

---

### Pitfall 9: KV Store Has No Query/Index Capability

**What goes wrong:** You store channel-project mappings and user preferences, then need to query "which channels are linked to project X?" or "which users have Obsidian configured?" The KV store only supports key-based lookup -- no query by value, range, or prefix scan.

**Why it happens:** KV store is a simple key-value map, not a database. Developers discover this after building significant state management.

**Consequences:**
- Features like "show all linked channels" require scanning all keys or maintaining manual indexes
- Reverse lookups (project -> channels) are impossible without explicit reverse index keys

**Prevention:**
- Design KV key schema with query patterns in mind from the start:
  - `cp:{channel_id}` -> project info (forward lookup)
  - `pc:{project_id}` -> list of channel_ids (reverse index)
  - `uo:{user_id}` -> Obsidian config
  - `idx:channels` -> list of all channel IDs with mappings (index key)
- Every write to a mapping must ALSO update the reverse index
- Use `KVCompareAndSet` when updating index keys to avoid race conditions
- Keep prefixes short (50-char key limit)

**Detection:** Feature requests requiring "list all X where Y" patterns.

**Confidence:** HIGH (KV store documentation confirms key-only access)

**Sources:**
- [Plugin API KV store](https://github.com/mattermost/mattermost-plugin-api/blob/master/kv.go)

---

### Pitfall 10: Plane Self-Hosted Version May Not Match API Documentation

**What goes wrong:** You develop against Plane API docs (latest SaaS version) but your self-hosted instance is behind. Documented endpoints or fields don't exist.

**Why it happens:** Plane has multiple editions (Community, Commercial, Airgapped) with separate release cycles. API docs don't version-gate content.

**Consequences:** 404s, 400s, missing fields. Work item types or custom properties may not exist on older versions.

**Prevention:**
- Document which Plane version the plugin targets. Set a minimum.
- On plugin startup, make a test API call and log/warn if Plane version is below tested minimum
- Build Plane client with defensive deserialization: `json:",omitempty"` and pointer types for optional fields
- Test against the ACTUAL self-hosted instance, not just API docs

**Detection:** Unexpected API response structure, missing fields, unknown error codes.

**Confidence:** HIGH (confirmed by Plane's edition/versioning docs)

**Sources:**
- [Plane Editions and Versions](https://developers.plane.so/self-hosting/editions-and-versions)

---

## Minor Pitfalls

---

### Pitfall 11: `min_server_version` Mismatch

**What goes wrong:** Plugin built for newer Mattermost features installed on older server. Silently fails or crashes on missing API methods.

**Prevention:** Set `min_server_version` in plugin.json to actual tested version (recommend "10.0.0" minimum for current module paths). Test against production server version.

---

### Pitfall 12: Interactive Dialog Timeout on Slow External APIs

**What goes wrong:** Dialog opens, triggers Plane API call to fetch project list for dropdown. If Plane is slow, dialog submission times out.

**Prevention:**
- Pre-fetch and cache project lists, states, labels (KV store with 5-min TTL) rather than real-time loading
- 10-second HTTP timeout on Plane client
- If cache is empty and Plane is slow, show a "loading failed, try again" ephemeral message

---

### Pitfall 13: Plugin Debugging Requires Disabling Health Checks

**What goes wrong:** During debugging with Delve, you pause at a breakpoint. Mattermost health check pings plugin every 30 seconds. No response = Mattermost kills and restarts plugin.

**Prevention:**
- Set `PluginSettings.EnableHealthCheck: false` in config.json during development
- Use `MM_DEBUG=1 make deploy` for dev builds

**Sources:**
- [Debug Server Plugins](https://developers.mattermost.com/integrate/plugins/components/server/debugging/)

---

### Pitfall 14: OnConfigurationChange Errors Don't Stop the Plugin

**What goes wrong:** `OnConfigurationChange` validates new config. If invalid (empty Plane API key), error is logged but plugin keeps running with invalid config. All subsequent API calls fail with mysterious auth errors.

**Prevention:**
- In `OnConfigurationChange`, validate critical settings and store a "configValid" boolean
- Before ANY external API call, check `configValid` flag
- Log clear warning with admin guidance when config is incomplete
- Implement `/task health` subcommand that validates all connections

---

### Pitfall 15: Plane Webhook Duplicate Events

**What goes wrong:** A single status change in Plane may trigger multiple webhook deliveries (reported as a known issue).

**Prevention:**
- Deduplicate using `X-Plane-Delivery` UUID header
- Store `wd:{delivery_uuid}` in KV with short TTL (1 hour)
- Check for existing key before processing; skip if exists

**Confidence:** MEDIUM (reported in Plane GitHub issues)

---

### Pitfall 16: Obsidian Note Path Sanitization

**What goes wrong:** User creates a task with title containing `/`, `\`, `..`, or special characters. This becomes the Obsidian note file path (`PUT /vault/{path}`), potentially creating files outside intended directory or failing.

**Prevention:**
- Sanitize all user-provided text before using as file paths
- Strip or replace: `/`, `\`, `..`, `<`, `>`, `:`, `"`, `|`, `?`, `*`
- Truncate to reasonable length (200 chars max for filename)
- Always prepend a configured vault path prefix (e.g., "Tasks/")

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| **Phase 1: Plugin skeleton + config** | KV store schema not designed for queries (#9); in-memory state (#3) | Design KV key naming + reverse indexes before writing code |
| **Phase 1: Plane API client** | Rate limit exhaustion (#4); endpoint deprecation (#1); version mismatch (#10) | Use `/work-items/`; implement caching + rate awareness from day one; validate Plane version on startup |
| **Phase 1: Slash commands + dialogs** | Mobile incompatibility (#6); dialog timeouts (#12) | Slash commands as primary UX; pre-cache dropdown data |
| **Phase 2: Channel-project linking** | KV reverse index maintenance (#9); concurrent updates (#3) | Forward + reverse KV keys; KVCompareAndSet |
| **Phase 2: Post action buttons** | Desktop-only (#6); lost context (#7) | Progressive enhancement; always include permalink + thread context |
| **Phase 3: Obsidian integration** | Per-user endpoints (#2); Docker networking (#8); note path sanitization (#16) | "User-present, best-effort" design; test container connectivity; sanitize all paths |
| **Phase 4: Plane webhooks** | Network reachability (#5); duplicate events (#15) | Validate network path early; dedup with delivery UUID |
| **Phase 4: Periodic summaries** | Background job duplication in HA (#3); rate limit budget (#4) | Use `cluster.Schedule()`; reserve rate limit capacity for background jobs |

---

## Sources

- [Mattermost Plugin HA Documentation](https://developers.mattermost.com/integrate/plugins/components/server/ha/)
- [Mattermost Plugin Best Practices](https://developers.mattermost.com/integrate/plugins/best-practices/)
- [Mattermost Mobile Plugins](https://developers.mattermost.com/integrate/plugins/components/mobile/)
- [Mattermost Debug Server Plugins](https://developers.mattermost.com/integrate/plugins/components/server/debugging/)
- [Mattermost Plugin API (Go)](https://pkg.go.dev/github.com/mattermost/mattermost-plugin-api)
- [Mattermost Server Plugin Reference](https://developers.mattermost.com/integrate/reference/server/server-reference/)
- [Plane API Documentation](https://developers.plane.so/api-reference/introduction)
- [Plane Webhooks](https://developers.plane.so/dev-tools/intro-webhooks)
- [Plane Editions and Versions](https://developers.plane.so/self-hosting/editions-and-versions)
- [Obsidian Local REST API GitHub](https://github.com/coddingtonbear/obsidian-local-rest-api)
- [Obsidian Local REST API (DeepWiki)](https://deepwiki.com/coddingtonbear/obsidian-local-rest-api)
- [Obsidian Local REST API Docker Issue](https://github.com/coddingtonbear/obsidian-local-rest-api/discussions/166)
