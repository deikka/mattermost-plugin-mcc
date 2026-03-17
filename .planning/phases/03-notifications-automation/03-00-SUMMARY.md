---
phase: 03-notifications-automation
plan: 00
subsystem: api, database, testing
tags: [go, hmac, kv-store, webhook, mattermost-plugin]

# Dependency graph
requires:
  - phase: 02-channel-intelligence
    provides: Channel-project binding CRUD, store patterns, command router
provides:
  - Webhook payload types (PlaneWebhookEvent, WebhookIssueData, WebhookCommentData)
  - HMAC signature verification for incoming webhooks
  - Reverse index (project -> channels) with automatic maintenance on bind/unbind
  - NotificationConfig, DigestConfig, WorkItemStateCache store types with CRUD
  - PlaneWebhookSecret in plugin configuration and settings_schema
  - RED test stubs for all Phase 3 behaviors (17 skipped tests)
  - Command router entries for plane/notifications and plane/digest
affects: [03-notifications-automation]

# Tech tracking
tech-stack:
  added: [crypto/hmac, crypto/sha256]
  patterns: [reverse-index-on-binding, webhook-dedup-with-ttl, hmac-signature-verification]

key-files:
  created:
    - server/webhook_plane.go
    - server/webhook_plane_test.go
    - server/command_handlers_notify.go
    - server/command_handlers_notify_test.go
    - server/digest.go
    - server/digest_test.go
  modified:
    - server/store/store.go
    - server/store/store_test.go
    - server/configuration.go
    - plugin.json
    - server/command_router.go
    - server/command.go
    - server/command_handlers.go
    - server/command_handlers_binding_test.go

key-decisions:
  - "Reverse index maintained automatically in SaveChannelBinding/DeleteChannelBinding (not separate operation)"
  - "HMAC verification is permissive when PlaneWebhookSecret is empty (accepts all)"
  - "Webhook dedup uses KVSetWithOptions with 1-hour TTL expiry"
  - "Webhook types use nested structs matching Plane's JSON structure (not flat fields like plane.WorkItem)"

patterns-established:
  - "Reverse index pattern: forward binding write also updates project_channels_ reverse index"
  - "Webhook signature verification: HMAC-SHA256 with hex-encoded digest comparison"
  - "RED test stubs: t.Skip with plan reference for future implementation"

requirements-completed: [NOTF-01, NOTF-02]

# Metrics
duration: 7min
completed: 2026-03-17
---

# Phase 03 Plan 00: Wave 0 Infrastructure Summary

**Webhook types, reverse index CRUD, HMAC verification, KV store extensions, and RED test stubs for Phase 3 notifications/automation**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-17T16:45:56Z
- **Completed:** 2026-03-17T16:53:33Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments
- New Go types for webhook payloads matching Plane's nested JSON structure (PlaneWebhookEvent, WebhookIssueData, WebhookCommentData)
- Reverse index (project -> channels) with automatic maintenance on SaveChannelBinding/DeleteChannelBinding
- HMAC-SHA256 webhook signature verification with permissive mode when no secret configured
- NotificationConfig, DigestConfig, WorkItemStateCache store types with full CRUD
- PlaneWebhookSecret added to plugin configuration and settings_schema
- 17 RED test stubs for Plans 03-01 and 03-02, plus 3 real HMAC verification tests
- Command router extended with plane/notifications and plane/digest entries

## Task Commits

Each task was committed atomically:

1. **Task 1: Store types, reverse index CRUD, configuration, and settings_schema** - `ec6c917` (feat)
2. **Task 2: Webhook types, handler stubs, command stubs, digest stubs, and RED test files** - `57bf4a7` (feat)

## Files Created/Modified
- `server/store/store.go` - Added Phase 3 types (NotificationConfig, DigestConfig, WorkItemStateCache), reverse index CRUD, 7 new KV prefixes
- `server/store/store_test.go` - Added 10 new tests for reverse index and config CRUD, updated binding tests for reverse index
- `server/configuration.go` - Added PlaneWebhookSecret field to configuration struct
- `plugin.json` - Added PlaneWebhookSecret to settings_schema with secret flag
- `server/webhook_plane.go` - Webhook payload types, HMAC verification, dedup helpers
- `server/webhook_plane_test.go` - 3 real HMAC tests + 8 skipped stubs
- `server/command_handlers_notify.go` - handlePlaneNotifications and handlePlaneDigest stubs
- `server/command_handlers_notify_test.go` - 6 skipped stubs
- `server/digest.go` - startDigestScheduler, stopDigestScheduler, runDigestCheck stubs
- `server/digest_test.go` - 4 skipped stubs
- `server/command_router.go` - Added plane/notifications, plane/digest entries and aliases
- `server/command.go` - Added autocomplete entries for notifications and digest
- `server/command_handlers.go` - Updated help text with new commands
- `server/command_handlers_binding_test.go` - Added reverse index mock expectations

## Decisions Made
- Reverse index maintained automatically inside SaveChannelBinding/DeleteChannelBinding, not as separate operations -- keeps the index always consistent
- HMAC verification is permissive when PlaneWebhookSecret is empty -- allows initial setup without webhook secret
- Webhook dedup uses KVSetWithOptions with 1-hour TTL expiry -- auto-cleans without background task
- Webhook types use nested structs (WebhookStateDetail, WebhookAssignee, etc.) matching Plane's actual webhook JSON, separate from the flat plane.WorkItem type used by the API client

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All types compile and are ready for Plan 03-01 (webhook handler implementation) and Plan 03-02 (digest scheduler)
- RED test stubs define the behavioral contracts for both plans
- Reverse index enables webhook event routing to bound channels
- PlaneWebhookSecret is ready for admin configuration in System Console

## Self-Check: PASSED

All 6 created files verified on disk. Both task commits (ec6c917, 57bf4a7) found in git log.

---
*Phase: 03-notifications-automation*
*Completed: 2026-03-17*
