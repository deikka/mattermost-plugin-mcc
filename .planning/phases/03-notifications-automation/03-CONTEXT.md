# Phase 3: Notifications + Automation - Context

**Gathered:** 2026-03-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Cambios en tareas de Plane (estado, asignacion, comentarios) se publican automaticamente en el canal de Mattermost vinculado al proyecto. Bot publica resumen periodico configurable del estado del proyecto. NO incluye edicion de tareas desde Mattermost, notificaciones por DM, ni filtros granulares por tipo de evento.

</domain>

<decisions>
## Implementation Decisions

### Eventos a notificar
- Tres tipos de eventos: cambios de estado, cambios de asignacion, nuevos comentarios
- NO notificar creacion de tareas nuevas (las creadas desde Mattermost ya tienen confirmacion efimera)
- Solo notificar en canales vinculados (binding 1:1 de Phase 2) — proyectos sin canal no generan notificaciones
- Todos los cambios visibles para todos — no ocultar cambios del usuario que los hizo
- Solo notificar cambios hechos directamente en Plane — los cambios originados desde el plugin (ej. /task plane create) no generan notificacion webhook para evitar duplicacion

### Formato de notificaciones
- Card rica (SlackAttachment) reutilizando el patron del link unfurling de Phase 2
- Titulo de la card incluye accion + nombre de tarea (ej. "🟡 Estado cambiado: Fix login bug")
- Mostrar transicion antes → despues para cambios de estado (ej. "Open → In Progress")
- Notificaciones de comentarios incluyen texto truncado (~200 caracteres) del comentario
- Cada card incluye link a la tarea en Plane

### Resumen periodico
- Configuracion por canal via comando: `/task plane digest daily|weekly|off`
- Hora de publicacion personalizable por canal
- Frecuencias soportadas: diario, semanal, desactivado (off)
- Contenido tipo dashboard: contadores por estado (Open/In Progress/Done) + tareas completadas en el periodo + tareas nuevas + cambios de estado + link al proyecto
- Post visible al canal completo (no efimero) — funciona como standup automatico

### Control de ruido
- Sin agrupacion temporal: cada cambio = una notificacion (volumen manejable para equipo de 2-10)
- Todo o nada: cuando las notificaciones estan activas, llegan los 3 tipos de eventos sin filtro por tipo
- Comando `/task plane notifications on|off` para pausar/reanudar notificaciones sin desvincular el proyecto
- Posts independientes: cada notificacion es un post separado, sin threads por tarea

### Claude's Discretion
- Mecanismo de recepcion de webhooks de Plane (endpoint HTTP, polling, o adaptador segun capacidades de Plane)
- Formato exacto de los campos en la SlackAttachment de notificaciones
- Esquema de KV store para configuracion de digest (prefijos, estructura)
- Implementacion del scheduler para resumenes periodicos (goroutine con ticker, cron plugin, etc.)
- Manejo de errores cuando Plane no envia datos completos en el webhook
- Hora default si el usuario no especifica hora personalizada

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `buildWorkItemAttachment()`: Construye SlackAttachment card con Estado/Prioridad/Asignado — reutilizable para notificaciones
- `stateGroupEmoji()` / `priorityLabel()`: Formateo visual de estado y prioridad
- `store.GetChannelBinding()`: Resuelve canal → proyecto para enrutar notificaciones
- `p.sendEphemeral()` / `p.API.CreatePost()`: Patrones de publicacion de mensajes (efimero y visible)
- `p.botUserID`: Bot account listo para publicar notificaciones
- `p.planeClient.ListProjects()` / `p.planeClient.GetWorkItemBySequence()`: Consultas a Plane API con cache
- `p.planeClient.ListWorkspaceMembers()`: Resolucion de nombres de usuarios
- `writeJSON()` / `writeError()` / `mattermostAuthMiddleware()`: Helpers HTTP en api.go
- `p.router` (gorilla/mux): Router HTTP listo para nuevas rutas de webhook

### Established Patterns
- Command handlers como funciones con signature estandar en command_handlers.go
- Routing via handler map en command_router.go con soporte de aliases
- KV store con prefijos (`channel_project_`, `user_plane_`, etc.) via store.Store
- SlackAttachment cards para informacion rica (link unfurling)
- Posts visibles al canal para eventos de equipo (binding announcements)
- Respuestas efimeras para confirmaciones personales
- Plane API calls con cache TTL via plane.Client

### Integration Points
- `command_router.go`: Registrar handlers para `plane digest`, `plane notifications`
- `command.go`: Anadir autocomplete para nuevos subcomandos
- `api.go / initAPI()`: Nuevo endpoint para recibir webhooks de Plane
- `plugin.go / OnActivate()`: Inicializar scheduler de resumenes periodicos
- `store/store.go`: Nuevos tipos DigestConfig + NotificationConfig con CRUD
- `plane/client.go`: Posibles nuevos metodos para consultas de actividad reciente

</code_context>

<specifics>
## Specific Ideas

- El dashboard del resumen periodico debe parecer un "standup automatico" — contadores + cambios concretos del periodo + link a Plane
- Las cards de notificacion reutilizan el estilo visual del link unfurling para consistencia
- La transicion "antes → despues" en cambios de estado es clave para entender el flujo sin abrir Plane
- El truncamiento de comentarios a ~200 chars evita mensajes enormes manteniendo contexto suficiente

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 03-notifications-automation*
*Context gathered: 2026-03-17*
