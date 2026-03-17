package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// handlePlaneLink handles /task plane link [project].
// Binds the current channel to a Plane project. Posts a visible message to the channel.
func handlePlaneLink(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	_, ok := requirePlaneConnection(p, args)
	if !ok {
		return &model.CommandResponse{}
	}

	if !p.planeClient.IsConfigured() {
		return p.respondEphemeral(args,
			"No se pudo conectar con Plane. Verifica la URL y configuracion en System Console.")
	}

	// List available projects
	projects, err := p.planeClient.ListProjects()
	if err != nil {
		p.API.LogError("Failed to list projects for link", "error", err.Error())
		return p.respondEphemeral(args, "Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
	}
	if len(projects) == 0 {
		return p.respondEphemeral(args, "No se encontraron proyectos en tu workspace de Plane.")
	}

	// If no project specified, show usage with available projects
	if len(subArgs) == 0 {
		var names []string
		for _, proj := range projects {
			names = append(names, proj.Name+" ("+proj.Identifier+")")
		}
		return p.respondEphemeral(args, fmt.Sprintf(
			"Uso: `/task plane link <proyecto>`\n\nProyectos disponibles: %s",
			strings.Join(names, ", ")))
	}

	// Find matching project
	query := strings.Join(subArgs, " ")
	project := findProjectByNameOrID(projects, query)
	if project == nil {
		var names []string
		for _, proj := range projects {
			names = append(names, proj.Name+" ("+proj.Identifier+")")
		}
		return p.respondEphemeral(args, fmt.Sprintf(
			"Proyecto '%s' no encontrado. Disponibles: %s",
			query, strings.Join(names, ", ")))
	}

	// Create and save the binding
	binding := &store.ChannelProjectBinding{
		ProjectID:   project.ID,
		ProjectName: project.Name,
		BoundBy:     args.UserId,
		BoundAt:     time.Now().Unix(),
	}

	if err := p.store.SaveChannelBinding(args.ChannelId, binding); err != nil {
		p.API.LogError("Failed to save channel binding", "error", err.Error())
		return p.respondEphemeral(args, "Error al guardar la vinculacion. Intenta de nuevo.")
	}

	// Post visible message to the channel (not ephemeral)
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: args.ChannelId,
		Message:   fmt.Sprintf(":link: Canal vinculado al proyecto **%s**", project.Name),
	}
	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("Failed to post link notification", "error", appErr.Error())
	}

	return &model.CommandResponse{}
}

// handlePlaneUnlink handles /task plane unlink.
// Removes the channel-project binding. Posts a visible message to the channel.
func handlePlaneUnlink(p *Plugin, c *plugin.Context, args *model.CommandArgs, subArgs []string) *model.CommandResponse {
	_, ok := requirePlaneConnection(p, args)
	if !ok {
		return &model.CommandResponse{}
	}

	// Check if channel is bound
	binding, err := p.store.GetChannelBinding(args.ChannelId)
	if err != nil {
		p.API.LogError("Failed to get channel binding", "error", err.Error())
		return p.respondEphemeral(args, "Error al verificar la vinculacion. Intenta de nuevo.")
	}
	if binding == nil {
		return p.respondEphemeral(args,
			"Este canal no esta vinculado a ningun proyecto. Usa `/task plane link <proyecto>` para vincularlo.")
	}

	// Delete the binding
	if err := p.store.DeleteChannelBinding(args.ChannelId); err != nil {
		p.API.LogError("Failed to delete channel binding", "error", err.Error())
		return p.respondEphemeral(args, "Error al desvincular el canal. Intenta de nuevo.")
	}

	// Post visible message to the channel
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: args.ChannelId,
		Message:   fmt.Sprintf(":broken_chain: Canal desvinculado del proyecto **%s**", binding.ProjectName),
	}
	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("Failed to post unlink notification", "error", appErr.Error())
	}

	return &model.CommandResponse{}
}
