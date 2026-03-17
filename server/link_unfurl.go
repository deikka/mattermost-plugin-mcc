package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
)

// planeURLMatch holds the extracted project and work item IDs from a Plane URL.
type planeURLMatch struct {
	ProjectID  string
	WorkItemID string
}

// extractPlaneWorkItemURLs finds Plane work item URLs in a message.
// Returns matches with project ID and work item ID extracted.
// The planeURL and workspace are used to build the expected URL pattern.
func extractPlaneWorkItemURLs(message, planeURL, workspace string) []planeURLMatch {
	escapedBase := regexp.QuoteMeta(strings.TrimRight(planeURL, "/"))
	escapedWS := regexp.QuoteMeta(workspace)
	uuidPattern := `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`
	pattern := fmt.Sprintf(`%s/%s/projects/(%s)/work-items/(%s)`,
		escapedBase, escapedWS, uuidPattern, uuidPattern)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(message, -1)
	var results []planeURLMatch
	for _, m := range matches {
		if len(m) >= 3 {
			results = append(results, planeURLMatch{ProjectID: m[1], WorkItemID: m[2]})
		}
	}
	return results
}

// buildWorkItemAttachment creates a SlackAttachment card for a Plane work item.
// Used by link unfurling to show a rich preview of the work item.
func buildWorkItemAttachment(item *plane.WorkItem, planeURL, workspace, projectID string) *model.SlackAttachment {
	stateEmoji := stateGroupEmoji(item.StateGroup)
	pLabel := priorityLabel(item.Priority)
	stateName := item.StateName
	if stateName == "" {
		stateName = item.StateGroup
	}

	workItemURL := fmt.Sprintf("%s/%s/projects/%s/work-items/%s",
		strings.TrimRight(planeURL, "/"), workspace, projectID, item.ID)

	projectName := item.ProjectName
	if projectName == "" {
		projectName = projectID
	}

	fields := []*model.SlackAttachmentField{
		{Title: "Status", Value: stateEmoji + " " + stateName, Short: true},
		{Title: "Priority", Value: pLabel, Short: true},
	}

	// Add assignee if available (resolved by caller)
	if item.AssigneeName != "" {
		fields = append(fields, &model.SlackAttachmentField{
			Title: "Assigned", Value: item.AssigneeName, Short: true,
		})
	}

	// Add project name
	fields = append(fields, &model.SlackAttachmentField{
		Title: "Project", Value: projectName, Short: true,
	})

	return &model.SlackAttachment{
		Color:     "#3f76ff",
		Title:     item.Name,
		TitleLink: workItemURL,
		Fields:    fields,
		Footer:    "Plane",
	}
}

// handleLinkUnfurl processes a posted message for Plane URL unfurling.
// Called by MessageHasBeenPosted. Creates a bot reply with a rich preview
// card for the first detected Plane work item URL.
func (p *Plugin) handleLinkUnfurl(post *model.Post) {
	// Skip bot posts to avoid infinite loops
	if post.UserId == p.botUserID {
		return
	}

	cfg := p.getConfiguration()
	if cfg.PlaneURL == "" || cfg.PlaneWorkspace == "" {
		return
	}

	// Extract Plane work item URLs
	urls := extractPlaneWorkItemURLs(post.Message, cfg.PlaneURL, cfg.PlaneWorkspace)
	if len(urls) == 0 {
		return
	}

	// Process only the first URL to avoid spam
	match := urls[0]

	// Fetch work item details using global API key
	workItem, err := p.planeClient.GetWorkItem(match.ProjectID, match.WorkItemID)
	if err != nil {
		p.API.LogWarn("Failed to fetch work item for unfurl",
			"projectID", match.ProjectID,
			"workItemID", match.WorkItemID,
			"error", err.Error())
		return
	}

	// Resolve assignee name from workspace members cache if possible
	if len(workItem.Assignees) > 0 {
		members, err := p.planeClient.ListWorkspaceMembers()
		if err == nil {
			for _, m := range members {
				if m.Member.ID == workItem.Assignees[0] {
					name := m.Member.DisplayName
					if name == "" {
						name = strings.TrimSpace(m.Member.FirstName + " " + m.Member.LastName)
					}
					if name == "" {
						name = m.Member.Email
					}
					workItem.AssigneeName = name
					break
				}
			}
		}
	}

	// Build attachment
	attachment := buildWorkItemAttachment(workItem, cfg.PlaneURL, cfg.PlaneWorkspace, match.ProjectID)

	// Create bot reply with attachment
	replyPost := &model.Post{
		UserId:    p.botUserID,
		ChannelId: post.ChannelId,
		RootId:    post.Id,
		Message:   "",
	}
	model.ParseSlackAttachment(replyPost, []*model.SlackAttachment{attachment})

	if _, appErr := p.API.CreatePost(replyPost); appErr != nil {
		p.API.LogWarn("Failed to create unfurl reply",
			"postID", post.Id,
			"error", appErr.Error())
	}
}
