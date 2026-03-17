# Phase 1: Foundation + Core Plane Commands - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Plugin scaffold de Mattermost en Go + configuración de Plane y Obsidian + slash commands para crear y consultar tareas en Plane. Incluye bot account, System Console config, diálogos interactivos y respuestas efímeras. NO incluye menú contextual de mensajes (Phase 2), channel-project binding (Phase 2), ni webhooks/notificaciones (Phase 3).

</domain>

<decisions>
## Implementation Decisions

### Estructura de comandos
- Comando raíz `/task` con subcomandos: `/task plane create`, `/task plane mine`, `/task plane status`, `/task connect`, `/task obsidian setup`, `/task help`
- Alias cortos disponibles: `/task p c` = `/task plane create`, `/task p m` = `/task plane mine`
- Autocompletado completo: subcomandos + hints de argumentos (ej: `/task plane create [título]`)
- Si comando incorrecto: sugerir comando correcto ("Did you mean /task plane create?")

### Diálogo de creación de tarea
- Campos: Título (obligatorio), Descripción (textarea), Proyecto (selector dinámico), Prioridad (selector), Asignado (selector), Labels (multi-select dinámico con labels del proyecto)
- Smart defaults: Proyecto = vinculado al canal (si existe binding en Phase 2, vacío si no), Asignado = quien crea la tarea, Prioridad = ninguna/media
- Modo rápido inline: `/task p c "Título de la tarea"` crea directamente con smart defaults sin abrir diálogo
- Diálogo completo cuando se usa `/task plane create` sin argumentos

### Formato de respuestas
- `/task plane mine`: Lista compacta, 1 línea por tarea con emoji de estado + título + proyecto + prioridad. Incluir estado. Máximo 10 tareas. Link "Abrir en Plane" al final.
- `/task plane status`: Resumen con contadores por estado (Open/In Progress/Done) + barra de progreso + última actividad + link a Plane
- Confirmación de creación: Mensaje efímero simple en 1 línea: "✅ Tarea creada: [Título] — [Proyecto] [link]"
- Todas las respuestas de consultas personales son efímeras (solo visible para quien ejecuta)

### Mapeo de usuarios
- `/task connect`: Claude decide mecanismo más práctico (email match, API key personal, o combinación)
- Si usuario no conectado intenta usar `/task plane`: bloquear y guiar — "Primero conecta tu cuenta: /task connect"
- Configuración de Obsidian (`/task obsidian setup`): Claude decide formato más simple (host+port+key o URL+key)

### Claude's Discretion
- Mecanismo exacto de `/task connect` (auto email match vs API key personal vs híbrido)
- Formato de input para `/task obsidian setup` (host+port+key vs URL completa+key)
- Health check al activar plugin (validar conexión Plane)
- Esquema de KV store (prefijos de keys, índices)
- Rate limiting y caching strategy para Plane API (60 req/min)
- Manejo de errores de red/API con mensajes accionables

</decisions>

<specifics>
## Specific Ideas

- La lista de tareas debe incluir siempre el estado (no solo el emoji de color, también texto legible)
- El modo rápido inline (`/task p c "título"`) es clave — para el equipo la velocidad importa más que completar todos los campos
- Los alias cortos son importantes para el día a día: `/task p c`, `/task p m` deben funcionar

</specifics>

<code_context>
## Existing Code Insights

### Reusable Assets
- Proyecto greenfield — no hay código existente

### Established Patterns
- Mattermost Plugin SDK: `plugin.MattermostPlugin` base struct, `OnActivate` hook, `ExecuteCommand`, `pluginapi.Client` wrapper
- Interactive Dialogs: `model.OpenDialogRequest` con campos text/textarea/select/bool
- KV Store: `pluginapi.KVService` con `Get/Set/Delete` y `CompareAndSet` para concurrencia
- Post actions: `model.PostAction` con botones interactivos en mensajes

### Integration Points
- System Console: `settings_schema` en `plugin.json` para configuración admin (Plane URL, API key, workspace)
- Bot account: `Helpers.EnsureBot()` en `OnActivate` para crear bot que publica mensajes
- Slash commands: `api.RegisterCommand()` con `AutoComplete: true`
- Plane API: `https://{plane_url}/api/v1/workspaces/{slug}/projects/{id}/work-items/` (usar work-items, NO issues)

</code_context>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 01-foundation-core-plane-commands*
*Context gathered: 2026-03-17*
