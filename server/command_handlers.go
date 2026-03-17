package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// workItemWithProject pairs a work item with its project metadata for display.
type workItemWithProject struct {
	plane.WorkItem
	ProjectName string
	ProjectID   string
}

// sortWorkItemsByUpdated sorts work items by UpdatedAt descending (most recent first).
func sortWorkItemsByUpdated(items []workItemWithProject) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
}

const helpText = `**Comandos de Gestion de Tareas**

**Plane**
- ` + "`/task plane create [titulo]`" + ` -- Crear una nueva tarea (alias: ` + "`/task p c`" + `)
- ` + "`/task plane mine`" + ` -- Ver tus tareas asignadas (alias: ` + "`/task p m`" + `)
- ` + "`/task plane status [detail] [proyecto]`" + ` -- Ver estado del proyecto (alias: ` + "`/task p s`" + `)
- ` + "`/task plane link [proyecto]`" + ` -- Vincular canal a un proyecto de Plane (alias: ` + "`/task p l`" + `)
- ` + "`/task plane unlink`" + ` -- Desvincular canal del proyecto de Plane (alias: ` + "`/task p u`" + `)

**Configuracion**
- ` + "`/task connect`" + ` -- Vincular tu cuenta de Mattermost con Plane
- ` + "`/task obsidian setup`" + ` -- Configurar endpoint de Obsidian REST API

**Otros**
- ` + "`/task help`" + ` -- Mostrar este mensaje de ayuda`

// handleHelp returns the formatted list of all available commands.
func handleHelp(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, helpText)
}

// handlePlaneCreate handles /task plane create [title].
// If subArgs are provided, performs quick inline creation with smart defaults.
// If no subArgs, opens an interactive dialog with all fields.
func handlePlaneCreate(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	mapping, ok := requirePlaneConnection(p, args)
	if !ok {
		return &model.CommandResponse{}
	}

	if !p.planeClient.IsConfigured() {
		return p.respondEphemeral(args,
			"No se pudo conectar con Plane. Verifica la URL y configuracion en System Console.")
	}

	// Quick inline mode: /task plane create Fix the login bug
	if len(subArgs) > 0 {
		title := strings.Join(subArgs, " ")
		// Strip surrounding quotes if present
		if len(title) >= 2 && ((title[0] == '"' && title[len(title)-1] == '"') || (title[0] == '\'' && title[len(title)-1] == '\'')) {
			title = title[1 : len(title)-1]
		}
		title = strings.TrimSpace(title)
		if title == "" {
			return p.respondEphemeral(args, "Uso: `/task plane create Tu titulo aqui`")
		}

		// Get projects to find target project
		projects, err := p.planeClient.ListProjects()
		if err != nil {
			p.API.LogError("Failed to list projects for inline create", "error", err.Error())
			return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
		}
		if len(projects) == 0 {
			return p.respondEphemeral(args, "No se encontraron proyectos en tu workspace de Plane.")
		}

		// Use first project as default
		targetProject := projects[0]
		suffix := ""

		// Check channel binding -- use bound project if available
		binding, _ := p.store.GetChannelBinding(args.ChannelId)
		if binding != nil {
			if proj := findProjectByNameOrID(projects, binding.ProjectName); proj != nil {
				targetProject = *proj
				suffix = fmt.Sprintf(" (Proyecto: %s)", targetProject.Name)
			}
		}

		req := &plane.CreateWorkItemRequest{
			Name:      title,
			Priority:  "none",
			Assignees: []string{mapping.PlaneUserID},
		}

		workItem, err := p.planeClient.CreateWorkItem(targetProject.ID, req)
		if err != nil {
			p.API.LogError("Failed to create work item inline", "error", err.Error())
			return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
		}

		workItemURL := p.planeClient.GetWorkItemURL(targetProject.Identifier, workItem.SequenceID)
		msg := formatTaskCreatedMessage(title, targetProject.Name, workItemURL) + suffix
		return p.respondEphemeral(args, msg)
	}

	// Dialog mode: no arguments -- open interactive dialog
	if err := openCreateTaskDialog(p, args.TriggerId, args.ChannelId, args.UserId); err != nil {
		p.API.LogError("Failed to open create task dialog", "error", err.Error())
		return p.respondEphemeral(args, "No se pudo abrir el dialogo de creacion. Intenta de nuevo.")
	}

	return &model.CommandResponse{}
}

