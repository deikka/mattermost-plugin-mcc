package main

import (
	"fmt"
	"strconv"
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
// Configures periodic project summaries for the current channel.
// Requires the channel to be bound to a Plane project first.
func handlePlaneDigest(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	// Validate frequency argument
	if len(subArgs) == 0 || (subArgs[0] != "daily" && subArgs[0] != "weekly" && subArgs[0] != "off") {
		return p.respondEphemeral(args, "Uso: `/task plane digest daily|weekly|off [hora]` -- Hora es 0-23 (default: 9)")
	}

	frequency := subArgs[0]

	// Check channel binding
	binding, err := p.store.GetChannelBinding(args.ChannelId)
	if err != nil {
		p.API.LogError("Failed to check channel binding", "error", err.Error())
		return p.respondEphemeral(args, "Algo salio mal. Intenta de nuevo.")
	}
	if binding == nil {
		return p.respondEphemeral(args, "Este canal no esta vinculado a un proyecto de Plane. Usa `/task plane link` primero.")
	}

	// Parse optional hour (default 9)
	hour := 9
	if len(subArgs) > 1 {
		parsed, parseErr := strconv.Atoi(subArgs[1])
		if parseErr != nil || parsed < 0 || parsed > 23 {
			return p.respondEphemeral(args, "La hora debe ser un numero entre 0 y 23.")
		}
		hour = parsed
	}

	// Build and save config
	config := &store.DigestConfig{
		Frequency: frequency,
		Hour:      hour,
		Weekday:   1, // Monday default for weekly
		UpdatedBy: args.UserId,
		UpdatedAt: time.Now().Unix(),
	}
	if err := p.store.SaveDigestConfig(args.ChannelId, config); err != nil {
		p.API.LogError("Failed to save digest config", "error", err.Error())
		return p.respondEphemeral(args, "Error al guardar la configuracion. Intenta de nuevo.")
	}

	// Return confirmation
	switch frequency {
	case "daily":
		return p.respondEphemeral(args, fmt.Sprintf(
			"Resumen diario configurado para las %d:00 en este canal. Proyecto: **%s**",
			hour, binding.ProjectName))
	case "weekly":
		return p.respondEphemeral(args, fmt.Sprintf(
			"Resumen semanal configurado para los lunes a las %d:00 en este canal. Proyecto: **%s**",
			hour, binding.ProjectName))
	default: // "off"
		return p.respondEphemeral(args, "Resumen periodico desactivado para este canal.")
	}
}
