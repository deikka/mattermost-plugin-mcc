---
phase: 01-foundation-core-plane-commands
plan: 01
subsystem: plugin-scaffold
tags: [go, mattermost-plugin, pluginapi, gorilla-mux, slash-commands, autocomplete]

requires:
  - phase: 01-00
    provides: "Test infrastructure: testutil package, mock Plane server, test stubs"
provides:
  - "Compilable Mattermost plugin with System Console settings for Plane (URL, API Key, Workspace)"
  - "Bot account (task-bot) created on plugin activation via pluginapi.BotService"
  - "Full autocomplete tree for /task command with plane, connect, obsidian, help subcommands"
  - "Command routing via handler map with alias support (p/c, p/m, p/s)"
  - "Levenshtein-based command suggestion for unknown subcommands"
  - "/task help returns formatted ephemeral command list"
  - "Stub handlers for all unimplemented commands (plane/create, plane/mine, plane/status, connect, obsidian/setup)"
  - "Non-blocking Plane connection health check with admin DM notification"
affects: [01-02, 01-03]

tech-stack:
  added: [mattermost-server-public-v0.1.21, gorilla-mux-v1.8.1, pkg-errors-v0.9.1, pluginapi]
  patterns: [handler-map-routing, progressive-key-matching, thread-safe-config-rwmutex, ephemeral-response-helpers, non-blocking-health-check]

key-files:
  created:
    - plugin.json
    - go.mod
    - Makefile
    - assets/icon.svg
    - server/manifest.go
    - server/plugin.go
    - server/configuration.go
    - server/bot.go
    - server/command.go
    - server/command_router.go
    - server/command_handlers.go
  modified:
    - server/plugin_test.go
    - server/command_test.go

key-decisions:
  - "Used mattermost/server/public/pluginapi instead of deprecated mattermost-plugin-api module (correct monorepo import path)"
  - "Progressive key matching in command router: tries longest key first, shortens until match (supports args after commands)"
  - "Non-blocking Plane health check via goroutine in OnActivate (plugin activates even if Plane unreachable)"
  - "Admin notification of Plane connection failure via DM using p.API.GetUsers with system_admin role filter"

patterns-established:
  - "Handler map pattern: map[string]CommandHandlerFunc with alias resolution"
  - "Ephemeral response pattern: p.respondEphemeral(args, message) returns empty CommandResponse"
  - "Thread-safe config: RWMutex guard, Clone on change, OnConfigurationChange hook"
  - "Test pattern: setupTestPlugin for config tests, setupActivatedPlugin for full lifecycle tests"
  - "Test pattern: setupCommandTestPlugin for command routing tests with permissive mock"

requirements-completed: [CONF-01, CONF-04, CONF-05]

duration: 15min
completed: 2026-03-17
---

# Phase 1 Plan 01: Plugin Scaffold Summary

**Mattermost Go plugin with System Console settings, bot account, full autocomplete command tree, handler-map routing with alias support, and /task help**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-17T06:44:45Z
- **Completed:** 2026-03-17T06:59:45Z
- **Tasks:** 3
- **Files modified:** 18

## Accomplishments
- Plugin compiles and produces valid Go binaries for linux-amd64, darwin-amd64, darwin-arm64
- Admin sees PlaneURL, PlaneAPIKey (secret), PlaneWorkspace in System Console settings
- Bot account "task-bot" created via pluginapi on activation
- Full autocomplete tree: /task plane (create/mine/status), /task p (c/m/s), /task connect, /task obsidian setup, /task help
- Command router with handler map dispatches all commands and aliases correctly
- Unknown commands get Levenshtein-based "Did you mean...?" suggestions
- /task help returns formatted ephemeral list of all commands with aliases
- All 6 handler functions exist (help implemented, 5 stubs for future plans)
- 14 passing tests covering configuration, activation, command routing, aliases, suggestions

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold plugin from starter template and configure manifest** - `7b2d1a5` (feat)
2. **Task 2: Implement Plugin struct, OnActivate, configuration, and bot account** - `d66c7de` + `20831be` (test+feat)
3. **Task 3: Implement command registration with autocomplete tree, router with aliases, and /task help handler** - `6a8dd13` (fix)

