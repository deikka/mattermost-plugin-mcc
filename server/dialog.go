package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// openCreateTaskDialog opens the interactive dialog for creating a task in Plane.
// It checks for a channel binding and pre-selects the bound project if available.
// Delegates to openCreateTaskDialogWithContext with empty pre-populated fields.
func openCreateTaskDialog(p *Plugin, triggerID, channelID, userID string) error {
	binding, _ := p.store.GetChannelBinding(channelID)
	return openCreateTaskDialogWithContext(p, triggerID, channelID, userID, "", "", binding, "")
}

// openCreateTaskDialogWithContext opens the task creation dialog with optional
// pre-populated fields. Used by both the slash command and the context menu.
//
// Parameters:
//   - preTitle: pre-populate the title field (empty string = no default)
//   - preDescription: pre-populate the description field (empty string = no default)
//   - binding: if non-nil, pre-select the bound project
//   - sourcePostID: if non-empty, passed as query param so the submission handler
//     can add a reaction to the original message
func openCreateTaskDialogWithContext(p *Plugin, triggerID, channelID, userID, preTitle, preDescription string, binding *store.ChannelProjectBinding, sourcePostID string) error {
	// Pre-populate project options from Plane
	projects, err := p.planeClient.ListProjects()
	if err != nil {
		p.API.LogError("Failed to list projects for dialog", "error", err.Error())
		return fmt.Errorf("could not fetch projects: %w", err)
	}
	if len(projects) == 0 {
		return fmt.Errorf("no projects found")
	}

	projectOptions := make([]*model.PostActionOptions, 0, len(projects))
	for _, proj := range projects {
		projectOptions = append(projectOptions, &model.PostActionOptions{
			Text:  proj.Name,
			Value: proj.ID,
		})
	}

	// Default project: use binding if available, otherwise first project
	defaultProjectID := projects[0].ID
	if binding != nil {
		defaultProjectID = binding.ProjectID
	}

	// Pre-populate assignee options from the first project's members
	var assigneeOptions []*model.PostActionOptions
	members, err := p.planeClient.ListProjectMembers(projects[0].ID)
	if err != nil {
		p.API.LogWarn("Failed to list members for dialog, assignee select will be empty", "error", err.Error())
	} else {
		assigneeOptions = make([]*model.PostActionOptions, 0, len(members))
		for _, m := range members {
			displayName := m.DisplayName
			if displayName == "" {
				displayName = m.Email
			}
			assigneeOptions = append(assigneeOptions, &model.PostActionOptions{
				Text:  displayName,
				Value: m.ID,
			})
		}
	}

	// Determine default assignee (current user's Plane ID)
	defaultAssignee := ""
	mapping, err := p.store.GetPlaneUser(userID)
	if err == nil && mapping != nil {
		defaultAssignee = mapping.PlaneUserID
	}

	// Build callback URL (include source_post_id if present)
	callbackURL := fmt.Sprintf("/plugins/%s/api/v1/dialog/create-task", manifestID)
	if sourcePostID != "" {
		callbackURL += "?source_post_id=" + sourcePostID
	}

	dialog := model.OpenDialogRequest{
		TriggerId: triggerID,
		URL:       callbackURL,
		Dialog: model.Dialog{
			CallbackId: "create_task",
			Title:      "Crear Tarea en Plane",
			Elements: []model.DialogElement{
				{
					DisplayName: "Title",
					Name:        "title",
					Type:        "text",
					SubType:     "text",
					Default:     preTitle,
					MinLength:   1,
					MaxLength:   255,
					Placeholder: "Task title",
				},
				{
					DisplayName: "Description",
					Name:        "description",
					Type:        "textarea",
					Optional:    true,
					Default:     preDescription,
					Placeholder: "Task description (optional)",
				},
				{
					DisplayName: "Project",
					Name:        "project_id",
					Type:        "select",
					Options:     projectOptions,
					Default:     defaultProjectID,
				},
				{
					DisplayName: "Priority",
					Name:        "priority",
					Type:        "select",
					Default:     "none",
					Optional:    true,
					Options: []*model.PostActionOptions{
						{Text: "None", Value: "none"},
						{Text: "Low", Value: "low"},
						{Text: "Medium", Value: "medium"},
						{Text: "High", Value: "high"},
						{Text: "Urgent", Value: "urgent"},
					},
				},
				{
					DisplayName: "Assignee",
					Name:        "assignee_id",
					Type:        "select",
					Optional:    true,
					Default:     defaultAssignee,
					Options:     assigneeOptions,
				},
				{
					DisplayName: "Labels (comma-separated)",
					Name:        "labels",
					Type:        "text",
					Optional:    true,
					Placeholder: "bug, frontend, urgent",
				},
			},
			SubmitLabel:    "Crear Tarea",
			NotifyOnCancel: false,
		},
	}

	if appErr := p.API.OpenInteractiveDialog(dialog); appErr != nil {
		return fmt.Errorf("open interactive dialog: %s", appErr.Error())
	}

	return nil
}
