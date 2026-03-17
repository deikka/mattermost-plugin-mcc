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

const helpText = `**Task Management Commands**

**Plane**
- ` + "`/task plane create [title]`" + ` -- Create a new task (alias: ` + "`/task p c`" + `)
- ` + "`/task plane mine`" + ` -- Show your assigned tasks (alias: ` + "`/task p m`" + `)
- ` + "`/task plane status [project]`" + ` -- Show project status (alias: ` + "`/task p s`" + `)
- ` + "`/task plane link [project]`" + ` -- Bind channel to a Plane project (alias: ` + "`/task p l`" + `)
- ` + "`/task plane unlink`" + ` -- Unbind channel from Plane project (alias: ` + "`/task p u`" + `)

**Configuration**
- ` + "`/task connect`" + ` -- Link your Mattermost account with Plane
- ` + "`/task obsidian setup`" + ` -- Configure Obsidian REST API endpoint

**Other**
- ` + "`/task help`" + ` -- Show this help message`

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
		projectID := projects[0].ID
		projectName := projects[0].Name
		suffix := ""

		// Check channel binding -- use bound project if available
		binding, _ := p.store.GetChannelBinding(args.ChannelId)
		if binding != nil {
			if proj := findProjectByNameOrID(projects, binding.ProjectName); proj != nil {
				projectID = proj.ID
				projectName = proj.Name
				suffix = fmt.Sprintf(" (Proyecto: %s)", projectName)
			}
		}

		req := &plane.CreateWorkItemRequest{
			Name:      title,
			Priority:  "none",
			Assignees: []string{mapping.PlaneUserID},
		}

		workItem, err := p.planeClient.CreateWorkItem(projectID, req)
		if err != nil {
			p.API.LogError("Failed to create work item inline", "error", err.Error())
			return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
		}

		workItemURL := p.planeClient.GetWorkItemURL(projectID, workItem.ID)
		msg := formatTaskCreatedMessage(title, projectName, workItemURL) + suffix
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
			"You have no tasks assigned in Plane. Create one with `/task plane create`!")
	}

	// Sort by UpdatedAt descending and limit to 10
	sortWorkItemsByUpdated(allItems)
	if len(allItems) > 10 {
		allItems = allItems[:10]
	}

	// Format list
	var sb strings.Builder
	sb.WriteString("**Your assigned tasks:**" + suffix + "\n\n")
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
	sb.WriteString(fmt.Sprintf("\n---\n[Open Plane](%s/%s)", planeBaseURL, workspace))

	return p.respondEphemeral(args, sb.String())
}

// handlePlaneStatus handles /task plane status [project].
// Shows project summary with Open/In Progress/Done counts and progress bar.
func handlePlaneStatus(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
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
			"Which project? Available: %s. Usage: `/task plane status {project}`",
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
	sb.WriteString(fmt.Sprintf("**Project: %s** (%s)%s\n\n", project.Name, project.Identifier, bindingSuffix))
	sb.WriteString("| Status | Count |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| :white_circle: Open | %d |\n", open))
	sb.WriteString(fmt.Sprintf("| :large_blue_circle: In Progress | %d |\n", inProgress))
	sb.WriteString(fmt.Sprintf("| :white_check_mark: Done | %d |\n\n", done))
	sb.WriteString(fmt.Sprintf("**Progress:** %s %d%%\n", bar, percent))
	sb.WriteString(fmt.Sprintf("**Total:** %d work items\n\n", total))
	sb.WriteString(fmt.Sprintf("[Open in Plane](%s)", projectURL))

	return p.respondEphemeral(args, sb.String())
}

