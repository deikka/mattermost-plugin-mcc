# Requirements: Mattermost Command Center

**Defined:** 2026-03-17
**Core Value:** Cualquier conversacion en Mattermost puede convertirse en una tarea accionable en Plane u Obsidian con un solo clic, sin cambiar de contexto.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Setup & Configuration

- [x] **CONF-01**: Admin puede configurar URL, API key y workspace de Plane desde System Console de Mattermost
- [x] **CONF-02**: Usuario puede vincular su cuenta Mattermost con su usuario Plane via `/task connect`
- [x] **CONF-03**: Usuario puede configurar su endpoint de Obsidian Local REST API via `/task obsidian setup` (host, puerto, API key)
- [x] **CONF-04**: Usuario puede ver comandos disponibles y su uso via `/task help`
- [x] **CONF-05**: Plugin crea bot account automaticamente al activarse para publicar mensajes

### Task Creation (Plane)

- [x] **CREA-01**: Usuario puede crear tarea en Plane via `/task plane create` con dialogo interactivo (titulo, descripcion, proyecto, prioridad, asignado)
- [x] **CREA-02**: Usuario puede crear tarea en Plane desde menu contextual "..." de cualquier mensaje, con texto del mensaje pre-poblado como descripcion y permalink al mensaje original
- [x] **CREA-03**: Al crear tarea en canal vinculado, el proyecto Plane se pre-selecciona automaticamente
- [x] **CREA-04**: Usuario recibe confirmacion efimera con link a la tarea creada en Plane

### Queries & Consultation

- [x] **QERY-01**: Usuario puede ver sus tareas asignadas en Plane via `/task plane mine` (respuesta efimera)
- [x] **QERY-02**: Usuario puede ver resumen de estado de un proyecto Plane (open/in-progress/done) via `/task plane status`

### Channel-Project Binding

- [x] **BIND-01**: Usuario puede vincular un canal de Mattermost a un proyecto de Plane via `/task plane link`
- [x] **BIND-02**: Comandos ejecutados en canal vinculado usan automaticamente el proyecto asociado sin necesidad de especificarlo

### Notifications

- [ ] **NOTF-01**: Cambios en tareas de Plane (estado, asignacion, comentarios) se publican automaticamente en el canal vinculado via webhooks
- [ ] **NOTF-02**: Bot publica resumen periodico (configurable: diario/semanal) del estado del proyecto en el canal vinculado
- [x] **NOTF-03**: Al pegar URL de tarea de Plane en chat, se muestra preview inline con titulo, estado y asignado (link unfurling)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Obsidian Integration

- **OBSI-01**: Usuario puede crear nota en Obsidian desde slash command `/task obsidian create`
- **OBSI-02**: Usuario puede crear nota en Obsidian desde menu contextual de mensaje
- **OBSI-03**: Notas creadas incluyen frontmatter (fuente, autor, canal, timestamp, link a mensaje)
- **OBSI-04**: Manejo graceful cuando Obsidian esta offline o no accesible

### Advanced Queries

- **ADVQ-01**: Usuario puede buscar tareas en Plane por titulo/descripcion via `/task plane search`
- **ADVQ-02**: Comandos contextuales: `/task plane mine` sin proyecto muestra las del proyecto vinculado

### Advanced Notifications

- **ADVN-01**: Right-hand sidebar panel con vista de tareas del proyecto vinculado

## Out of Scope

| Feature | Reason |
|---------|--------|
| Sincronizacion bidireccional Obsidian <-> Plane | Complejidad alta, herramientas con propositos diferentes |
| Edicion de tareas desde Mattermost | v1 se centra en creacion y consulta; UX de edicion en chat es pobre |
| Soporte multi-instancia Plane | Complejidad enterprise innecesaria para equipo de 2-10 |
| Extraccion automatica de tareas con AI | Genera falsos positivos y desconfianza; creacion explicita es suficiente |
| Browsing de vault Obsidian desde MM | Vaults son personales; exponer estructura es riesgo de privacidad |
| Kanban/board view en Mattermost | Chat no es UI de project management; links a Plane para gestion visual |
| App movil dedicada | Se usa a traves del cliente Mattermost existente |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| CONF-01 | Phase 1 | Complete |
| CONF-02 | Phase 1 | Complete |
| CONF-03 | Phase 1 | Complete |
| CONF-04 | Phase 1 | Complete |
| CONF-05 | Phase 1 | Complete |
| CREA-01 | Phase 1 | Complete |
| CREA-02 | Phase 2 | Complete |
| CREA-03 | Phase 2 | Complete |
| CREA-04 | Phase 1 | Complete |
| QERY-01 | Phase 1 | Complete |
| QERY-02 | Phase 1 | Complete |
| BIND-01 | Phase 2 | Complete |
| BIND-02 | Phase 2 | Complete |
| NOTF-01 | Phase 3 | Pending |
| NOTF-02 | Phase 3 | Pending |
| NOTF-03 | Phase 2 | Complete |

**Coverage:**
- v1 requirements: 16 total
- Mapped to phases: 16
- Unmapped: 0

---
*Requirements defined: 2026-03-17*
*Last updated: 2026-03-17 after roadmap creation*
