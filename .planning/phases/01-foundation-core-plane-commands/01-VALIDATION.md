---
phase: 1
slug: foundation-core-plane-commands
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 + go.uber.org/mock v0.6.0 |
| **Config file** | None — Go uses standard `_test.go` convention |
| **Quick run command** | `go test ./server/... -count=1 -short` |
| **Full suite command** | `make test` (or `go test ./server/... -v -count=1`) |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./server/... -count=1 -short`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | CONF-01 | unit | `go test ./server/ -run TestConfiguration -count=1` | Wave 0 | pending |
| 01-01-02 | 01 | 1 | CONF-05 | unit | `go test ./server/ -run TestOnActivate -count=1` | Wave 0 | pending |
| 01-02-01 | 02 | 1 | CONF-02 | unit | `go test ./server/ -run TestConnectCommand -count=1` | Wave 0 | pending |
| 01-02-02 | 02 | 1 | CONF-03 | unit | `go test ./server/ -run TestObsidianSetup -count=1` | Wave 0 | pending |
| 01-02-03 | 02 | 1 | CONF-04 | unit | `go test ./server/ -run TestHelpCommand -count=1` | Wave 0 | pending |
| 01-03-01 | 03 | 2 | CREA-01 | unit+integration | `go test ./server/ -run TestCreateTask -count=1` | Wave 0 | pending |
| 01-03-02 | 03 | 2 | CREA-04 | unit | `go test ./server/ -run TestCreateTaskConfirmation -count=1` | Wave 0 | pending |
| 01-03-03 | 03 | 2 | QERY-01 | unit | `go test ./server/ -run TestPlaneMine -count=1` | Wave 0 | pending |
| 01-03-04 | 03 | 2 | QERY-02 | unit | `go test ./server/ -run TestPlaneStatus -count=1` | Wave 0 | pending |

*Status: pending / green / red / flaky*

---

## Wave 0 Requirements

- [ ] `server/plugin_test.go` — covers CONF-01, CONF-05 (OnActivate, config loading)
- [ ] `server/command_test.go` — covers CONF-02, CONF-03, CONF-04, QERY-01, QERY-02 (command routing + handlers)
- [ ] `server/dialog_test.go` — covers CREA-01, CREA-04 (dialog open + submission)
- [ ] `server/plane/client_test.go` — covers Plane API client unit tests with HTTP mocks
- [ ] `server/store/store_test.go` — covers KV store operations
- [ ] `server/testutil/` — shared test helpers (mock Plane server, mock API)
- [ ] Mattermost `plugintest.API` mock setup in test helpers

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dialog multi-select for labels | CREA-01 | Mattermost dialog multi-select not confirmed in docs | Open `/task plane create`, verify label selector works |
| Ephemeral post rendering | CREA-04 | Visual confirmation needed | Run `/task plane create`, verify ephemeral message shows link |
| System Console settings UI | CONF-01 | Requires Mattermost UI interaction | Navigate to System Console > Plugin Settings, verify fields |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
