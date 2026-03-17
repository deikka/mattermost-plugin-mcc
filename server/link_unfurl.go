package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
)

// planeURLMatch holds the extracted identifier and sequence from a Plane browse URL.
type planeURLMatch struct {
	Identifier string // e.g. "TENDERIO"
	SequenceID int    // e.g. 1
}

// extractPlaneWorkItemURLs finds Plane work item URLs in a message.
// Detects the browse URL format: {planeURL}/{workspace}/browse/{IDENTIFIER}-{N}
func extractPlaneWorkItemURLs(message, planeURL, workspace string) []planeURLMatch {
	escapedBase := regexp.QuoteMeta(strings.TrimRight(planeURL, "/"))
	escapedWS := regexp.QuoteMeta(workspace)
	pattern := fmt.Sprintf(`%s/%s/browse/([A-Z][A-Z0-9_]*)-(\d+)`,
		escapedBase, escapedWS)
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(message, -1)
	var results []planeURLMatch
	for _, m := range matches {
		if len(m) >= 3 {
			seqID, _ := strconv.Atoi(m[2])
			results = append(results, planeURLMatch{Identifier: m[1], SequenceID: seqID})
		}
	}
	return results
}

// buildWorkItemAttachment creates a SlackAttachment card for a Plane work item.
func buildWorkItemAttachment(item *plane.WorkItem, planeURL, workspace, identifier string, sequenceID int) *model.SlackAttachment {
	stateEmoji := stateGroupEmoji(item.StateGroup)
	pLabel := priorityLabel(item.Priority)
	stateName := item.StateName
	if stateName == "" {
		stateName = item.StateGroup
	}

	workItemURL := fmt.Sprintf("%s/%s/browse/%s-%d",
		strings.TrimRight(planeURL, "/"), workspace, identifier, sequenceID)

	projectName := item.ProjectName
	if projectName == "" {
		projectName = identifier
	}

	fields := []*model.SlackAttachmentField{
		{Title: "Estado", Value: stateEmoji + " " + stateName, Short: true},
		{Title: "Prioridad", Value: pLabel, Short: true},
	}

	if item.AssigneeName != "" {
		fields = append(fields, &model.SlackAttachmentField{
			Title: "Asignado", Value: item.AssigneeName, Short: true,
		})
	}

	fields = append(fields, &model.SlackAttachmentField{
		Title: "Proyecto", Value: projectName, Short: true,
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
func (p *Plugin) handleLinkUnfurl(post *model.Post) {
	if post.UserId == p.botUserID {
		return
	}

	cfg := p.getConfiguration()
	if cfg.PlaneURL == "" || cfg.PlaneWorkspace == "" {
		return
	}

	urls := extractPlaneWorkItemURLs(post.Message, cfg.PlaneURL, cfg.PlaneWorkspace)
	if len(urls) == 0 {
		return
	}

	match := urls[0]

	// Find project by identifier to get its UUID
	projects, err := p.planeClient.ListProjects()
	if err != nil {
		p.API.LogWarn("Failed to list projects for unfurl", "error", err.Error())
		return
	}

	var projectID string
	for _, proj := range projects {
		if strings.EqualFold(proj.Identifier, match.Identifier) {
			projectID = proj.ID
			break
		}
	}
	if projectID == "" {
		p.API.LogWarn("Project not found for unfurl", "identifier", match.Identifier)
		return
	}

	// Fetch work item by sequence ID
	workItem, err := p.planeClient.GetWorkItemBySequence(projectID, match.SequenceID)
	if err != nil {
		p.API.LogWarn("Failed to fetch work item for unfurl",
			"identifier", match.Identifier,
			"sequenceID", match.SequenceID,
			"error", err.Error())
		return
	}

	// Resolve assignee name
	if len(workItem.Assignees) > 0 {
		members, err := p.planeClient.ListWorkspaceMembers()
		if err == nil {
			for _, m := range members {
				if m.ID == workItem.Assignees[0] {
					name := m.DisplayName
					if name == "" {
						name = strings.TrimSpace(m.FirstName + " " + m.LastName)
					}
					if name == "" {
						name = m.Email
					}
					workItem.AssigneeName = name
					break
				}
			}
		}
	}

	attachment := buildWorkItemAttachment(workItem, cfg.PlaneURL, cfg.PlaneWorkspace, match.Identifier, match.SequenceID)

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
