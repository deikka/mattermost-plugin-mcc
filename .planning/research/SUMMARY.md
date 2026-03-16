# Research Summary: Mattermost Command Center

**Domain:** Chat-to-task integration plugin (Mattermost + Plane + Obsidian)
**Researched:** 2026-03-16
**Overall confidence:** HIGH

## Executive Summary

The Mattermost Command Center is a Go plugin that turns Mattermost chat conversations into actionable tasks in Plane and personal notes in Obsidian. The technology ecosystem is mature and well-documented: Mattermost's plugin SDK provides everything needed (slash commands, interactive dialogs, post action menus, HTTP endpoints, KV storage, bot accounts) without requiring an external bridge service. This is the single most important architectural finding -- the originally proposed bridge service is unnecessary and would add complexity.

The Plane REST API v1 is well-documented with clear endpoints for work item CRUD, project listing, and webhook event delivery. The critical constraint is a 60 request/minute rate limit that demands aggressive caching. Additionally, the `/issues/` endpoints are being deprecated by March 31, 2026 -- the plugin must use `/work-items/` exclusively from day one.

The Obsidian Local REST API is the most architecturally challenging integration. It runs per-user on individual machines, requires Obsidian to be open, uses self-signed TLS certificates, and may have Docker networking issues. The design must treat it as "best-effort, user-present" -- never assuming the endpoint is available. This naturally pushes Obsidian integration to a later phase.

The recommended stack is pure Go for the server plugin (standard library + Mattermost SDK + gorilla/mux), minimal TypeScript/React for the webapp component (just registering post menu actions), and Mattermost's built-in KV store for all persistence. No external database, no separate services, no third-party HTTP libraries.

## Key Findings

**Stack:** Go 1.22+ plugin using `github.com/mattermost/mattermost/server/public` (plugin + model) and `github.com/mattermost/mattermost-plugin-api` (higher-level wrapper). No bridge service. Custom HTTP clients for Plane and Obsidian APIs (no SDKs exist in Go for either).

**Architecture:** Monolithic plugin with internal service layer. Plugin struct holds Plane client, Obsidian router, KV store layer, and bot account. HTTP routing via gorilla/mux for webhooks and action callbacks. All persistent state in Mattermost KV store with structured key prefixes and reverse indexes.

**Critical pitfall:** Plane API rate limit (60 req/min) combined with endpoint deprecation (`/issues/` -> `/work-items/` by March 31, 2026). Must use new endpoints and implement caching from the first API call.

## Implications for Roadmap

Based on research, suggested phase structure:

1. **Foundation + Core Plane Integration** - Start with the highest-value, lowest-risk integration
   - Addresses: Plugin scaffold, Plane API client, slash commands (`/task plane create`, `/task plane mine`), KV store schema, bot account, System Console config
   - Avoids: Obsidian per-user complexity (Pitfall #2), webhook network issues (Pitfall #5)
   - Risk: Plane API version mismatch with self-hosted instance (Pitfall #10). Mitigate with startup health check.

2. **Channel Intelligence + Post Actions** - Add the smart defaults and desktop UX enhancements
   - Addresses: Channel-project binding, auto-routing, post context menu "Create Task", search, project status
   - Avoids: No mobile assumptions (Pitfall #6) -- slash commands remain primary UX
   - Risk: KV reverse index maintenance (Pitfall #9). Design indexes before coding.

3. **Obsidian Integration** - Add the personal knowledge capture flow
   - Addresses: Per-user Obsidian config, note creation from messages, `/task obsidian create`
   - Avoids: Vault browsing (anti-feature), bidirectional sync (anti-feature)
   - Risk: Network reachability (Pitfall #2, #8). Requires spike to validate Mattermost-to-Obsidian connectivity.
   - **Needs deeper research:** Docker networking behavior with Obsidian Local REST API, recommended network topology for the team.

4. **Notifications + Polish** - Close the feedback loop from Plane back to Mattermost
   - Addresses: Plane webhook receiver, HMAC verification, channel notifications, link unfurling
   - Avoids: Periodic summaries in this phase (high complexity, rate limit risk)
   - Risk: Plane webhook deliverability between self-hosted instances (Pitfall #5). Validate network path before building.

**Phase ordering rationale:**
- Phase 1 delivers the core value proposition (chat -> Plane task) with minimal moving parts
- Phase 2 builds on Phase 1's infrastructure to add UX polish and channel context
- Phase 3 is isolated (Obsidian) and can be built independently of Phase 2 if needed
- Phase 4 requires Phase 2's channel-project mappings for webhook routing
- Each phase is usable on its own -- no phase depends on all previous phases being perfect

**Research flags for phases:**
- Phase 1: Standard patterns, well-documented. Unlikely to need additional research.
- Phase 2: KV store patterns may need a spike to validate reverse index performance at scale.
- Phase 3: **NEEDS DEEPER RESEARCH** -- Obsidian network topology and Docker connectivity should be spiked before committing to architecture.
- Phase 4: Plane webhook deliverability in specific self-hosted network topology should be validated early.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All technologies verified against official docs. Go module paths confirmed. Plugin SDK well-documented. |
| Features | HIGH | Feature landscape derived from analyzing official Mattermost plugins (Jira, GitHub, ServiceNow). API capabilities verified. |
| Architecture | HIGH | Pattern follows official Mattermost plugin architecture. No bridge = simpler than originally proposed. |
| Plane API | HIGH | Endpoints, auth, rate limits, webhooks all verified from official docs at developers.plane.so |
| Obsidian API | HIGH for endpoints, MEDIUM for network topology | API endpoints well-documented. Docker networking behavior has known issues (GitHub discussion). |
| Pitfalls | HIGH | Critical pitfalls verified from official docs (rate limits, deprecation, HA, mobile limitations). |

## Gaps to Address

- **Plane self-hosted version verification:** Need to check what version of Plane the team's instance runs and whether it supports `/work-items/` endpoints. This is a Phase 1 prerequisite.
- **Obsidian network topology:** Need to validate that the Mattermost server can reach each team member's Obsidian Local REST API. If Mattermost is in Docker, test connectivity early. This is a Phase 3 prerequisite.
- **Plane user identity mapping:** Need to determine how to map Mattermost user IDs to Plane user IDs for assignee filtering. Options: email match, manual mapping via `/task connect`, or admin-configured mapping.
- **Mattermost server version:** Need to confirm the team's Mattermost server version supports the plugin APIs used (recommend minimum v10.0.0 for current module paths).
- **Rate limit budget allocation:** With 60 req/min, need to decide how much budget goes to interactive commands vs. cached background operations vs. webhook callbacks. This should be designed in Phase 1.
