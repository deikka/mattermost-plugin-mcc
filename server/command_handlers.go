package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

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
// Stub: will be implemented in Plan 01-03.
func handlePlaneCreate(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "This command is not yet implemented. Coming in the next update.")
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
