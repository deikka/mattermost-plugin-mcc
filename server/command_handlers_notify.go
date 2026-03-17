package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// handlePlaneNotifications handles /task plane notifications on|off.
// Stub -- will be implemented in Plan 03-01.
func handlePlaneNotifications(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "Notifications command not yet implemented")
}

// handlePlaneDigest handles /task plane digest daily|weekly|off [hour].
// Stub -- will be implemented in Plan 03-02.
func handlePlaneDigest(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "Digest command not yet implemented")
}