// handleConnect handles /task connect.
// Links a Mattermost user to their Plane account via email match.
func handleConnect(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	// Check if Plane client is configured
	if !p.planeClient.IsConfigured() {
		return p.respondEphemeral(args,
			"Plane is not configured yet. Ask your admin to set it up in **System Console > Plugins > Mattermost Command Center**.")
	}

	// Check if already connected
	existing, err := p.store.GetPlaneUser(args.UserId)
	if err != nil {
		p.API.LogError("Failed to check existing Plane connection", "error", err.Error())
		return p.respondEphemeral(args, "Something went wrong checking your connection status. Please try again.")
	}
	if existing != nil {
		return p.respondEphemeral(args, fmt.Sprintf(
			"Your account is already linked to Plane as **%s** (%s). Run `/task disconnect` to unlink.",
			existing.PlaneDisplayName, existing.PlaneEmail))
	}

	// Get Mattermost user's email
	mmUser, appErr := p.API.GetUser(args.UserId)
	if appErr != nil {
		p.API.LogError("Failed to get Mattermost user", "error", appErr.Error())
		return p.respondEphemeral(args, "Could not retrieve your Mattermost profile. Please try again.")
	}

	// Fetch workspace members from Plane
	members, err := p.planeClient.ListWorkspaceMembers()
	if err != nil {
		p.API.LogError("Failed to list Plane workspace members", "error", err.Error())
		return p.respondEphemeral(args,
			"Could not reach Plane. Check your network and Plane URL in **System Console > Plugins > Mattermost Command Center**.")
	}

	// Search for email match
	var matches []struct {
		userID      string
		email       string
		displayName string
	}
	for _, m := range members {
		if strings.EqualFold(m.Member.Email, mmUser.Email) {
			displayName := m.Member.DisplayName
			if displayName == "" {
				displayName = strings.TrimSpace(m.Member.FirstName + " " + m.Member.LastName)
			}
			matches = append(matches, struct {
				userID      string
				email       string
				displayName string
			}{
				userID:      m.Member.ID,
				email:       m.Member.Email,
				displayName: displayName,
			})
		}
	}

	switch len(matches) {
	case 0:
		return p.respondEphemeral(args, fmt.Sprintf(
			"Could not find a Plane account matching your email (%s). "+
				"Please verify your Plane account email matches your Mattermost email, or contact your admin.",
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
			return p.respondEphemeral(args, "Connected your account but failed to save. Please try again.")
		}
		return p.respondEphemeral(args, fmt.Sprintf(
			"Connected! Your Mattermost account is now linked to **%s** (%s) in Plane.",
			matches[0].displayName, matches[0].email))
	default:
		return p.respondEphemeral(args,
			"Found multiple Plane accounts matching your email. Please contact your admin.")
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
			Title:      "Configure Obsidian REST API",
			Elements: []model.DialogElement{
				{
					DisplayName: "Host",
					Name:        "host",
					Type:        "text",
					Default:     "127.0.0.1",
					HelpText:    "Hostname or IP of the machine running Obsidian",
					Placeholder: "127.0.0.1",
				},
				{
					DisplayName: "Port",
					Name:        "port",
					Type:        "text",
					Default:     "27124",
					HelpText:    "Port for the Obsidian Local REST API plugin (default: 27124)",
					Placeholder: "27124",
				},
				{
					DisplayName: "API Key",
					Name:        "api_key",
					Type:        "text",
					SubType:     "password",
					HelpText:    "API key from the Obsidian Local REST API plugin settings",
					Placeholder: "Your Obsidian REST API key",
				},
			},
			SubmitLabel:    "Save Configuration",
			NotifyOnCancel: false,
		},
	}

	if appErr := p.API.OpenInteractiveDialog(dialog); appErr != nil {
		p.API.LogError("Failed to open Obsidian setup dialog", "error", appErr.Error())
		return p.respondEphemeral(args, "Could not open the configuration dialog. Please try again.")
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
		p.sendEphemeral(args.UserId, args.ChannelId, "Something went wrong. Please try again.")
		return nil, false
	}
	if mapping == nil {
		p.sendEphemeral(args.UserId, args.ChannelId,
			"You haven't linked your Plane account yet. Run `/task connect` to get started.")
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
