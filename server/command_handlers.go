package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

const helpText = `**Task Management Commands**

**Plane**
- ` + "`/task plane create [title]`" + ` -- Create a new task (alias: ` + "`/task p c`" + `)
- ` + "`/task plane mine`" + ` -- Show your assigned tasks (alias: ` + "`/task p m`" + `)
- ` + "`/task plane status [project]`" + ` -- Show project status (alias: ` + "`/task p s`" + `)

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

		// Use first project (if only one, it's the right one; if multiple, use default)
		projectID := projects[0].ID
		projectName := projects[0].Name

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
		msg := formatTaskCreatedMessage(title, projectName, workItemURL)
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
// Stub: will be implemented in Plan 01-03.
func handlePlaneMine(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "This command is not yet implemented. Coming in the next update.")
}

// handlePlaneStatus handles /task plane status [project].
// Stub: will be implemented in Plan 01-03.
func handlePlaneStatus(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "This command is not yet implemented. Coming in the next update.")
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
