---
phase: 03-notifications-automation
plan: 01
subsystem: api, webhook, notifications
tags: [go, hmac, webhook, slack-attachment, mattermost-plugin, kv-store]

# Dependency graph
requires:
  - phase: 03-notifications-automation
    provides: Webhook types, HMAC verification, reverse index, NotificationConfig CRUD, RED test stubs
provides:
  - Full webhook handler (HMAC verify, dedup, event parsing, routing, notification cards)
  - State change detection via cached state comparison
  - Assignee change detection via sorted ID hash comparison
  - Comment notifications with HTML stripping and text truncation
  - Self-notification suppression via plugin_action_ KV marker
  - /task plane notifications on|off command
affects: [03-notifications-automation]

# Tech tracking
tech-stack:
  added: [crypto/sha512, regexp, io]
  patterns: [webhook-event-routing, state-change-detection-via-cache, assignee-hash-comparison, self-notification-suppression]

key-files:
  created: []
  modified:
    - server/webhook_plane.go
    - server/webhook_plane_test.go
    - server/command_handlers_notify.go
    - server/command_handlers_notify_test.go
    - server/api.go
    - server/plugin.go
    - server/command_handlers.go
    - server/dialog_test.go

key-decisions:
  - "State change detected by comparing cached state group with current webhook state group"
  - "Assignee change detected by comparing sorted ID hash (SHA-512/256) with cached hash"
  - "nil NotificationConfig treated as enabled (default on for bound channels)"
  - "Comment webhook uses issue ID as fallback name since issue name may not be in comment payload"
  - "markPluginAction integrated in both dialog and inline create flows for self-notification suppression"

patterns-established:
  - "Webhook route registered on main router (not auth subrouter) since HMAC replaces Mattermost auth"
  - "Notification cards use SlackAttachment with color #3f76ff and footer Plane"
  - "Cache TTL 7 days for state/assignee caches, 5 minutes for plugin_action_ markers"

requirements-completed: [NOTF-01]

# Metrics
duration: 19min
completed: 2026-03-17
---

# Phase 03 Plan 01: Webhook Handler and Notifications Summary

**Plane webhook receiver with HMAC verification, state/assignee/comment notification cards, self-notification suppression, and /task plane notifications on|off command**

## Performance

- **Duration:** 19 min
- **Started:** 2026-03-17T16:58:13Z
- **Completed:** 2026-03-17T17:17:23Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments
- Full webhook endpoint at /api/v1/webhook/plane with HMAC-SHA256 verification and delivery deduplication
- State change and assignee change detection via cached state comparison in KV store
- Comment notifications with HTML tag stripping and 200-char text truncation
- Self-notification suppression -- plugin-created work items don't trigger webhook notifications
- /task plane notifications on|off command for toggling per-channel notifications
- 20 new/updated tests passing (16 webhook, 4 notification command) with zero t.Skip remaining for 03-01 scope

## Task Commits

Each task was committed atomically (TDD: test then feat):

1. **Task 1: Webhook handler -- HMAC, dedup, event routing, notification cards**
   - `f72c19b` (test) - RED: failing tests for webhook handler
   - `ca707b8` (feat) - GREEN: full webhook implementation
2. **Task 2: Notifications on/off command**
   - `f84ff24` (test) - RED: failing tests for notifications command
   - `5f2d7fe` (feat) - GREEN: notifications command implementation

## Files Created/Modified
- `server/webhook_plane.go` - Full webhook handler: handlePlaneWebhook, handleIssueWebhook, handleIssueCommentWebhook, notification card builders, helper functions
- `server/webhook_plane_test.go` - 16 tests: HMAC verification, dedup, state change, assignee change, comment, unbound project, self-notification, disabled notifications, helper unit tests
- `server/command_handlers_notify.go` - handlePlaneNotifications: validates on/off, checks binding, saves config
- `server/command_handlers_notify_test.go` - 4 tests: on, off, requires binding, no args
- `server/api.go` - Webhook route registration on main router + markPluginAction in dialog create
- `server/plugin.go` - markPluginAction helper with 5-minute TTL
- `server/command_handlers.go` - markPluginAction in inline create
- `server/dialog_test.go` - Added KVSetWithOptions mock for plugin_action_ key

## Decisions Made
- State change detected by comparing cached state group with current webhook state group -- avoids false positives from other state properties changing
- Assignee change detected via sorted ID hash comparison (SHA-512/256) -- deterministic regardless of order, avoids storing full assignee list
- nil NotificationConfig treated as enabled by default -- newly bound channels get notifications without explicit configuration
- Comment webhook uses issue ID as fallback task name since Plane's comment webhook data may not include the parent issue name
- Webhook route registered directly on p.router (not auth subrouter) because webhooks authenticate via HMAC, not Mattermost user session

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added KVSetWithOptions mock for markPluginAction in existing tests**
- **Found during:** Task 1 (webhook handler implementation)
- **Issue:** Adding markPluginAction to CreateWorkItem flows caused TestBindingAwareCreateInline to panic due to unexpected KVSetWithOptions call
- **Fix:** Added permissive KVSetWithOptions mock for plugin_action_ prefix in newPlaneCreateTestPlugin helper
- **Files modified:** server/dialog_test.go
- **Verification:** All existing tests pass without modification
- **Committed in:** ca707b8 (Task 1 feat commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor test mock update required by integrating markPluginAction into existing create flows. No scope creep.

## Issues Encountered
None

## User Setup Required

**External services require manual configuration.** Per the plan's user_setup section:
- Create a webhook in Plane workspace settings (Workspace Settings > Webhooks > Create Webhook)
- Set URL to `{mattermost_site_url}/plugins/com.klab.mattermost-command-center/api/v1/webhook/plane`
- Enable Issue and Issue Comment events
- Copy webhook secret to Mattermost System Console > Plugins > Mattermost Command Center > Plane Webhook Secret

## Next Phase Readiness
- Webhook pipeline complete and ready for production use
- Plan 03-02 (digest scheduler) can proceed -- all infrastructure from 03-00 is in place
- Digest stubs and test stubs remain from 03-00 for Plan 03-02 implementation

## Self-Check: PASSED

All 8 modified files verified on disk. All 4 task commits (f72c19b, ca707b8, f84ff24, 5f2d7fe) found in git log.

---
*Phase: 03-notifications-automation*
*Completed: 2026-03-17*
