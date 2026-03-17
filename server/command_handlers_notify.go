package main

import (
	"fmt"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// handlePlaneNotifications handles /task plane notifications on|off.
// Toggles Plane change notifications for the current channel.
// Requires the channel to be bound to a Plane project first.
func handlePlaneNotifications(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	// Validate arguments
	if len(subArgs) == 0 || (subArgs[0] != "on" && subArgs[0] != "off") {
		return p.respondEphemeral(args, "Uso: `/task plane notifications on|off`")
	}

	// Check channel binding
	binding, err := p.store.GetChannelBinding(args.ChannelId)
	if err != nil {
		p.API.LogError("Failed to check channel binding", "error", err.Error())
		return p.respondEphemeral(args, "Algo salio mal. Intenta de nuevo.")
	}
	if binding == nil {
		return p.respondEphemeral(args, "Este canal no esta vinculado a un proyecto de Plane. Usa `/task plane link` primero.")
	}

	// Save notification config
	enabled := subArgs[0] == "on"
	config := &store.NotificationConfig{
		Enabled:   enabled,
		UpdatedBy: args.UserId,
		UpdatedAt: time.Now().Unix(),
	}
	if err := p.store.SaveNotificationConfig(args.ChannelId, config); err != nil {
		p.API.LogError("Failed to save notification config", "error", err.Error())
		return p.respondEphemeral(args, "Error al guardar la configuracion. Intenta de nuevo.")
	}

	// Return confirmation
	if enabled {
		return p.respondEphemeral(args, fmt.Sprintf(
			"Notificaciones de Plane activadas para este canal. Los cambios en tareas del proyecto **%s** se publicaran aqui.",
			binding.ProjectName))
	}
	return p.respondEphemeral(args, "Notificaciones de Plane desactivadas para este canal.")
}

// handlePlaneDigest handles /task plane digest daily|weekly|off [hour].
// Stub -- will be implemented in Plan 03-02.
func handlePlaneDigest(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	return p.respondEphemeral(args, "Digest command not yet implemented")
}