// handlePlaneMine handles /task plane mine.
// Shows up to 10 tasks assigned to the current user across all projects.
func handlePlaneMine(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	mapping, ok := requirePlaneConnection(p, args)
	if !ok {
		return &model.CommandResponse{}
	}

	if !p.planeClient.IsConfigured() {
		return p.respondEphemeral(args,
			"No se pudo conectar con Plane. Verifica la URL y configuracion en System Console.")
	}

	projects, err := p.planeClient.ListProjects()
	if err != nil {
		p.API.LogError("Failed to list projects for mine", "error", err.Error())
		return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
	}
	if len(projects) == 0 {
		return p.respondEphemeral(args, "No se encontraron proyectos en tu workspace de Plane.")
	}

	// Check channel binding -- filter to bound project only
	binding, _ := p.store.GetChannelBinding(args.ChannelId)
	suffix := ""
	if binding != nil {
		var filtered []plane.Project
		for _, proj := range projects {
			if proj.ID == binding.ProjectID {
				filtered = []plane.Project{proj}
				break
			}
		}
		if len(filtered) > 0 {
			projects = filtered
			suffix = fmt.Sprintf(" (Proyecto: %s)", binding.ProjectName)
		}
	}

	// Fetch assigned work items from up to 5 projects
	var allItems []workItemWithProject
	maxProjects := 5
	if len(projects) < maxProjects {
		maxProjects = len(projects)
	}
	for _, proj := range projects[:maxProjects] {
		items, err := p.planeClient.ListWorkItems(proj.ID, mapping.PlaneUserID)
		if err != nil {
			p.API.LogWarn("Failed to list work items for project", "project", proj.Name, "error", err.Error())
			continue
		}
		for _, item := range items {
			allItems = append(allItems, workItemWithProject{
				WorkItem:    item,
				ProjectName: proj.Name,
				ProjectID:   proj.ID,
			})
		}
	}

	if len(allItems) == 0 {
		return p.respondEphemeral(args,
			"No tienes tareas asignadas en Plane. Crea una con `/task plane create`!")
	}

	// Sort by UpdatedAt descending and limit to 10
	sortWorkItemsByUpdated(allItems)
	if len(allItems) > 10 {
		allItems = allItems[:10]
	}

	// Format list
	var sb strings.Builder
	sb.WriteString("**Tus tareas asignadas:**" + suffix + "\n\n")
	for _, item := range allItems {
		emoji := stateGroupEmoji(item.StateGroup)
		pLabel := priorityLabel(item.Priority)
		stateName := item.StateName
		if stateName == "" {
			stateName = item.StateGroup
		}

		line := fmt.Sprintf("%s **%s** -- %s", emoji, item.Name, item.ProjectName)
		if pLabel != "" {
			line += " · " + pLabel
		}
		line += " · " + stateName
		sb.WriteString(line + "\n")
	}

	// Add footer link
	cfg := p.getConfiguration()
	planeBaseURL := strings.TrimRight(cfg.PlaneURL, "/")
	workspace := cfg.PlaneWorkspace
	sb.WriteString(fmt.Sprintf("\n---\n[Abrir Plane](%s/%s)", planeBaseURL, workspace))

	return p.respondEphemeral(args, sb.String())
}