## Files Created/Modified
- `plugin.json` - Plugin manifest with id, settings_schema (3 Plane settings), server executables
- `go.mod` / `go.sum` - Go 1.25 module with mattermost/server/public, gorilla/mux, pkg/errors
- `Makefile` - build/test/bundle/deploy/clean targets
- `assets/icon.svg` - Checkmark icon for System Console
- `server/manifest.go` - Auto-generated manifest ID constant
- `server/plugin.go` - Plugin struct, OnActivate (config + bot + commands + router + health check), ServeHTTP
- `server/configuration.go` - Thread-safe config with RWMutex, Clone, getConfiguration, setConfiguration, OnConfigurationChange
- `server/bot.go` - ensureBot, sendEphemeral, respondEphemeral helpers
- `server/command.go` - registerCommands with full AutocompleteData tree (plane, p, connect, obsidian, help)
- `server/command_router.go` - ExecuteCommand with progressive key matching, alias resolution, suggestCommand with Levenshtein
- `server/command_handlers.go` - handleHelp (implemented) + 5 stub handlers
- `server/plugin_test.go` - TestConfiguration, TestConfigurationDefaults, TestOnActivate, TestOnActivateBotCreation
- `server/command_test.go` - TestHelpCommand, TestCommandRouting, TestCommandAliases, TestUnknownCommandSuggestion, TestLevenshtein + stubs

## Decisions Made
- **Used mattermost/server/public/pluginapi** instead of deprecated mattermost-plugin-api module: the separate mattermost-plugin-api package (latest v0.1.4) still uses mattermost-server/v6 module paths which conflict with the monorepo. The pluginapi package is available within the monorepo at mattermost/server/public/pluginapi.
- **Progressive key matching in router**: when user types `/task plane create My Title`, the router tries `plane/create/My/Title` (miss), then shorter keys until `plane/create` matches, passing `["My", "Title"]` as subArgs. This is simpler and more robust than fixed-depth parsing.
- **API.GetUsers for admin notification**: instead of pluginapi.Client.User.ListAdmins(), used raw p.API.GetUsers with system_admin role filter since the pluginapi method may not be available in all versions.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Used pluginapi from monorepo instead of separate mattermost-plugin-api module**
- **Found during:** Task 1 (go.mod setup)
- **Issue:** Plan specified `github.com/mattermost/mattermost-plugin-api v0.3.0` which doesn't exist. Latest version (v0.1.4) uses deprecated mattermost-server/v6 module paths, incompatible with monorepo.
- **Fix:** Used `github.com/mattermost/mattermost/server/public/pluginapi` from the monorepo (same API, correct module path).
- **Files modified:** go.mod, server/plugin.go
- **Verification:** `go build ./server/...` compiles, pluginapi.Client works in tests
- **Committed in:** 7b2d1a5

**2. [Rule 1 - Bug] Fixed command routing for commands with arguments**
- **Found during:** Task 3 (command router implementation)
- **Issue:** Original `buildCommandKey` joined ALL parts into a single key, so `/task plane create My Title` produced `plane/create/My/Title` which never matched.
- **Fix:** Implemented progressive key matching: try full key, then shorter keys until match found, passing remaining parts as subArgs.
- **Files modified:** server/command_router.go
- **Verification:** TestCommandRoutingWithArgs and TestCommandAliasesWithArgs pass
- **Committed in:** 6a8dd13

**3. [Rule 3 - Blocking] Created Wave 0 test infrastructure (01-00 dependency)**
- **Found during:** Task 2 (test creation)
- **Issue:** Plan 01-01 depends on 01-00 (Wave 0 test infrastructure) which had not been executed. Test stubs and helpers were needed.
- **Fix:** Created test infrastructure inline: setupTestPlugin, setupActivatedPlugin, setupCommandTestPlugin helpers, and test stubs for future plans.
- **Files modified:** server/plugin_test.go, server/command_test.go
- **Verification:** All tests compile and pass
- **Committed in:** 20831be, 6a8dd13

---

**Total deviations:** 3 auto-fixed (1 bug fix, 2 blocking)
**Impact on plan:** All auto-fixes necessary for correctness and compilation. No scope creep.

## Issues Encountered
- `EnsureBotUser` vs `EnsureBot` mock naming: the pluginapi.BotService.EnsureBot internally calls `api.EnsureBotUser()`, not `api.EnsureBot()`. Required reading pluginapi source to identify the correct mock method name.
- sirupsen/logrus missing from go.sum: the pluginapi package imports logrus which wasn't in go.sum after initial tidy. Resolved with `go get github.com/sirupsen/logrus`.

## User Setup Required

None - no external service configuration required at this stage. Plugin settings will be configured by admin in System Console after deployment.

## Next Phase Readiness
- Plugin scaffold complete, ready for Plan 01-02 (Plane API client, KV store, /task connect, /task obsidian setup)
- All stub handlers ready to be replaced with real implementations
- Handler map pattern established for easy addition of new commands
- Test infrastructure ready for Plans 01-02 and 01-03 to flesh out remaining test stubs

## Self-Check: PASSED

- All 13 key files exist on disk
- All 4 task commits verified in git history
- All required tests pass (TestConfiguration, TestOnActivate, TestHelpCommand, TestCommandRouting, TestCommandAliases, TestUnknownCommandSuggestion)

---
*Phase: 01-foundation-core-plane-commands*
*Completed: 2026-03-17*
