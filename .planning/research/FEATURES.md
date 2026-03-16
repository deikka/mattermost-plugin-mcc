# Feature Landscape

**Domain:** Chat-to-task integration plugin (Mattermost + Plane + Obsidian)
**Researched:** 2026-03-16

## Table Stakes

Features users expect from a chat-to-task Mattermost integration. Missing = product feels broken. Derived from analyzing Mattermost-Jira, Mattermost-GitHub, Mattermost-ServiceNow plugins, and the community Todo plugin.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Slash command to create Plane task** | Every Mattermost integration uses slash commands as primary interface. Jira: `/jira issue create`. GitHub: `/github todo`. Core value proposition. | Medium | `/task plane create "Title"` with optional `--priority high --assignee @user`. Opens interactive dialog with fields (title, description, project, assignee, priority). |
| **Post context menu "Create Task in Plane"** | Jira plugin's most-used feature: click "..." on message -> "Create Jira Issue". This is the core value -- turning conversations into tasks without typing commands. | Medium | Register `postDropdownMenuAction` in webapp. Pre-populate description with message text + permalink. Note: does NOT work on mobile clients. |
| **Interactive dialog for task creation** | Jira and ServiceNow both open structured forms. Users need to select project, set priority, assign. Raw slash command text is insufficient for structured data. | Medium | Mattermost supports text, textarea, select, bool fields. Pre-fill project from channel mapping when available. |
| **Account/connection setup** (`/task connect`) | Every official plugin requires users to connect their external account first. Standard pattern. | Medium | Plane: admin configures API key in System Console. Obsidian: each user runs `/task obsidian setup` to provide their host:port + API key. Stored encrypted in KV store per user. |
| **View my tasks** (`/task plane mine`) | GitHub has `/github todo`. Jira shows assigned issues. Users need to see their work without leaving chat. | Low | Plane API: `GET /work-items/?assignee={user_id}`. Return as ephemeral message (visible only to requester). |
| **Help command** (`/task help`) | Every plugin has help. Essential for discoverability. | Low | List available subcommands, brief descriptions, link to setup docs. |
| **Ephemeral responses for personal queries** | All Mattermost plugins return personal data as ephemeral messages. Posting someone's task list to a public channel would be noisy and inappropriate. | Low | Use `SendEphemeralPost` API. Only notifications and shared operations post to channel. |
| **Slash command autocomplete** | GitHub and Jira plugins support autocomplete. Users expect tab-completion for subcommands. | Low | Register commands with `AutoComplete: true` and provide `AutoCompleteHint` strings. |
| **Actionable error messages** | When API fails, auth expires, or Obsidian is offline, tell the user what to do. "Connection to Plane failed. Check the Plane URL in System Console > Plugins > Command Center." | Low | Pattern from Jira plugin: include the fix action in the error message. |
| **Plugin configuration via System Console** | Admins expect GUI configuration, not manual config file editing. | Low | Plane URL, API key, workspace slug in `settings_schema`. Fields marked `secret: true` are masked. |

## Differentiators

