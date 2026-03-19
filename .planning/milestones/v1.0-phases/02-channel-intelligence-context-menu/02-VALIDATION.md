---
phase: 02
slug: channel-intelligence-context-menu
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-17
validated: 2026-03-19
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 (same as Phase 1) |
| **Config file** | None — Go standard `_test.go` convention |
| **Quick run command** | `go test ./server/... -count=1 -short` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./server/... -count=1 -short`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 02-00-01 | 00 | 0 | BIND-01 | unit | `go test ./server/store/ -run TestChannelBinding -count=1` | ✅ | ✅ green |
| 02-00-02 | 00 | 0 | BIND-01 | unit | `go test ./server/ -run TestPlaneLink -count=1` | ✅ | ✅ green |
| 02-00-03 | 00 | 0 | BIND-02 | unit | `go test ./server/ -run TestBindingAware -count=1` | ✅ | ✅ green |
| 02-00-04 | 00 | 0 | CREA-02 | integration | `go test ./server/ -run TestContextMenuAction -count=1` | ✅ | ✅ green |
| 02-00-05 | 00 | 0 | CREA-03 | unit | `go test ./server/ -run TestDialogPreselect -count=1` | ✅ | ✅ green |
| 02-00-06 | 00 | 0 | NOTF-03 | unit | `go test ./server/ -run TestExtractPlaneURLs -count=1` | ✅ | ✅ green |
| 02-00-07 | 00 | 0 | NOTF-03 | unit | `go test ./server/ -run TestBuildWorkItemAttachment -count=1` | ✅ | ✅ green |
| 02-00-08 | 00 | 0 | NOTF-03 | unit | `go test ./server/plane/ -run TestGetWorkItem -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `server/store/store_test.go` — ChannelProjectBinding CRUD (4 tests: SaveAndGet, GetNotFound, Delete, Overwrite)
- [x] `server/link_unfurl_test.go` — URL extraction (4 tests), attachment building (2 tests), MessageHasBeenPosted (3 tests)
- [x] `server/command_handlers_binding_test.go` — plane link/unlink (4 tests), binding-aware handlers (3 tests), context menu action (3 tests), dialog pre-selection (1 test)
- [x] `server/plane/client_test.go` — GetWorkItem (2 tests: success + not found)
- [x] `webapp/` — no automated testing needed for ~50 lines of JS; manual verification in browser

*Existing infrastructure from Phase 1 covers test helpers and mock Plane server.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Context menu appears in post "..." dropdown | CREA-02 | Webapp UI, requires browser | Install plugin, right-click message "...", verify "Create Task" option appears |
| Dialog pre-populates from message text | CREA-02 | Webapp -> server dialog flow | Click "Create Task" from context menu, verify title/description pre-populated |
| Dialog opens via store.dispatch (no trigger_id) | CREA-02 | Webapp Redux dispatch, requires browser | Click "Create Task", verify dialog modal appears (not a 400 error) |
| Link unfurl card renders correctly | NOTF-03 | Visual rendering in Mattermost | Paste Plane URL in chat, verify attachment card with title/status/assignee renders |
| Emoji reaction appears on source message | CREA-02 | Requires live Mattermost instance | Create task from context menu, verify :memo: reaction on original message |
| Bot announces binding in channel | BIND-01 | Visual + permissions check | Run `/task plane link`, verify bot post visible to all channel members |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** validated 2026-03-19

---

## Validation Audit 2026-03-19

| Metric | Count |
|--------|-------|
| Gaps found | 3 (test assertions using shortcodes instead of Unicode emojis) |
| Resolved | 3 |
| Escalated | 0 |

**Fixes applied:**
- `link_unfurl_test.go`: Updated `TestBuildWorkItemAttachment` to use `🔵` instead of `:large_blue_circle:`
- `command_test.go`: Updated `TestPlaneMine` to use Unicode emojis (`🔵`, `⚪`, `✅`) instead of shortcodes
- `webhook_plane_test.go`: Added `Priority` field to `WorkItemStateCache` in `TestWebhookIssueStateChange` and `TestWebhookAssigneeChange` to match current struct definition
