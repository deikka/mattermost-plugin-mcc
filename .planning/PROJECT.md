# Mattermost Command Center

## What This Is

Un plugin de Mattermost que convierte el chat en un centro de comando para gestión de tareas, conectando directamente con Plane (self-hosted) y Obsidian (vaults individuales vía Local REST API plugin). Permite crear tareas, consultar estados, vincular canales a proyectos y recibir notificaciones — todo sin salir de Mattermost.

Diseñado para un equipo pequeño (2-10 personas) donde Mattermost es el hub central de comunicación y toma de decisiones.

## Core Value

Cualquier conversación en Mattermost puede convertirse en una tarea accionable en Plane u Obsidian con un solo clic, sin cambiar de contexto.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Crear tareas en Plane desde mensajes de Mattermost (botón contextual + slash command)
- [ ] Crear tareas en Obsidian desde mensajes de Mattermost (botón contextual + slash command)
- [ ] Consultar mis tareas pendientes en Plane desde Mattermost
- [ ] Consultar estado de un proyecto Plane desde Mattermost
- [ ] Buscar tareas en Plane por título/descripción desde Mattermost
- [ ] Vincular un canal de Mattermost a un proyecto de Plane
- [ ] Tareas creadas en canal vinculado van automáticamente al proyecto Plane asociado
- [ ] Notificaciones de cambios en tareas de Plane se publican en el canal vinculado
- [ ] Resumen periódico (diario/semanal) del estado de un proyecto vinculado publicado en el canal
- [ ] Comandos contextuales: `/tasks` sin proyecto muestra las del proyecto vinculado al canal

### Out of Scope

- Sincronización bidireccional Obsidian ↔ Plane — complejidad alta, herramientas con propósitos diferentes
- Edición de tareas existentes desde Mattermost — v1 se centra en creación y consulta
- Integraciones con otras herramientas (Jira, Notion, etc.) — foco en Plane + Obsidian
- App móvil dedicada — se usa a través del cliente Mattermost existente
- Dashboard web propio — las queries desde chat son suficientes para v1

## Context

- **Mattermost**: Instancia privada self-hosted con acceso admin completo. Soporte para plugins (Go).
- **Plane**: Self-hosted con proyectos activos. API REST disponible.
- **Obsidian**: Vaults individuales por usuario. Cada usuario tiene instalado el plugin Local REST API que expone un endpoint HTTP local.
- **Equipo**: 2-10 personas. Iteración rápida, no necesita escalar a cientos de usuarios.
- **Flujo actual**: Las conversaciones en Mattermost generan decisiones y tareas que se crean manualmente en Plane u Obsidian — el contexto se pierde o se duplica.

## Constraints

- **Mattermost Plugin SDK**: Plugins en Go — el core del plugin debe ser Go
- **Obsidian REST API**: Requiere que cada usuario tenga el plugin activo y accesible desde la red donde corre el servicio bridge
- **Plane API**: Depende de la versión de Plane instalada — verificar endpoints disponibles
- **Red**: El servicio bridge necesita acceso de red tanto a Plane como a los endpoints de Obsidian REST API de cada usuario

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Plugin Mattermost (Go) + servicio bridge | Plugin nativo ofrece botones contextuales y deep integration; bridge centraliza lógica de conexión con APIs externas | — Pending |
| Obsidian vía Local REST API plugin | Es el mecanismo ya establecido en el equipo; evita reinventar sync | — Pending |
| Comandos separados para Plane y Obsidian | El usuario decide conscientemente dónde va la tarea | — Pending |

---
*Last updated: 2026-03-16 after initialization*
