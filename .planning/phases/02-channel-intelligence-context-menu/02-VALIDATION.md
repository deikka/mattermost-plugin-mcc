---
phase: 02
slug: channel-intelligence-context-menu
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
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
| 02-00-01 | 00 | 0 | BIND-01 | unit | `go test ./server/store/ -run TestChannelBinding -count=1` | ❌ W0 | ⬜ pending |
| 02-00-02 | 00 | 0 | BIND-01 | unit | `go test ./server/ -run TestPlaneLink -count=1` | ❌ W0 | ⬜ pending |
| 02-00-03 | 00 | 0 | BIND-02 | unit | `go test ./server/ -run TestBindingAware -count=1` | ❌ W0 | ⬜ pending |
| 02-00-04 | 00 | 0 | CREA-02 | integration | `go test ./server/ -run TestContextMenuAction -count=1` | ❌ W0 | ⬜ pending |
| 02-00-05 | 00 | 0 | CREA-03 | unit | `go test ./server/ -run TestDialogPreselect -count=1` | ❌ W0 | ⬜ pending |
| 02-00-06 | 00 | 0 | NOTF-03 | unit | `go test ./server/ -run TestExtractPlaneURLs -count=1` | ❌ W0 | ⬜ pending |
| 02-00-07 | 00 | 0 | NOTF-03 | unit | `go test ./server/ -run TestLinkUnfurl -count=1` | ❌ W0 | ⬜ pending |
| 02-00-08 | 00 | 0 | NOTF-03 | unit | `go test ./server/plane/ -run TestGetWorkItem -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `server/store/store_test.go` — add tests for ChannelProjectBinding CRUD
- [ ] `server/link_unfurl_test.go` — test URL extraction, attachment building, MessageHasBeenPosted logic
- [ ] `server/command_handlers_binding_test.go` — add tests for plane link/unlink, binding-aware handlers, context menu action, dialog pre-selection
- [ ] `server/plane/client_test.go` — add test for GetWorkItem
- [ ] `webapp/` — no automated testing needed for ~50 lines of JS; manual verification in browser

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

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
