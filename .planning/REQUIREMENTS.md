# Requirements: Mattermost Command Center

**Defined:** 2026-03-17
**Core Value:** Cualquier conversación en Mattermost puede convertirse en una tarea accionable en Plane u Obsidian con un solo clic, sin cambiar de contexto.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Setup & Configuration

- [ ] **CONF-01**: Admin puede configurar URL, API key y workspace de Plane desde System Console de Mattermost
- [ ] **CONF-02**: Usuario puede vincular su cuenta Mattermost con su usuario Plane via `/task connect`
- [ ] **CONF-03**: Usuario puede configurar su endpoint de Obsidian Local REST API via `/task obsidian setup` (host, puerto, API key)
- [ ] **CONF-04**: Usuario puede ver comandos disponibles y su uso via `/task help`
- [ ] **CONF-05**: Plugin crea bot account automáticamente al activarse para publicar mensajes

### Task Creation (Plane)

- [ ] **CREA-01**: Usuario puede crear tarea en Plane via `/task plane create` con diálogo interactivo (título, descripción, proyecto, prioridad, asignado)
- [ ] **CREA-02**: Usuario puede crear tarea en Plane desde menú contextual "..." de cualquier mensaje, con texto del mensaje pre-poblado como descripción y permalink al mensaje original
- [ ] **CREA-03**: Al crear tarea en canal vinculado, el proyecto Plane se pre-selecciona automáticamente
- [ ] **CREA-04**: Usuario recibe confirmación efímera con link a la tarea creada en Plane

### Queries & Consultation

- [ ] **QERY-01**: Usuario puede ver sus tareas asignadas en Plane via `/task plane mine` (respuesta efímera)
- [ ] **QERY-02**: Usuario puede ver resumen de estado de un proyecto Plane (open/in-progress/done) via `/task plane status`

### Channel-Project Binding

- [ ] **BIND-01**: Usuario puede vincular un canal de Mattermost a un proyecto de Plane via `/task plane link`
- [ ] **BIND-02**: Comandos ejecutados en canal vinculado usan automáticamente el proyecto asociado sin necesidad de especificarlo

### Notifications

- [ ] **NOTF-01**: Cambios en tareas de Plane (estado, asignación, comentarios) se publican automáticamente en el canal vinculado via webhooks
- [ ] **NOTF-02**: Bot publica resumen periódico (configurable: diario/semanal) del estado del proyecto en el canal vinculado
- [ ] **NOTF-03**: Al pegar URL de tarea de Plane en chat, se muestra preview inline con título, estado y asignado (link unfurling)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Obsidian Integration

- **OBSI-01**: Usuario puede crear nota en Obsidian desde slash command `/task obsidian create`
- **OBSI-02**: Usuario puede crear nota en Obsidian desde menú contextual de mensaje
- **OBSI-03**: Notas creadas incluyen frontmatter (fuente, autor, canal, timestamp, link a mensaje)
- **OBSI-04**: Manejo graceful cuando Obsidian está offline o no accesible

### Advanced Queries

- **ADVQ-01**: Usuario puede buscar tareas en Plane por título/descripción via `/task plane search`
- **ADVQ-02**: Comandos contextuales: `/task plane mine` sin proyecto muestra las del proyecto vinculado

### Advanced Notifications

- **ADVN-01**: Right-hand sidebar panel con vista de tareas del proyecto vinculado

## Out of Scope

| Feature | Reason |
|---------|--------|
| Sincronización bidireccional Obsidian ↔ Plane | Complejidad alta, herramientas con propósitos diferentes |
| Edición de tareas desde Mattermost | v1 se centra en creación y consulta; UX de edición en chat es pobre |
| Soporte multi-instancia Plane | Complejidad enterprise innecesaria para equipo de 2-10 |
| Extracción automática de tareas con AI | Genera falsos positivos y desconfianza; creación explícita es suficiente |
| Browsing de vault Obsidian desde MM | Vaults son personales; exponer estructura es riesgo de privacidad |
| Kanban/board view en Mattermost | Chat no es UI de project management; links a Plane para gestión visual |
| App móvil dedicada | Se usa a través del cliente Mattermost existente |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | — | Pending |
| CONF-02 | — | Pending |
| CONF-03 | — | Pending |
| CONF-04 | — | Pending |
| CONF-05 | — | Pending |
| CREA-01 | — | Pending |
| CREA-02 | — | Pending |
| CREA-03 | — | Pending |
| CREA-04 | — | Pending |
| QERY-01 | — | Pending |
| QERY-02 | — | Pending |
| BIND-01 | — | Pending |
| BIND-02 | — | Pending |
| NOTF-01 | — | Pending |
| NOTF-02 | — | Pending |
| NOTF-03 | — | Pending |

**Coverage:**
- v1 requirements: 16 total
- Mapped to phases: 0
- Unmapped: 16 ⚠️

---
*Requirements defined: 2026-03-17*
*Last updated: 2026-03-17 after initial definition*
