---
phase: 01-foundation-core-plane-commands
plan: 02
subsystem: plane-api-client
tags: [go, plane-api, kv-store, slash-commands, email-match, ttl-cache, interactive-dialog, http-api]

requires:
  - phase: 01-01
    provides: "Plugin scaffold with command routing, bot account, System Console settings"
provides:
  - "Plane API HTTP client with auth, caching, CRUD for projects/states/labels/members/work-items"
  - "In-memory TTL cache (5min projects/labels/members, 10min states) reducing Plane API calls"
  - "KV store package for user-Plane mapping and Obsidian config persistence"
  - "/task connect command with automatic email-based Plane user matching"
  - "/task obsidian setup command with interactive dialog (host/port/api_key)"
  - "requirePlaneConnection guard function blocking unconnected users"
  - "HTTP API endpoints for dialog dynamic selects (projects, members, labels)"
  - "Obsidian setup dialog submission handler with port validation"
affects: [01-03]

tech-stack:
  added: [plane-api-client, kv-store-package, ttl-cache]
  patterns: [cached-api-calls, email-based-user-matching, interactive-dialog-submission, mattermost-auth-middleware, dynamic-select-endpoints]

key-files:
  created:
    - server/plane/cache.go
    - server/plane/projects.go
    - server/plane/work_items.go
    - server/api.go
  modified:
    - server/plane/types.go
    - server/plane/client.go
    - server/plane/client_test.go
    - server/store/store.go
    - server/store/store_test.go
    - server/command_handlers.go
    - server/command_test.go
    - server/plugin.go
    - server/configuration.go

key-decisions:
  - "Workspace members API returns direct array (not paginated) -- handled differently from project-scoped endpoints"
  - "MemberWrapper struct separates member metadata from user details matching Plane response nesting"
  - "Cache uses type assertions at caller sites (cache stores interface{}) -- simple and sufficient for single-process plugin"
  - "OnConfigurationChange syncs planeClient config and invalidates cache on admin settings changes"
  - "Email matching is case-insensitive via strings.EqualFold for robustness"

patterns-established:
  - "Cached Plane API call: check cache -> API call -> decode -> cache result -> return"
  - "Dialog submission handler: decode SubmitDialogRequest -> validate -> save -> ephemeral confirm -> return 200"
  - "Mattermost auth middleware: validate Mattermost-User-Id header -> 401 if empty"
  - "requirePlaneConnection guard: check store -> block with guidance if unconnected"
  - "Dynamic select response: [{text: string, value: string}] format for dialog selects"

requirements-completed: [CONF-02, CONF-03]

duration: 8min
completed: 2026-03-17
---

# Phase 1 Plan 02: Plane API Client + KV Store + Connect/Obsidian Commands Summary

**Plane API client with TTL caching, KV store for user mappings, /task connect via email match, /task obsidian setup dialog, and HTTP API endpoints for dynamic selects**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-17T07:04:58Z
- **Completed:** 2026-03-17T07:13:17Z
- **Tasks:** 3
- **Files modified:** 12

## Accomplishments
- Full Plane API client with authenticated requests, typed responses, and in-memory TTL cache
- KV store persists user-Plane mappings and Obsidian REST API configuration per user
- /task connect automatically matches Mattermost email to Plane workspace member and links accounts
- /task obsidian setup opens interactive dialog with host/port/API key fields and validates+saves on submit
- HTTP API endpoints serve dynamic select data for projects, members, labels in dialog format
- 33 passing unit tests across plane client, store, and command handlers (14 plane + 12 store + 7 command)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Plane API client package with types, CRUD operations, and cache** - `23b2448` (feat)
2. **Task 2: Create KV store package and implement /task connect and /task obsidian setup handlers** - `639e1c8` (feat)
3. **Task 3: Wire HTTP API endpoints for dialog dynamic selects and obsidian setup submission** - `b2d46cd` (feat)

## Files Created/Modified
- `server/plane/types.go` - All Plane API structs: WorkItem, Project, State, Label, Member, MemberWrapper, PaginatedResponse, APIError
- `server/plane/client.go` - HTTP client with doRequest, parseAPIError, UpdateConfig, InvalidateCache, GetWorkItemURL
- `server/plane/cache.go` - In-memory TTL cache with Get/Set/Invalidate/InvalidateAll
- `server/plane/projects.go` - ListProjects, ListProjectStates, ListProjectLabels, ListProjectMembers, ListWorkspaceMembers
- `server/plane/work_items.go` - CreateWorkItem, ListWorkItems
- `server/plane/client_test.go` - 14 tests covering all client operations, cache, errors, config
- `server/store/store.go` - KV store with user_plane_ and user_obsidian_ prefixed keys (unchanged from Wave 0)
- `server/store/store_test.go` - 12 tests covering all CRUD operations and edge cases
- `server/command_handlers.go` - handleConnect (email match), handleObsidianSetup (dialog), requirePlaneConnection guard
- `server/command_test.go` - 7 new tests for connect/obsidian/guard functionality
- `server/plugin.go` - Added planeClient and store fields, initialized in OnActivate
- `server/configuration.go` - OnConfigurationChange syncs planeClient and invalidates cache
- `server/api.go` - HTTP routes for select/projects, select/members, select/labels, dialog/obsidian-setup, dialog/create-task

## Decisions Made
- **Workspace members uses direct array decode**: The Plane workspace members endpoint returns a plain JSON array rather than the paginated `{results: [...]}` wrapper used by project-scoped endpoints. ListWorkspaceMembers handles this correctly.
- **MemberWrapper struct for nested member data**: Plane returns `{member: {id, email, ...}, role: N}` for members. MemberWrapper holds this structure while Member holds the user details.
- **Case-insensitive email matching**: `strings.EqualFold` used in handleConnect to handle email casing differences between Mattermost and Plane.
- **Cache invalidation on config change**: When admin changes Plane settings in System Console, OnConfigurationChange calls UpdateConfig and InvalidateCache to prevent stale data.

## Deviations from Plan

None - plan executed exactly as written. The store package and types were partially pre-built in Wave 0, which accelerated Task 2.

## Issues Encountered
None - all code compiled on first attempt and tests passed without debugging.

## User Setup Required

None - no external service configuration required at this stage.

## Next Phase Readiness
- Plane API client ready for Plan 01-03 to use CreateWorkItem and ListWorkItems
- requirePlaneConnection guard ready for Plan 01-03 command handlers
- HTTP API endpoints ready for dialog dynamic selects in Plan 01-03
- handleCreateTaskDialog stub ready to be implemented in Plan 01-03

## Self-Check: PASSED

- All 13 key files exist on disk
- All 3 task commits verified in git history (23b2448, 639e1c8, b2d46cd)
- All tests pass: `go test ./server/... -count=1 -short` (3 packages, 0 failures)
- Verification checklist confirmed: X-API-Key auth, /work-items/ endpoints, correct KV prefixes, Mattermost-User-Id middleware, correct cache TTLs

---
*Phase: 01-foundation-core-plane-commands*
*Completed: 2026-03-17*