Features that set this plugin apart from basic webhook integrations. Not expected but valued for a 2-10 person team using Mattermost as command center.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Channel-to-project binding** (`/task plane link`) | Core differentiator. Channel represents a project context; all task commands default to that project. Eliminates repetitive project selection. GitHub plugin has `/github default-repo` -- follow this pattern for Plane projects. | Medium | Store channel-project mapping in KV store. Maintain reverse index `project_channels:{project_id}` for webhook routing. |
| **Automatic project routing** | Tasks created in a linked channel auto-assign to the bound Plane project without selection. Friction drops from 5 clicks to 1. | Low | Depends on channel-project binding. Read binding from KV, pre-fill project in dialog. |
| **Plane webhook notifications in linked channels** | When a task changes status, gets assigned, or receives a comment in Plane, post update to the linked Mattermost channel. Closes the feedback loop. | High | Plugin receives webhooks via `ServeHTTP` at `/plugins/{id}/webhook/plane`. Verify HMAC signature. Route to channel via project-channel reverse index. Plane supports Issue, Project, Cycle, Module, Comment webhook events. |
| **Dual-target creation** (Plane + Obsidian from same message) | Unique to this plugin: one message can create a Plane issue for the team AND an Obsidian note for personal reference. No existing tool does this. | Medium | Two separate context menu actions or a combined dialog with "destination" selector. Different API calls, same source message context. |
| **Obsidian note creation from messages** | Personal knowledge capture directly from team conversation. Each user saves to their own vault. | Medium | Per-user Obsidian endpoint config. `PUT /vault/Tasks/{title}.md` with markdown content including frontmatter (source, author, channel, timestamp, Mattermost link). |
| **Project status summary** (`/task plane status`) | Quick health check: open/in-progress/done counts, recent activity. Saves opening Plane. | Medium | Aggregate Plane API calls (list work items by state). Format as rich message attachment with fields. Cache aggressively (project states/labels change rarely). |
| **Search Plane tasks** (`/task plane search "query"`) | Find tasks by title/description without leaving chat. | Low | Plane API supports search. Return top results as interactive message. |
| **Periodic project digest** | Automated daily/weekly summary of linked project health posted to channel. | High | Requires scheduled goroutine in plugin (use `cluster.Schedule()` for HA safety). Configurable frequency per channel binding. Plane API rate limit (60/min) is the constraint -- stagger and cache. |
| **Task preview on Plane URL paste** (link unfurling) | When someone pastes a Plane issue URL, show inline preview with title, status, assignee. GitHub and Jira plugins do this. | Medium | Register `MessageWillBePosted` hook. Parse Plane URLs with regex. Fetch issue details. Return as message attachment. |

## Anti-Features

Features to explicitly NOT build. Based on project scope, team size, and avoiding complexity traps.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| **Bidirectional Obsidian-Plane sync** | Explicitly out of scope (PROJECT.md). Different tools, different data models. Sync = conflict resolution nightmare. | Keep as separate targets. User consciously chooses destination. |
| **Edit tasks from Mattermost** | PROJECT.md: v1 is creation and consultation only. Editing UX in chat is poor (limited form fields, no inline editing). High API surface area. | Create and view only. Include "Open in Plane" link for edits. |
| **Full kanban/board in Mattermost** | Chat window is not a project management UI. Building mini-Plane inside Mattermost duplicates effort with worse UX. | Provide summary views and links. List format, not board. |
| **AI-powered task extraction** | Auto-detecting "tasks" in conversation creates false positives, user distrust, and noise. For 2-10 people, explicit creation is fast enough. | Manual trigger (context menu, slash command) with message text pre-populated. User confirms before creating. |
| **Multi-instance Plane support** | Enterprise complexity for a small team. Significant config overhead. | Single Plane instance + single workspace. Multiple projects within that workspace. |
| **Obsidian vault browsing from Mattermost** | Vaults are personal. Exposing vault structure via chat is a security/privacy concern. Local REST API is per-user by design. | Users can only interact with their own vault. Create and search only. |
| **Custom webhook configuration UI** | High effort admin panel for something configurable via Plane's own UI + a documented setup process. | Document webhook setup in plugin README. Admin configures in Plane UI, pastes secret into System Console. |
| **Mobile-specific UI** | PROJECT.md excludes mobile app. Mattermost mobile renders slash commands and interactive messages natively. | Ensure all core interactions use slash commands + interactive dialogs (work on all clients). Post action menu items are desktop-only progressive enhancement. |
| **Notification deduplication with read receipts** | Tracking "seen" state in chat is complex and unreliable. Over-engineering for a small team. | Post notifications to channel. Users manage attention via Mattermost's native notification preferences. |

## Feature Dependencies

