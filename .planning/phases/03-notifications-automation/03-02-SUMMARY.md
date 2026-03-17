---
phase: 03-notifications-automation
plan: 02
subsystem: api, scheduler, notifications
tags: [go, cluster-schedule, kv-store, periodic-digest, mattermost-plugin]

# Dependency graph
requires:
  - phase: 03-notifications-automation
    provides: DigestConfig CRUD, store types, command router entries, digest stubs and RED test stubs
provides:
  - /task plane digest daily|weekly|off [hour] command with config persistence
  - Cluster-safe periodic digest scheduler (1-minute interval check)
  - Rich digest posts with state counters, progress bar, and project link
  - Scheduler lifecycle integrated into plugin activation/deactivation
affects: []

# Tech tracking
tech-stack:
  added: [pluginapi/cluster]
  patterns: [cluster-schedule-ha-safe-job, kv-list-scan-for-configs, calendar-hour-dedup]

key-files:
  created: []
  modified:
    - server/digest.go
    - server/digest_test.go
    - server/command_handlers_notify.go
    - server/command_handlers_notify_test.go
    - server/plugin.go
    - server/plugin_test.go

key-decisions:
  - "cluster.Schedule with 1-minute rounded interval for HA-safe single execution across plugin instances"
  - "KVList scan for digest_config_ keys -- acceptable for small number of channels (<20)"
  - "Calendar-hour dedup for daily digests, calendar-day dedup for weekly -- prevents re-posting within same period"
  - "buildDigestPost takes pre-fetched work items slice to enable unit testing without HTTP mocks"
  - "Digest scheduler failure is non-blocking on plugin activation -- logged but doesn't prevent startup"

patterns-established:
  - "Scheduler lifecycle: start on OnActivate, stop on OnDeactivate"
  - "Cluster job pattern: Schedule + MakeWaitForRoundedInterval + Close"
  - "KV timestamp storage as string-encoded Unix seconds for digest_last_ keys"

requirements-completed: [NOTF-02]

# Metrics
duration: 5min
completed: 2026-03-17
---

# Phase 03 Plan 02: Digest Scheduler Summary

**Periodic project digest with configurable daily/weekly schedule via cluster.Schedule, rich state-counter posts, and /task plane digest command**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-17T17:22:03Z
- **Completed:** 2026-03-17T17:27:52Z
- **Tasks:** 1 (TDD: test + feat commits)
- **Files modified:** 6

## Accomplishments
- Full `/task plane digest daily|weekly|off [hour]` command with configurable hour (default 9), binding validation, and Spanish-language confirmation messages
- Cluster-safe digest scheduler using `cluster.Schedule` with 1-minute rounded interval -- only one plugin instance fires digests at a time
- Rich digest posts visible to entire channel: state counters table (Open/In Progress/Done), progress bar, project link, and footer with disable instructions
- Calendar-hour dedup for daily and calendar-day dedup for weekly prevents re-posting within the same period
- 9 new tests passing (7 command + 3 execution/content), zero t.Skip remaining, full test suite green

## Task Commits

Each task was committed atomically (TDD: test then feat):

1. **Task 1: Digest command and scheduler implementation**
   - `a5df708` (test) - RED: failing tests for digest command and scheduler
   - `f753111` (feat) - GREEN: full implementation of command, scheduler, and content builder

## Files Created/Modified
- `server/digest.go` - Full implementation: startDigestScheduler, stopDigestScheduler, runDigestCheck, isDigestDue, isDigestAlreadyPosted, buildDigestPost
- `server/digest_test.go` - 3 tests: daily execution fires post, not-due-yet skips, content verification with state counters
- `server/command_handlers_notify.go` - handlePlaneDigest: parses frequency + hour, validates binding, saves DigestConfig, returns confirmation
- `server/command_handlers_notify_test.go` - 7 tests: daily, daily+hour, weekly, off, requires binding, invalid frequency (replaced 3 t.Skip stubs)
- `server/plugin.go` - Added digestJob field (cluster.Job), startDigestScheduler in OnActivate, OnDeactivate method
- `server/plugin_test.go` - Added KV mocks for cluster.Schedule goroutine, OnDeactivate cleanup in OnActivate tests

## Decisions Made
- cluster.Schedule with 1-minute rounded interval for HA-safe single execution -- prevents duplicate digests across clustered plugin instances
- KVList scan for digest_config_ prefixed keys -- simple and acceptable for the expected small number of channels
- Calendar-hour dedup for daily, calendar-day dedup for weekly -- uses string-encoded Unix timestamp in digest_last_ KV keys
- buildDigestPost accepts pre-fetched work items to enable direct unit testing without HTTP server mocks
- Digest scheduler failure is non-blocking on plugin activation -- digest is a convenience feature, not critical path

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Added KV mocks for cluster.Schedule in existing OnActivate tests**
- **Found during:** Task 1 (GREEN phase)
- **Issue:** Adding startDigestScheduler to OnActivate caused TestOnActivate to panic -- cluster.Schedule spawns a goroutine that calls KVGet("cron_PlaneDigestScheduler") on the mock API without matching expectations
- **Fix:** Added permissive KVGet, KVDelete, KVList mocks to setupActivatedPlugin and OnDeactivate cleanup in OnActivate tests
- **Files modified:** server/plugin_test.go
- **Verification:** All existing OnActivate tests pass without modification
- **Committed in:** f753111 (Task 1 feat commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor test mock update required by integrating cluster.Schedule into OnActivate. No scope creep.

## Issues Encountered
None

## User Setup Required
None - digest is configured per-channel via `/task plane digest` command.

## Next Phase Readiness
- All Phase 3 plans complete (03-00 infrastructure, 03-01 webhooks, 03-02 digest)
- Full notification and automation pipeline operational
- No remaining t.Skip stubs in any test file
- Complete test suite green across all phases

## Self-Check: PASSED
