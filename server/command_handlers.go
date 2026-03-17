package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
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
// Stub: will be implemented in Plan 01-02.
func handleConnect(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "This command is not yet implemented. Coming in the next update.")
}

// handleObsidianSetup handles /task obsidian setup.
// Stub: will be implemented in Plan 01-02.
func handleObsidianSetup(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "This command is not yet implemented. Coming in the next update.")
}
