---
phase: 02-channel-intelligence-context-menu
plan: 02
subsystem: api, ui
tags: [mattermost-plugin, plane-api, link-unfurl, slack-attachment, regex, webhook]

requires:
  - phase: 01-foundation
    provides: "Plane API client, plugin scaffold, command handlers, stateGroupEmoji/priorityLabel helpers"
provides:
  - "GetWorkItem API method for fetching single work item by ID"
  - "extractPlaneWorkItemURLs regex-based URL parser for Plane work item URLs"
  - "buildWorkItemAttachment SlackAttachment card builder for work item previews"
  - "MessageHasBeenPosted hook for automatic link unfurling"
  - "handleLinkUnfurl handler with bot-skip, assignee resolution, and error handling"
affects: [02-channel-intelligence-context-menu, 03-context-menu]

tech-stack:
  added: []
  patterns:
    - "MessageHasBeenPosted hook pattern for passive message processing"
    - "UUID regex extraction from message text with QuoteMeta for config-derived patterns"
    - "SlackAttachment card builder pattern with conditional fields"
    - "Assignee name resolution from workspace members cache"

key-files:
  created:
    - server/link_unfurl.go
    - server/link_unfurl_test.go
  modified:
    - server/plane/work_items.go
    - server/plane/types.go
    - server/plane/client_test.go
    - server/plugin.go
    - server/command_handlers.go
    - server/command_test.go
    - server/dialog.go

key-decisions:
  - "Only first Plane URL per message is unfurled to avoid spam"
  - "Assignee resolved via ListWorkspaceMembers cache (not extra API call per user)"
  - "GetWorkItem uses expand=state_detail,project_detail query params for enriched response"
  - "Bot posts skipped via UserId comparison to prevent infinite loops"

patterns-established:
  - "MessageHasBeenPosted hook pattern: check bot skip, extract config, parse URLs, fetch data, build attachment, create reply"
  - "planeURLMatch struct for structured URL extraction results"
  - "buildWorkItemAttachment pattern: conditional fields based on data availability"

requirements-completed: [NOTF-03]

duration: 12min
completed: 2026-03-17
---

# Phase 02 Plan 02: Link Unfurling Summary

**Plane work item URL unfurling via MessageHasBeenPosted hook with rich SlackAttachment preview cards showing title, status, priority, assignee, and project**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-17T09:05:05Z
- **Completed:** 2026-03-17T09:17:09Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- GetWorkItem API method fetches single work items with state/project expand params
- URL extraction via regex handles single, multiple, trailing slash, and partial match cases
- SlackAttachment card builder creates rich preview cards with status emoji, priority label, assignee, and project
- MessageHasBeenPosted hook unfurls first Plane URL in any posted message
- Bot post detection prevents infinite loop (bot never unfurls its own messages)
- Failed API calls silently logged without showing errors to users

## Task Commits

Each task was committed atomically:

1. **Task 1: GetWorkItem API method + URL extraction** (TDD)
   - `2ad90a6` (test: failing tests for GetWorkItem, URL extraction, attachment builder)
   - `7dd90d3` (feat: implement GetWorkItem, extractPlaneWorkItemURLs, buildWorkItemAttachment)

2. **Task 2: MessageHasBeenPosted hook for link unfurling** (TDD)
   - `2850319` (test: failing tests for MessageHasBeenPosted hook)
   - `94b138a` (feat: implement handleLinkUnfurl + MessageHasBeenPosted hook)

## Files Created/Modified
- `server/link_unfurl.go` - URL extraction, attachment builder, handleLinkUnfurl handler
- `server/link_unfurl_test.go` - Tests for URL extraction, attachment builder, MessageHasBeenPosted hook
- `server/plane/work_items.go` - Added GetWorkItem method with expand params
- `server/plane/types.go` - Added AssigneeName field to WorkItem struct
- `server/plane/client_test.go` - Added TestGetWorkItem and TestGetWorkItemNotFound
- `server/plugin.go` - Added MessageHasBeenPosted hook registration
- `server/command_handlers.go` - Fixed pre-existing unused statusSuffix variable
- `server/command_test.go` - Added channel binding KVGet mock to setupMineStatusTestPlugin
- `server/dialog.go` - Added missing store import for ChannelProjectBinding

## Decisions Made
- Only first Plane URL per message is unfurled (avoids spam when multiple URLs pasted)
- Assignee name resolved via cached ListWorkspaceMembers (no extra API call per user)
- GetWorkItem uses `expand=state_detail,project_detail` query params for enriched response data
- Bot's own posts skipped via UserId == botUserID check (prevents infinite unfurl loops)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed pre-existing compilation errors in dialog.go and command_handlers.go**
- **Found during:** Task 2 (MessageHasBeenPosted implementation)
- **Issue:** `dialog.go` missing `store` import for `ChannelProjectBinding` type; `command_handlers.go` had unused `statusSuffix` variable and missing reference. Both from plan 02-01 scaffolding that was auto-generated but incomplete.
- **Fix:** Added store import to dialog.go; removed statusSuffix from handlePlaneStatus (linter restored it as bindingSuffix)
- **Files modified:** server/dialog.go, server/command_handlers.go
- **Verification:** `go vet ./server/` passes
- **Committed in:** 94b138a (Task 2 commit)

**2. [Rule 3 - Blocking] Added channel binding KVGet mock to setupMineStatusTestPlugin**
- **Found during:** Task 2 (running full test suite)
- **Issue:** Pre-existing test helper `setupMineStatusTestPlugin` didn't mock KVGet for `channel_project_*` keys. Auto-generated plan 02-01 code added `GetChannelBinding` calls to `handlePlaneMine` and `handlePlaneStatus` without updating test mocks.
- **Fix:** Added permissive `KVGet` mock matching `channel_project_` prefix returning nil (no binding)
- **Files modified:** server/command_test.go
- **Verification:** TestPlaneMine and TestPlaneStatus pass
- **Committed in:** 94b138a (Task 2 commit)

**3. [Rule 3 - Blocking] External linter repeatedly reverted test file**
- **Found during:** Task 2 (writing test file)
- **Issue:** An external linter/tool kept reverting `link_unfurl_test.go` to skipped stubs and adding `//go:build plan0202` build tag. Required multiple write attempts.
- **Fix:** Persisted with Write tool until content stuck; removed build tag
- **Files modified:** server/link_unfurl_test.go
- **Verification:** All 11 tests run and pass without build tag
- **Committed in:** 94b138a (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (all blocking)
**Impact on plan:** All auto-fixes were necessary to unblock compilation and test execution. No scope creep.

## Issues Encountered
- Pre-existing plan 02-01 tests (`command_handlers_binding_test.go`) fail because link/unlink commands are not yet implemented. These are out of scope and logged as known pre-existing failures.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Link unfurling works for any Plane work item URL in any channel
- Pattern established for MessageHasBeenPosted hook processing
- Ready for plan 02-03 (context menu integration)

## Self-Check: PASSED

All created files verified present. All 4 commit hashes verified in git log.

---
*Phase: 02-channel-intelligence-context-menu*
*Completed: 2026-03-17*
