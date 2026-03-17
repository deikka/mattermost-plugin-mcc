# Roadmap: Mattermost Command Center

## Overview

Este roadmap lleva el plugin desde cero hasta un centro de comando funcional en 3 fases. La Fase 1 entrega el valor central: crear y consultar tareas de Plane desde Mattermost via slash commands. La Fase 2 convierte el plugin en una experiencia nativa con menus contextuales, vinculacion canal-proyecto y link unfurling. La Fase 3 cierra el ciclo de feedback con notificaciones automaticas desde Plane y resumenes periodicos.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation + Core Plane Commands** - Plugin scaffold, configuracion, creacion y consulta de tareas en Plane via slash commands
- [ ] **Phase 2: Channel Intelligence + Context Menu** - Vinculacion canal-proyecto, menu contextual en mensajes y link unfurling
- [ ] **Phase 3: Notifications + Automation** - Webhooks de Plane hacia canales vinculados y resumenes periodicos

## Phase Details

### Phase 1: Foundation + Core Plane Commands
**Goal**: Usuarios pueden crear tareas en Plane y consultar sus tareas pendientes directamente desde Mattermost via slash commands
**Depends on**: Nothing (first phase)
**Requirements**: CONF-01, CONF-02, CONF-03, CONF-04, CONF-05, CREA-01, CREA-04, QERY-01, QERY-02
**Success Criteria** (what must be TRUE):
  1. Admin puede configurar la conexion a Plane desde System Console y el plugin valida la conexion al activarse
  2. Usuario puede vincular su cuenta Mattermost con Plane via `/task connect` y configurar su endpoint de Obsidian via `/task obsidian setup`
  3. Usuario puede crear una tarea en Plane via `/task plane create`, completar el dialogo interactivo, y recibir confirmacion efimera con link a la tarea creada
  4. Usuario puede ver sus tareas asignadas en Plane via `/task plane mine` y el estado de un proyecto via `/task plane status`
  5. Usuario puede ver la lista de comandos disponibles via `/task help`
**Plans:** 4 plans

Plans:
- [x] 01-00-PLAN.md — Wave 0: Test infrastructure scaffolding (test helpers, mock Plane server, test stubs)
- [x] 01-01-PLAN.md — Plugin scaffold, System Console config, bot account, command registration with autocomplete, /task help
- [x] 01-02-PLAN.md — Plane API client, KV store, cache, /task connect, /task obsidian setup, HTTP API endpoints
- [x] 01-03-PLAN.md — /task plane create (dialog + inline), /task plane mine, /task plane status, ephemeral confirmations

### Phase 2: Channel Intelligence + Context Menu
**Goal**: Canales de Mattermost funcionan como espacios de proyecto donde crear tareas es un clic desde cualquier mensaje, con contexto automatico del proyecto vinculado
**Depends on**: Phase 1
**Requirements**: CREA-02, CREA-03, BIND-01, BIND-02, NOTF-03
**Success Criteria** (what must be TRUE):
  1. Usuario puede vincular un canal a un proyecto de Plane via `/task plane link` y los comandos en ese canal usan automaticamente el proyecto asociado
  2. Usuario puede crear tarea desde el menu contextual "..." de cualquier mensaje, con el texto pre-poblado como descripcion y permalink al mensaje original
  3. Al crear tarea en canal vinculado (via slash command o menu contextual), el proyecto Plane se pre-selecciona automaticamente
  4. Al pegar una URL de tarea de Plane en chat, se muestra preview inline con titulo, estado y asignado
**Plans:** 4 plans

Plans:
- [ ] 02-00-PLAN.md — Wave 0: Test stubs for Phase 2 (store binding, command handlers, link unfurling, GetWorkItem)
- [ ] 02-01-PLAN.md — Channel-project binding (store CRUD, link/unlink commands, binding-aware handlers, dialog pre-selection)
- [ ] 02-02-PLAN.md — Link unfurling (GetWorkItem API, URL extraction, MessageHasBeenPosted hook, SlackAttachment cards)
- [ ] 02-03-PLAN.md — Context menu "Create Task from Message" (webapp component, server handler, build setup, end-to-end verification)

### Phase 3: Notifications + Automation
**Goal**: Cambios en Plane se reflejan automaticamente en Mattermost, cerrando el ciclo de feedback sin que el equipo tenga que revisar Plane manualmente
**Depends on**: Phase 2 (requiere canal-proyecto mapping para enrutar notificaciones)
**Requirements**: NOTF-01, NOTF-02
**Success Criteria** (what must be TRUE):
  1. Cambios en tareas de Plane (estado, asignacion, comentarios) se publican automaticamente en el canal vinculado correspondiente
  2. Bot publica resumen periodico configurable (diario/semanal) del estado del proyecto en el canal vinculado
**Plans**: TBD

Plans:
- [ ] 03-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 2 -> 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation + Core Plane Commands | 4/4 | Complete | 2026-03-17 |
| 2. Channel Intelligence + Context Menu | 0/4 | In progress | - |
| 3. Notifications + Automation | 0/1 | Not started | - |
