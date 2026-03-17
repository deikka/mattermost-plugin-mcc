---
phase: 03
slug: notifications-automation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | None (Go convention, `go test ./...`) |
| **Quick run command** | `cd server && go test ./... -count=1 -short` |
| **Full suite command** | `cd server && go test ./... -count=1 -v` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd server && go test ./... -count=1 -short`
- **After every plan wave:** Run `cd server && go test ./... -count=1 -v`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 03-00-01 | 00 | 0 | NOTF-01 | unit | `cd server && go test -run TestHandlePlaneWebhook -count=1` | ❌ W0 | ⬜ pending |
| 03-00-02 | 00 | 0 | NOTF-01 | unit | `cd server && go test -run TestWebhookIssueStateChange -count=1` | ❌ W0 | ⬜ pending |
| 03-00-03 | 00 | 0 | NOTF-01 | unit | `cd server && go test -run TestWebhookAssigneeChange -count=1` | ❌ W0 | ⬜ pending |
| 03-00-04 | 00 | 0 | NOTF-01 | unit | `cd server && go test -run TestWebhookIssueComment -count=1` | ❌ W0 | ⬜ pending |
| 03-00-05 | 00 | 0 | NOTF-01 | unit | `cd server && go test -run TestWebhookDedup -count=1` | ❌ W0 | ⬜ pending |
| 03-00-06 | 00 | 0 | NOTF-01 | unit | `cd server && go test -run TestHandlePlaneNotifications -count=1` | ❌ W0 | ⬜ pending |
| 03-00-07 | 00 | 0 | NOTF-02 | unit | `cd server && go test -run TestHandlePlaneDigest -count=1` | ❌ W0 | ⬜ pending |
| 03-00-08 | 00 | 0 | NOTF-02 | unit | `cd server && go test -run TestDigestExecution -count=1` | ❌ W0 | ⬜ pending |
| 03-00-09 | 00 | 0 | NOTF-02 | unit | `cd server && go test -run TestReverseIndex -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `server/webhook_plane_test.go` — stubs for NOTF-01a through NOTF-01e (webhook endpoint, state change, assignee change, comment, dedup)
- [ ] `server/command_handlers_notify_test.go` — stubs for NOTF-01f, NOTF-02a (notifications on/off, digest config)
- [ ] `server/digest_test.go` — stubs for NOTF-02b (digest scheduler execution)
- [ ] `server/store/store_test.go` (additions) — stubs for NOTF-02c (reverse index CRUD)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Plane webhook delivery to plugin endpoint | NOTF-01 | Requires real Plane instance with webhook configured | 1. Configure webhook in Plane workspace settings pointing to plugin URL 2. Change task state in Plane 3. Verify notification appears in bound channel |
| Digest posts at configured hour | NOTF-02 | Requires waiting for scheduled time | 1. Set digest to daily with hour = current+1 2. Wait for scheduled time 3. Verify summary appears in channel |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
