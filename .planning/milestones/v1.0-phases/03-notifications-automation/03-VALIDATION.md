---
phase: 03
slug: notifications-automation
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-03-17
validated: 2026-03-19
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None (Go convention, `go test ./...`) |
| **Quick run command** | `go test ./server/... -count=1 -short` |
| **Full suite command** | `go test ./server/... -count=1 -v` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./server/... -count=1 -short`
- **After every plan wave:** Run `go test ./server/... -count=1 -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-00-01 | 00 | 0 | NOTF-01 | unit | `go test ./server/ -run TestHandlePlaneWebhook -count=1` | ✅ | ✅ green |
| 03-00-02 | 00 | 0 | NOTF-01 | unit | `go test ./server/ -run TestWebhookIssueStateChange -count=1` | ✅ | ✅ green |
| 03-00-03 | 00 | 0 | NOTF-01 | unit | `go test ./server/ -run TestWebhookAssigneeChange -count=1` | ✅ | ✅ green |
| 03-00-04 | 00 | 0 | NOTF-01 | unit | `go test ./server/ -run TestWebhookIssueComment -count=1` | ✅ | ✅ green |
| 03-00-05 | 00 | 0 | NOTF-01 | unit | `go test ./server/ -run TestWebhookDedup -count=1` | ✅ | ✅ green |
| 03-00-06 | 00 | 0 | NOTF-01 | unit | `go test ./server/ -run TestHandlePlaneNotifications -count=1` | ✅ | ✅ green |
| 03-00-07 | 00 | 0 | NOTF-02 | unit | `go test ./server/ -run TestHandlePlaneDigest -count=1` | ✅ | ✅ green |
| 03-00-08 | 00 | 0 | NOTF-02 | unit | `go test ./server/ -run TestDigest -count=1` | ✅ | ✅ green |
| 03-00-09 | 00 | 0 | NOTF-02 | unit | `go test ./server/store/ -run TestProjectChannel -count=1` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

### Test counts per file

- `webhook_plane_test.go`: 16 tests (HMAC valid/invalid/no-secret, dedup, state change, assignee change, comment, unbound, self-notification, disabled notifications, priority change, target date change)
- `command_handlers_notify_test.go`: 10 tests (notifications on/off/requires-binding/no-args, digest daily/daily-with-hour/weekly/off/requires-binding/invalid-frequency)
- `digest_test.go`: 3 tests (daily execution, not-due-yet, content verification)
- `store/store_test.go` (Phase 3 additions): 4 tests (GetProjectChannels empty, SaveAndGet, AddNoDuplicates, Remove)

---

## Wave 0 Requirements

- [x] `server/webhook_plane_test.go` — NOTF-01: webhook endpoint, state change, assignee change, comment, dedup (16 tests)
- [x] `server/command_handlers_notify_test.go` — NOTF-01/02: notifications on/off, digest config (10 tests)
- [x] `server/digest_test.go` — NOTF-02: digest scheduler execution (3 tests)
- [x] `server/store/store_test.go` — reverse index CRUD (4 tests)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Plane webhook delivery to plugin endpoint | NOTF-01 | Requires real Plane instance with webhook configured | 1. Configure webhook in Plane workspace settings pointing to plugin URL 2. Change task state in Plane 3. Verify notification appears in bound channel |
| Digest posts at configured hour | NOTF-02 | Requires waiting for scheduled time | 1. Set digest to daily with hour = current+1 2. Wait for scheduled time 3. Verify summary appears in channel |
| HMAC secret mismatch rejection in production | NOTF-01 | Requires production network path | 1. Configure wrong secret in System Console 2. Send test webhook from Plane 3. Verify 403 response |

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
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |

All 9 task verification entries already had passing tests. No test fixes required. Full suite: 33 Phase 3 tests green across 4 test files.
