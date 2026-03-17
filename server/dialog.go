package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
)

// openCreateTaskDialog opens the interactive dialog for creating a task in Plane.
// It pre-populates project, assignee, and label options by calling the Plane API
// at dialog-open time (since Mattermost dialogs don't support true dynamic selects).
func openCreateTaskDialog(p *Plugin, triggerID, channelID, userID string) error {
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

	// Pre-populate assignee options from the first project's members
	var assigneeOptions []*model.PostActionOptions
	members, err := p.planeClient.ListProjectMembers(projects[0].ID)
	if err != nil {
		p.API.LogWarn("Failed to list members for dialog, assignee select will be empty", "error", err.Error())
	} else {
		assigneeOptions = make([]*model.PostActionOptions, 0, len(members))
		for _, m := range members {
			displayName := m.Member.DisplayName
			if displayName == "" {
				displayName = m.Member.Email
			}
			assigneeOptions = append(assigneeOptions, &model.PostActionOptions{
				Text:  displayName,
				Value: m.Member.ID,
			})
		}
	}

	// Determine default assignee (current user's Plane ID)
	defaultAssignee := ""
	mapping, err := p.store.GetPlaneUser(userID)
	if err == nil && mapping != nil {
		defaultAssignee = mapping.PlaneUserID
	}

	dialog := model.OpenDialogRequest{
		TriggerId: triggerID,
		URL:       fmt.Sprintf("/plugins/%s/api/v1/dialog/create-task", manifestID),
		Dialog: model.Dialog{
			CallbackId: "create_task",
			Title:      "Create Task in Plane",
			Elements: []model.DialogElement{
				{
					DisplayName: "Title",
					Name:        "title",
					Type:        "text",
					SubType:     "text",
					MinLength:   1,
					MaxLength:   255,
					Placeholder: "Task title",
				},
				{
					DisplayName: "Description",
					Name:        "description",
					Type:        "textarea",
					Optional:    true,
					Placeholder: "Task description (optional)",
				},
				{
					DisplayName: "Project",
					Name:        "project_id",
					Type:        "select",
					Options:     projectOptions,
					Default:     projects[0].ID,
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
			SubmitLabel:    "Create Task",
			NotifyOnCancel: false,
		},
	}

	if appErr := p.API.OpenInteractiveDialog(dialog); appErr != nil {
		return fmt.Errorf("open interactive dialog: %s", appErr.Error())
	}

	return nil
}