```
Plugin scaffold + config (System Console)
  |
  +-> Bot Account creation (OnActivate)
  |     |
  |     +-> All message posting (confirmations, errors, notifications)
  |
  +-> Plane API Client wrapper
  |     |
  |     +-> /task plane create (slash command + dialog)
  |     |     |
  |     |     +-> Post context menu "Create Task in Plane" (webapp registration)
  |     |           |
  |     |           +-> Message text + permalink pre-population
  |     |
  |     +-> /task plane mine (list my tasks)
  |     |
  |     +-> /task plane search (search by text)
  |     |
  |     +-> /task plane status (project summary)
  |
  +-> Channel-to-Project Binding (/task plane link)
  |     |
  |     +-> Automatic project routing (commands default to bound project)
  |     |
  |     +-> Plane webhook routing (events -> correct channel)
  |     |     |
  |     |     +-> Webhook receiver (ServeHTTP + HMAC verification)
  |     |
  |     +-> Periodic digest (scheduled summaries per binding)
  |
  +-> Per-User Config in KV Store
        |
        +-> Obsidian API Client wrapper
        |     |
        |     +-> /task obsidian create (slash command)
        |     |     |
        |     |     +-> Post context menu "Save to Obsidian" (webapp)
        |     |
        |     +-> /task obsidian setup (store user endpoint + key)
        |
        +-> (Future) Per-user Plane API keys if needed

Link Unfurling (Plane URL previews)
  [Independent -- only needs Plane API connection, triggers on MessageWillBePosted]
```

## MVP Recommendation

**Phase 1 -- Foundation + Core Plane Integration:**
1. Plugin scaffold (starter template, manifest, settings_schema)
2. Bot account creation in OnActivate
3. Plane API client wrapper (list projects, create/list/get work items)
4. KV store layer with key schema design (channel mappings, user configs)
5. `/task plane create` slash command with interactive dialog
6. `/task plane mine` -- list my assigned tasks (ephemeral)
7. `/task help` -- command listing
8. Ephemeral error messages with actionable guidance

**Phase 2 -- Channel Intelligence + Post Actions:**
9. Channel-to-project binding (`/task plane link`)
10. Automatic project routing (commands default to bound project)
11. Post context menu "Create Task in Plane" (webapp component)
12. `/task plane search "query"` -- search tasks
13. `/task plane status` -- project summary

**Phase 3 -- Obsidian Integration:**
14. Per-user Obsidian configuration (`/task obsidian setup`)
15. Obsidian API client wrapper (create note, health check)
16. `/task obsidian create` slash command
17. Post context menu "Save to Obsidian" (webapp)
18. Graceful handling when Obsidian is offline

**Phase 4 -- Notifications + Polish:**
19. Plane webhook receiver with HMAC verification
20. Webhook event routing to linked channels
21. Link unfurling for Plane URLs
22. Periodic project digest (if rate limits allow)

**Defer to v2:**
- Right-hand sidebar panel (full React frontend -- high effort, low priority for 2-10 users)
- Per-user Plane API keys (single admin key sufficient at this scale)

## Sources

- [Mattermost Jira Plugin](https://docs.mattermost.com/integrations-guide/jira.html) - feature pattern reference
- [Mattermost GitHub Plugin](https://docs.mattermost.com/integrations-guide/github.html) - default-repo per channel pattern
- [Mattermost Interactive Messages](https://developers.mattermost.com/integrate/plugins/interactive-messages/) - button/action JSON structure
- [Mattermost Interactive Dialogs](https://developers.mattermost.com/integrate/plugins/interactive-dialogs/) - form field types
- [Mattermost Webapp Plugin Reference](https://developers.mattermost.com/integrate/reference/webapp/webapp-reference/) - registerPostDropdownMenuAction
- [Plane API Reference](https://developers.plane.so/api-reference/introduction) - available endpoints and scopes
- [Plane Create Work Item](https://developers.plane.so/api-reference/issue/add-issue) - request/response format
- [Plane List Work Items](https://developers.plane.so/api-reference/issue/list-issues) - filtering and expand params
- [Plane Webhooks](https://developers.plane.so/dev-tools/intro-webhooks) - supported events and verification
- [Obsidian Local REST API](https://github.com/coddingtonbear/obsidian-local-rest-api) - all endpoints
- [Obsidian Local REST API Docs](https://deepwiki.com/coddingtonbear/obsidian-local-rest-api) - detailed endpoint reference
- [Mattermost Plugin Demo](https://github.com/mattermost/mattermost-plugin-demo) - implementation patterns
- [Mattermost Plugin KV Store](https://github.com/mattermost/mattermost-plugin-api/blob/master/kv.go) - storage API
