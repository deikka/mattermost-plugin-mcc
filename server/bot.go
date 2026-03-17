package main

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
)

// ensureBot creates or retrieves the bot account used by this plugin.
func (p *Plugin) ensureBot() error {
	botID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    "task-bot",
		DisplayName: "Task Bot",
		Description: "Mattermost Command Center - Task management bot",
	})
	if err != nil {
		return errors.Wrap(err, "failed to ensure bot")
	}

	p.botUserID = botID
	return nil
}

// sendEphemeral sends an ephemeral message visible only to the specified user.
func (p *Plugin) sendEphemeral(userID, channelID, message string) {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   message,
	}
	p.API.SendEphemeralPost(userID, post)
}

// respondEphemeral sends an ephemeral message and returns an empty command response.
// This is the standard pattern for slash command handlers that reply ephemerally.
func (p *Plugin) respondEphemeral(args *model.CommandArgs, message string) *model.CommandResponse {
	p.sendEphemeral(args.UserId, args.ChannelId, message)
	return &model.CommandResponse{}
}
