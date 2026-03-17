# Phase 2: Channel Intelligence + Context Menu - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Convertir canales de Mattermost en espacios de proyecto: vincular canales a proyectos de Plane via `/task plane link`, crear tareas desde el menu contextual "..." de mensajes con texto pre-poblado, auto-seleccionar proyecto en canales vinculados, y mostrar previews inline al pegar URLs de tareas de Plane (link unfurling).

</domain>

<decisions>
## Implementation Decisions

### Channel-Project Binding
- Relacion 1:1: un canal solo puede estar vinculado a un proyecto de Plane
- `/task plane link` crea/reemplaza el binding; `/task plane unlink` lo elimina
- Cualquier usuario conectado (que haya hecho `/task connect`) puede vincular/desvincular
- Al vincular, el bot publica un mensaje visible para todo el canal (no efimero): "Canal vinculado al proyecto X"
- Al desvincular, mismo patron: post visible al canal

### Menu Contextual de Mensajes
- Se abre el dialog de creacion pre-poblado (no creacion directa)
- Titulo pre-poblado con las primeras ~80 caracteres del mensaje
- Descripcion pre-poblada con el texto completo del mensaje + permalink al mensaje original
- Tras crear la tarea, el bot anade emoji reaction :memo: (📝) al mensaje original como indicador visual
- Confirmacion efimera al usuario con link a la tarea creada (patron existente de formatTaskCreatedMessage)

### Link Unfurling
- Al pegar URL de tarea de Plane en chat, se muestra attachment debajo del mensaje (patron Mattermost estandar)
- Info mostrada: titulo, estado (con emoji), asignado, prioridad, nombre del proyecto
- Usa la API key global del admin (no requiere que el usuario tenga /task connect)
- Funciona en cualquier canal, no solo en canales vinculados
- Patron de URL a detectar: URLs que matcheen el PlaneURL configurado + path de work item

### Auto-seleccion de Proyecto
- En canal vinculado, el dialog de creacion pre-selecciona el proyecto vinculado pero es editable
- En inline create (`/task plane create titulo`), usa el proyecto vinculado automaticamente
- `/task plane mine` en canal vinculado filtra solo tareas del proyecto vinculado
- `/task plane status` en canal vinculado muestra el estado del proyecto vinculado sin pedir nombre
- Las respuestas efimeras incluyen "(Proyecto: X)" cuando se usa auto-seleccion para que el usuario sepa que proyecto se esta usando
- En canal sin binding, comportamiento actual sin cambios (primer proyecto como default para inline, selector para status)

### Claude's Discretion
- Longitud exacta del texto del mensaje usado como descripcion (truncamiento si necesario por limites de API)
- Formato exacto del attachment de link unfurling (colores, layout del card)
- Manejo de URLs de Plane que no correspondan a work items validos o accesibles
- Pattern matching para detectar URLs de Plane (regex vs string match)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `store.Store`: KV store con patron prefix + Get/Save/Delete — listo para nuevo prefijo `channel_project_`
- `openCreateTaskDialog()` / `handleCreateTaskDialog()`: Dialog de creacion con 6 campos — reutilizable con pre-poblado
- `formatTaskCreatedMessage()`: Formato de confirmacion — reutilizable
- `requirePlaneConnection()`: Guard de conexion — reutilizable para comandos que necesitan binding
- `findProjectByNameOrID()`: Busqueda de proyecto por nombre/ID — reutilizable para link/status
- `stateGroupEmoji()` / `priorityLabel()`: Formateo de estado/prioridad — reutilizable para unfurling
- `mattermostAuthMiddleware()`: Auth middleware HTTP — reutilizable para nuevos endpoints
- `p.sendEphemeral()` / `p.respondEphemeral()`: Helpers de respuesta efimera

### Established Patterns
- Command handlers como funciones standalone con signature `func(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse`
- Routing via handler map en command_router.go con soporte de aliases
- Plane API calls con cache TTL via plane.Client
- Dialog submissions via HTTP POST handlers en api.go
- Error handling: log error + respond ephemeral con mensaje amigable

### Integration Points
- `command_router.go`: Registrar nuevos handlers para `plane link`, `plane unlink`
- `command.go`: Anadir autocomplete entries para nuevos subcomandos
- `plugin.go`: Hook `MessageHasBeenPosted` para link unfurling; hook `MessageWillBePosted` no existe, pero se puede registrar post-action
- `plugin.json`: Registrar message action (context menu) en el manifest
- `api.go`: Nuevo endpoint para dialog submission desde context menu
- `store/store.go`: Nuevo tipo `ChannelProjectBinding` + CRUD con prefijo

</code_context>

<specifics>
## Specific Ideas

- El post de binding visible al canal permite que todo el equipo sepa que canal va con que proyecto — transparencia
- La reaction :memo: en el mensaje original es un indicador visual no intrusivo de que "de este mensaje salio una tarea"
- El unfurling en cualquier canal hace que compartir links de Plane sea util incluso en canales de discusion general
- El indicador "(Proyecto: X)" en respuestas efimeras evita confusion cuando el usuario esta en un canal vinculado y no recuerda a que proyecto

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-channel-intelligence-context-menu*
*Context gathered: 2026-03-17*