// handlePlaneStatus handles /task plane status [detail] [project].
// Shows project summary with Open/In Progress/Done counts and progress bar.
// With "detail" flag, shows tasks grouped by state with titles.
func handlePlaneStatus(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	// Check for "detail" / "detalle" flag
	detailed := false
	if len(subArgs) > 0 && (strings.EqualFold(subArgs[0], "detail") || strings.EqualFold(subArgs[0], "detalle")) {
		detailed = true
		subArgs = subArgs[1:]
	}

	_, ok := requirePlaneConnection(p, args)
	if !ok {
		return &model.CommandResponse{}
	}

	if !p.planeClient.IsConfigured() {
		return p.respondEphemeral(args,
			"No se pudo conectar con Plane. Verifica la URL y configuracion en System Console.")
	}

	projects, err := p.planeClient.ListProjects()
	if err != nil {
		p.API.LogError("Failed to list projects for status", "error", err.Error())
		return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
	}
	if len(projects) == 0 {
		return p.respondEphemeral(args, "No se encontraron proyectos en tu workspace de Plane.")
	}

	// Check channel binding -- use bound project when no args specified
	var project *plane.Project
	binding, _ := p.store.GetChannelBinding(args.ChannelId)
	if binding != nil && len(subArgs) == 0 {
		if proj := findProjectByNameOrID(projects, binding.ProjectName); proj != nil {
			project = proj
		}
	}

	if project == nil && len(subArgs) > 0 {
		query := strings.Join(subArgs, " ")
		project = findProjectByNameOrID(projects, query)
		if project == nil {
			var names []string
			for _, p := range projects {
				names = append(names, p.Name+" ("+p.Identifier+")")
			}
			return p.respondEphemeral(args, fmt.Sprintf(
				"Proyecto '%s' no encontrado. Disponibles: %s",
				query, strings.Join(names, ", ")))
		}
	} else if project == nil && len(projects) == 1 {
		project = &projects[0]
	} else if project == nil {
		var names []string
		for _, p := range projects {
			names = append(names, p.Name+" ("+p.Identifier+")")
		}
		return p.respondEphemeral(args, fmt.Sprintf(
			"Cual proyecto? Disponibles: %s. Uso: `/task plane status {proyecto}`",
			strings.Join(names, ", ")))
	}

	// Fetch all work items for this project
	workItems, err := p.planeClient.ListProjectWorkItems(project.ID)
	if err != nil {
		p.API.LogError("Failed to list work items for status", "error", err.Error())
		return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
	}

	// Group by state group
	groupCounts := map[string]int{
		"backlog":   0,
		"unstarted": 0,
		"started":   0,
		"completed": 0,
		"cancelled": 0,
	}
	for _, item := range workItems {
		group := item.StateGroup
		if group == "" {
			group = "backlog"
		}
		groupCounts[group]++
	}

	open := groupCounts["backlog"] + groupCounts["unstarted"]
	inProgress := groupCounts["started"]
	done := groupCounts["completed"]
	total := len(workItems)

	// Calculate progress percentage
	percent := 0
	if total > 0 {
		percent = (done * 100) / total
	}

	bar := progressBar(done, total, 20)

	// Build project URL
	cfg := p.getConfiguration()
	planeBaseURL := strings.TrimRight(cfg.PlaneURL, "/")
	workspace := cfg.PlaneWorkspace
	projectURL := fmt.Sprintf("%s/%s/projects/%s/work-items/", planeBaseURL, workspace, project.ID)

	// Build status suffix for binding indicator
	bindingSuffix := ""
	if binding != nil && len(subArgs) == 0 {
		bindingSuffix = fmt.Sprintf(" (Proyecto: %s)", binding.ProjectName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Proyecto: %s** (%s)%s\n\n", project.Name, project.Identifier, bindingSuffix))
	sb.WriteString("| Estado | Cantidad |\n")
	sb.WriteString("|--------|----------|\n")
	sb.WriteString(fmt.Sprintf("| :white_circle: Abierto | %d |\n", open))
	sb.WriteString(fmt.Sprintf("| :large_blue_circle: En Progreso | %d |\n", inProgress))
	sb.WriteString(fmt.Sprintf("| :white_check_mark: Hecho | %d |\n\n", done))
	sb.WriteString(fmt.Sprintf("**Progreso:** %s %d%%\n", bar, percent))
	sb.WriteString(fmt.Sprintf("**Total:** %d tareas\n\n", total))

	if detailed {
		// Group work items by display category
		type stateCategory struct {
			emoji string
			label string
			items []plane.WorkItem
		}
		categories := []stateCategory{
			{":large_blue_circle:", "En Progreso", nil},
			{":white_circle:", "Abierto", nil},
			{":white_check_mark:", "Hecho", nil},
		}
		for _, item := range workItems {
			group := item.StateGroup
			if group == "" {
				group = "backlog"
			}
			switch group {
			case "started":
				categories[0].items = append(categories[0].items, item)
			case "backlog", "unstarted":
				categories[1].items = append(categories[1].items, item)
			case "completed":
				categories[2].items = append(categories[2].items, item)
			}
		}

		sb.WriteString("---\n\n")
		for _, cat := range categories {
			if len(cat.items) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("%s **%s** (%d)\n", cat.emoji, cat.label, len(cat.items)))
			for _, item := range cat.items {
				itemURL := p.planeClient.GetWorkItemURL(project.Identifier, item.SequenceID)
				pLabel := priorityLabel(item.Priority)
				line := fmt.Sprintf("- [%s](%s)", item.Name, itemURL)
				if pLabel != "" {
					line += " · " + pLabel
				}
				sb.WriteString(line + "\n")
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("_Usa `/task plane status detail` para ver las tareas_\n\n"))
	}

	sb.WriteString(fmt.Sprintf("[Abrir en Plane](%s)", projectURL))

	return p.respondEphemeral(args, sb.String())
}

// handleConnect handles /task connect.
// Links a Mattermost user to their Plane account via email match.
func handleConnect(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	// Check if Plane client is configured
	if !p.planeClient.IsConfigured() {
		return p.respondEphemeral(args,
			"Plane no esta configurado. Pide a tu admin que lo configure en **System Console > Plugins > Mattermost Command Center**.")
	}

	// Check if already connected
	existing, err := p.store.GetPlaneUser(args.UserId)
	if err != nil {
		p.API.LogError("Failed to check existing Plane connection", "error", err.Error())
		return p.respondEphemeral(args, "Algo salio mal comprobando tu conexion. Intenta de nuevo.")
	}
	if existing != nil {
		return p.respondEphemeral(args, fmt.Sprintf(
			"Tu cuenta ya esta vinculada a Plane como **%s** (%s). Usa `/task disconnect` para desvincular.",
			existing.PlaneDisplayName, existing.PlaneEmail))
	}

	// Get Mattermost user's email
	mmUser, appErr := p.API.GetUser(args.UserId)
	if appErr != nil {
		p.API.LogError("Failed to get Mattermost user", "error", appErr.Error())
		return p.respondEphemeral(args, "No se pudo obtener tu perfil de Mattermost. Intenta de nuevo.")
	}

	// Fetch workspace members from Plane
	members, err := p.planeClient.ListWorkspaceMembers()
	if err != nil {
		p.API.LogError("Failed to list Plane workspace members", "error", err.Error())
		return p.respondEphemeral(args,
			"No se pudo conectar con Plane. Verifica la red y la URL de Plane en **System Console > Plugins > Mattermost Command Center**.")
	}

	// Search for email match
	var matches []struct {
		userID      string
		email       string
		displayName string
	}
	for _, m := range members {
		if strings.EqualFold(m.Email, mmUser.Email) {
			displayName := m.DisplayName
			if displayName == "" {
				displayName = strings.TrimSpace(m.FirstName + " " + m.LastName)
			}
			matches = append(matches, struct {
				userID      string
				email       string
				displayName string
			}{
				userID:      m.ID,
				email:       m.Email,
				displayName: displayName,
			})
		}
	}

	switch len(matches) {
	case 0:
		return p.respondEphemeral(args, fmt.Sprintf(
			"No se encontro una cuenta de Plane con tu email (%s). "+
				"Verifica que el email de tu cuenta de Plane coincida con el de Mattermost, o contacta a tu admin.",
			mmUser.Email))
	case 1:
		// Auto-link
		mapping := &store.PlaneUserMapping{
			PlaneUserID:      matches[0].userID,
			PlaneEmail:       matches[0].email,
			PlaneDisplayName: matches[0].displayName,
			ConnectedAt:      time.Now().Unix(),
		}
		if err := p.store.SavePlaneUser(args.UserId, mapping); err != nil {
			p.API.LogError("Failed to save Plane user mapping", "error", err.Error())
			return p.respondEphemeral(args, "Cuenta conectada pero fallo al guardar. Intenta de nuevo.")
		}
		return p.respondEphemeral(args, fmt.Sprintf(
			"Conectado! Tu cuenta de Mattermost esta vinculada a **%s** (%s) en Plane.",
			matches[0].displayName, matches[0].email))
	default:
		return p.respondEphemeral(args,
			"Se encontraron multiples cuentas de Plane con tu email. Contacta a tu admin.")
	}
}

// handleObsidianSetup handles /task obsidian setup.
// Opens an interactive dialog for configuring the Obsidian REST API endpoint.
func handleObsidianSetup(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	dialog := model.OpenDialogRequest{
		TriggerId: args.TriggerId,
		URL:       fmt.Sprintf("/plugins/%s/api/v1/dialog/obsidian-setup", manifestID),
		Dialog: model.Dialog{
			CallbackId: "obsidian_setup",
			Title:      "Configurar Obsidian REST API",
			Elements: []model.DialogElement{
				{
					DisplayName: "Host",
					Name:        "host",
					Type:        "text",
					Default:     "127.0.0.1",
					HelpText:    "Hostname o IP de la maquina que ejecuta Obsidian",
					Placeholder: "127.0.0.1",
				},
				{
					DisplayName: "Port",
					Name:        "port",
					Type:        "text",
					Default:     "27124",
					HelpText:    "Puerto del plugin Obsidian Local REST API (default: 27124)",
					Placeholder: "27124",
				},
				{
					DisplayName: "API Key",
					Name:        "api_key",
					Type:        "text",
					SubType:     "password",
					HelpText:    "API key de los ajustes del plugin Obsidian Local REST API",
					Placeholder: "Your Obsidian REST API key",
				},
			},
			SubmitLabel:    "Guardar Configuracion",
			NotifyOnCancel: false,
		},
	}

	if appErr := p.API.OpenInteractiveDialog(dialog); appErr != nil {
		p.API.LogError("Failed to open Obsidian setup dialog", "error", appErr.Error())
		return p.respondEphemeral(args, "No se pudo abrir el dialogo de configuracion. Intenta de nuevo.")
	}

	return &model.CommandResponse{}
}

// requirePlaneConnection checks if the user has a Plane account linked.
// If not, sends an ephemeral message guiding them to run /task connect.
// Returns the mapping and true if connected, nil and false otherwise.
func requirePlaneConnection(p *Plugin, args *model.CommandArgs) (*store.PlaneUserMapping, bool) {
	mapping, err := p.store.GetPlaneUser(args.UserId)
	if err != nil {
		p.API.LogError("Failed to check Plane connection", "error", err.Error())
		p.sendEphemeral(args.UserId, args.ChannelId, "Algo salio mal. Intenta de nuevo.")
		return nil, false
	}
	if mapping == nil {
		p.sendEphemeral(args.UserId, args.ChannelId,
			"Aun no has vinculado tu cuenta de Plane. Usa `/task connect` para empezar.")
		return nil, false
	}
	return mapping, true
}

// formatTaskCreatedMessage formats the task creation confirmation message.
// Uses the exact format from CONTEXT.md specification.
func formatTaskCreatedMessage(title, projectName, workItemURL string) string {
	return fmt.Sprintf(":white_check_mark: Tarea creada: **%s** -- %s [Ver en Plane](%s)", title, projectName, workItemURL)
}

// stateGroupEmoji maps a Plane state group to its display emoji.
func stateGroupEmoji(group string) string {
	switch group {
	case "backlog":
		return ":inbox_tray:"
	case "unstarted":
		return ":white_circle:"
	case "started":
		return ":large_blue_circle:"
	case "completed":
		return ":white_check_mark:"
	case "cancelled":
		return ":no_entry_sign:"
	default:
		return ":white_circle:"
	}
}

// priorityLabel maps a Plane priority string to a human-readable label with emoji.
func priorityLabel(priority string) string {
	switch strings.ToLower(priority) {
	case "urgent":
		return "Urgent :rotating_light:"
	case "high":
		return "High :red_circle:"
	case "medium":
		return "Medium :orange_circle:"
	case "low":
		return "Low :large_blue_circle:"
	case "none", "":
		return ""
	default:
		return priority
	}
}

// progressBar returns an ASCII progress bar with the given fill ratio.
// width is the number of characters for the bar content (inside brackets).
func progressBar(done, total, width int) string {
	if total == 0 {
		return "[" + strings.Repeat("-", width) + "]"
	}
	filled := (done * width) / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("=", filled) + strings.Repeat("-", width-filled) + "]"
}

// findProjectByNameOrID searches for a project matching a query by name or identifier.
// Matching is case-insensitive.
func findProjectByNameOrID(projects []plane.Project, query string) *plane.Project {
	query = strings.ToLower(strings.TrimSpace(query))
	for i, proj := range projects {
		if strings.ToLower(proj.Name) == query || strings.ToLower(proj.Identifier) == query {
			return &projects[i]
		}
	}
	return nil
}
