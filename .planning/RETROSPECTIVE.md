# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Mattermost Command Center MVP

**Shipped:** 2026-03-19
**Phases:** 3 | **Plans:** 11 | **Commits:** 72

### What Was Built
- Plugin Mattermost completo con 9 slash commands y menú contextual
- Integración bidireccional con Plane: crear, consultar, vincular, notificar
- Webapp component para context menu "Crear Tarea en Plane"
- Webhook receiver con HMAC, dedup y 5 tipos de notificación
- Digest scheduler cluster-safe con resúmenes periódicos configurables
- Suite de tests comprehensiva (~60 tests, 0 skips, 0 fails)

### What Worked
- **Wave 0 pattern**: escribir tests RED antes de implementación aceleró cada fase
- **Monolithic plugin**: investigación pre-roadmap descartó bridge service, simplificó todo
- **Progressive key matching**: router de comandos elegante que soporta aliases y sugerencias Levenshtein
- **Cached API calls**: TTL cache redujo llamadas a Plane API respetando rate limits (60 req/min)
- **3-day delivery**: de cero a plugin funcional con 16 requirements en 3 días

### What Was Inefficient
- **ROADMAP.md inconsistencias**: checkboxes de Phase 2 no se actualizaron, progreso mostraba "In Progress" cuando ya estaba completo — overhead manual que GSD debería automatizar
- **SUMMARY frontmatter**: ningún SUMMARY tiene `requirements-completed` — el campo no se usa en la práctica, considerar eliminarlo del workflow
- **Context menu trigger_id bypass**: diseñamos `openCreateTaskDialogWithContext` para reuso pero el context menu necesitó un approach totalmente diferente (Redux dispatch) — la investigación previa no capturó esta limitación de Mattermost
- **Emoji shortcodes vs Unicode**: tests escritos con shortcodes pero implementación usa Unicode — inconsistencia detectada tarde en Nyquist validation

### Patterns Established
- **Plugin Go monolítico** sin bridge service para integraciones con APIs externas
- **Reverse index en KV store** para routing eficiente (project → channels)
- **Self-notification suppression** via KV marker con TTL
- **HMAC webhook verification** con mode permisivo cuando secret está vacío
- **Cluster-safe scheduling** via `pluginapi/cluster.Schedule`
- **Binding-aware commands** que verifican canal vinculado antes de resolver proyecto

### Key Lessons
1. **Investigar limitaciones de UI antes de diseñar funciones**: el bypass de trigger_id costó rediseño del context menu
2. **Unicode emojis > shortcodes** en SlackAttachment: los shortcodes no se renderizan en todos los contextos
3. **Wave 0 tests son imprescindibles**: cada test RED descubierto durante la implementación fue un bug prevenido
4. **Plane API evoluciona rápido**: verificar endpoints antes de cada milestone (browse URLs, work-items vs issues)

### Cost Observations
- Model mix: ~60% sonnet (execution), ~30% opus (planning/review), ~10% haiku (quick checks)
- Sessions: ~8 sessions across 3 days
- Notable: Wave 0 pattern front-loads test writing but pays back in zero-rework implementation

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Commits | Phases | Key Change |
|-----------|---------|--------|------------|
| v1.0 | 72 | 3 | Wave 0 TDD, Nyquist validation, milestone audit |

### Cumulative Quality

| Milestone | Tests | LOC (prod) | LOC (test) |
|-----------|-------|------------|------------|
| v1.0 | ~60 | 4,518 | 4,429 |

### Top Lessons (Verified Across Milestones)

1. Wave 0 test infrastructure before implementation prevents rework
2. Research API limitations of target platform before designing interaction patterns
