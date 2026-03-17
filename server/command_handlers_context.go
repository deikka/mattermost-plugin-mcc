package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// truncateTitle truncates text to maxLen characters, appending "..." if truncated.
// Newlines are replaced with spaces for a clean single-line title.
func truncateTitle(text string, maxLen int) string {
	// Remove newlines for title
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

// handleCreateTaskFromMessage handles the context menu action.
// Receives post_id from the webapp, fetches the post content,
// builds pre-populated dialog fields, and returns dialog config JSON
// for the webapp to open client-side via store.dispatch.
//
// NOTE: This handler does NOT call openCreateTaskDialogWithContext or
// p.API.OpenInteractiveDialog because neither approach works for the
// context menu flow -- registerPostDropdownMenuAction does not provide
// a trigger_id, and the /api/v4/actions/dialogs/open REST endpoint
// requires one. Instead, the server returns the dialog config as JSON
// and the webapp opens it client-side by dispatching the
// openInteractiveDialog Redux action, which bypasses trigger_id validation.
func (p *Plugin) handleCreateTaskFromMessage(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Check Plane connection
	connected, err := p.store.IsPlaneConnected(userID)
	if err != nil || !connected {
		writeError(w, http.StatusForbidden, "Please run /task connect first to link your Plane account")
		return
	}

	var req struct {
		PostID string `json:"post_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PostID == "" {
		writeError(w, http.StatusBadRequest, "post_id is required")
		return
	}

	// Fetch the original post
	post, appErr := p.API.GetPost(req.PostID)
	if appErr != nil {
		p.API.LogError("Failed to get post for context menu action", "error", appErr.Error())
		writeError(w, http.StatusInternalServerError, "Could not fetch post")
		return
	}

	// Build permalink: {siteURL}/{teamName}/pl/{postID}
	permalink := p.buildPermalink(post.Id, post.ChannelId)

	// Truncate message for title (~80 chars)
	title := truncateTitle(post.Message, 80)

	// Full message + permalink for description
	description := post.Message
	if permalink != "" {
		description += "\n\n---\n[Original message](" + permalink + ")"
	}

	// Check channel binding for project pre-selection
	binding, _ := p.store.GetChannelBinding(post.ChannelId)

	// Build dialog config and return to webapp for client-side opening
	projects, projErr := p.planeClient.ListProjects()
	if projErr != nil || len(projects) == 0 {
		writeError(w, http.StatusInternalServerError, "Could not fetch projects from Plane")
		return
	}

	projectOptions := make([]map[string]string, 0, len(projects))
	for _, proj := range projects {
		projectOptions = append(projectOptions, map[string]string{
			"text":  proj.Name,
			"value": proj.ID,
		})
	}

	// Pre-populate assignee options from default project
	var assigneeOptions []map[string]string
	defaultProjectID := projects[0].ID
	if binding != nil {
		defaultProjectID = binding.ProjectID
	}
	members, _ := p.planeClient.ListProjectMembers(defaultProjectID)
	for _, m := range members {
		displayName := m.DisplayName
		if displayName == "" {
			displayName = m.Email
		}
		assigneeOptions = append(assigneeOptions, map[string]string{
			"text":  displayName,
			"value": m.ID,
		})
	}

	// Default assignee = current user's Plane ID
	defaultAssignee := ""
	mapping, _ := p.store.GetPlaneUser(userID)
	if mapping != nil {
		defaultAssignee = mapping.PlaneUserID
	}

	// Build callback URL with source_post_id for reaction after creation
	callbackURL := fmt.Sprintf("/plugins/%s/api/v1/dialog/create-task?source_post_id=%s", manifestID, post.Id)

	// Return dialog configuration for the webapp to open via store.dispatch
	dialogConfig := map[string]interface{}{
		"url": callbackURL,
		"dialog": map[string]interface{}{
			"callback_id": "create_task_from_message",
			"title":       "Crear Tarea en Plane",
			"elements": []map[string]interface{}{
				{
					"display_name": "Title",
					"name":         "title",
					"type":         "text",
					"sub_type":     "text",
					"min_length":   1,
					"max_length":   255,
					"placeholder":  "Task title",
					"default":      title,
				},
				{
					"display_name": "Description",
					"name":         "description",
					"type":         "textarea",
					"optional":     true,
					"placeholder":  "Task description",
					"default":      description,
				},
				{
					"display_name": "Project",
					"name":         "project_id",
					"type":         "select",
					"options":      projectOptions,
					"default":      defaultProjectID,
				},
				{
					"display_name": "Priority",
					"name":         "priority",
					"type":         "select",
					"default":      "none",
					"optional":     true,
					"options": []map[string]string{
						{"text": "None", "value": "none"},
						{"text": "Low", "value": "low"},
						{"text": "Medium", "value": "medium"},
						{"text": "High", "value": "high"},
						{"text": "Urgent", "value": "urgent"},
					},
				},
				{
					"display_name": "Assignee",
					"name":         "assignee_id",
					"type":         "select",
					"optional":     true,
					"default":      defaultAssignee,
					"options":      assigneeOptions,
				},
				{
					"display_name": "Labels (comma-separated)",
					"name":         "labels",
					"type":         "text",
					"optional":     true,
					"placeholder":  "bug, frontend, urgent",
				},
			},
			"submit_label":     "Crear Tarea",
			"notify_on_cancel": false,
		},
	}

	writeJSON(w, http.StatusOK, dialogConfig)
}

// buildPermalink constructs a Mattermost permalink for a post.
// Returns empty string if SiteURL is not configured or the channel is a DM.
func (p *Plugin) buildPermalink(postID, channelID string) string {
	siteURL := ""
	if cfg := p.API.GetConfig(); cfg != nil && cfg.ServiceSettings.SiteURL != nil {
		siteURL = strings.TrimRight(*cfg.ServiceSettings.SiteURL, "/")
	}
	if siteURL == "" {
		return ""
	}

	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil || channel.TeamId == "" {
		return "" // DMs/group messages don't have a team
	}

	team, appErr := p.API.GetTeam(channel.TeamId)
	if appErr != nil {
		return ""
	}

	return fmt.Sprintf("%s/%s/pl/%s", siteURL, team.Name, postID)
}
