# Mattermost Command Center

## What This Is

Un plugin de Mattermost que convierte el chat en un centro de comando para gestión de tareas, conectando directamente con Plane (self-hosted). Permite crear tareas desde mensajes, consultar estados, vincular canales a proyectos, recibir notificaciones de cambios y resúmenes periódicos — todo sin salir de Mattermost.

Diseñado para un equipo pequeño (2-10 personas) donde Mattermost es el hub central de comunicación.

## Core Value

Cualquier conversación en Mattermost puede convertirse en una tarea accionable en Plane con un solo clic, sin cambiar de contexto.

## Requirements

### Validated

- ✓ CONF-01: Admin configura Plane desde System Console — v1.0
- ✓ CONF-02: Usuario vincula cuenta Mattermost con Plane via `/task connect` — v1.0
- ✓ CONF-03: Usuario configura endpoint Obsidian via `/task obsidian setup` — v1.0
- ✓ CONF-04: Usuario ve comandos via `/task help` — v1.0
- ✓ CONF-05: Plugin crea bot account al activarse — v1.0
- ✓ CREA-01: Crear tarea via `/task plane create` con diálogo interactivo — v1.0
- ✓ CREA-02: Crear tarea desde menú contextual "..." con texto pre-poblado — v1.0
- ✓ CREA-03: Proyecto pre-seleccionado en canal vinculado — v1.0
- ✓ CREA-04: Confirmación efímera con link a tarea creada — v1.0
- ✓ QERY-01: Ver tareas asignadas via `/task plane mine` — v1.0
- ✓ QERY-02: Ver estado de proyecto via `/task plane status` — v1.0
- ✓ BIND-01: Vincular canal a proyecto via `/task plane link` — v1.0
- ✓ BIND-02: Comandos auto-usan proyecto vinculado — v1.0
- ✓ NOTF-01: Cambios en Plane publicados en canal vinculado via webhooks — v1.0
- ✓ NOTF-02: Digest periódico configurable diario/semanal — v1.0
- ✓ NOTF-03: Link unfurling de URLs de Plane — v1.0

### Active

- [ ] OBSI-01: Crear nota en Obsidian desde slash command
- [ ] OBSI-02: Crear nota en Obsidian desde menú contextual
- [ ] OBSI-03: Notas con frontmatter (fuente, autor, canal, timestamp)
- [ ] OBSI-04: Manejo graceful cuando Obsidian offline
- [ ] ADVQ-01: Buscar tareas en Plane por título/descripción
- [ ] ADVN-01: Sidebar panel con tareas del proyecto vinculado

### Out of Scope

- Sincronización bidireccional Obsidian ↔ Plane — complejidad alta, propósitos diferentes
- Edición de tareas desde Mattermost — chat no es buen UX para edición
- Soporte multi-instancia Plane — innecesario para equipo de 2-10
- Extracción automática de tareas con AI — falsos positivos, creación explícita es suficiente
- Kanban/board view en Mattermost — links a Plane para gestión visual
- App móvil dedicada — se usa a través del cliente Mattermost existente

## Context

Shipped v1.0 con 4,518 LOC Go (producción) + 4,429 LOC Go (tests) + 63 LOC JS (webapp).
Tech stack: Mattermost Plugin SDK (Go), Plane API, gorilla/mux, testify.
Full test suite green: ~60 tests across 3 packages.
Nyquist validation compliant on all 3 phases.

Known tech debt: comment notifications show UUID (Plane API limitation), digest URL format inconsistency.

## Constraints

- **Mattermost Plugin SDK**: Plugins en Go — el core del plugin debe ser Go
- **Plane API**: Usa `/work-items/` endpoints (deprecación `/issues/` en marzo 2026)
- **Obsidian REST API**: Requiere plugin activo y accesible desde red del servidor
- **Rate limits**: Plane API 60 req/min — caching TTL implementado (5-10 min)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Plugin monolítico Go (sin bridge service) | Research reveló que el plugin SDK de Mattermost accede directamente a APIs externas | ✓ Good — simplifica arquitectura |
| Obsidian diferido a v2 (solo setup en v1) | Complejidad de red para REST API local; foco en Plane primero | ✓ Good — v1 shipped rápido |
| `/work-items/` endpoints exclusivamente | Deprecación `/issues/` en Plane marzo 2026 | ✓ Good — futuro-proof |
| Unicode emojis en vez de shortcodes | Compatibilidad con SlackAttachment títulos | ✓ Good — rendering consistente |
| Context menu via Redux dispatch (sin trigger_id) | Mattermost no provee trigger_id desde post actions | ✓ Good — funciona, bypass documentado |
| Reverse index project→channels | Eficiente routing de webhooks a canales vinculados | ✓ Good — O(1) lookup |
| cluster.Schedule para digest | HA-safe en clusters multi-nodo | ✓ Good — single execution garantizada |
| HMAC permisivo sin secret | Facilita setup inicial sin requerir webhook secret | ⚠️ Revisit — considerar forzar secret en v2 |

---
*Last updated: 2026-03-19 after v1.0 milestone*
