package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// PlaneWebhookEvent represents the top-level webhook payload from Plane.
// Plane webhooks use nested JSON objects (not flat field notation).
type PlaneWebhookEvent struct {
	Event       string          `json:"event"`        // "issue", "issue_comment"
	Action      string          `json:"action"`       // "created", "updated", "deleted"
	WebhookID   string          `json:"webhook_id"`
	WorkspaceID string          `json:"workspace_id"`
	Data        json.RawMessage `json:"data"`
}

// WebhookIssueData represents a work item in webhook payload format.
// This differs from plane.WorkItem because Plane webhooks use nested objects
// for state, assignees, and labels instead of flat fields.
type WebhookIssueData struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	State      WebhookStateDetail `json:"state"`
	Assignees  []WebhookAssignee  `json:"assignees"`
	Labels     []WebhookLabel     `json:"labels"`
	Priority   string             `json:"priority"`
	Project    string             `json:"project"`
	SequenceID int                `json:"sequence_id"`
	CreatedAt  string             `json:"created_at"`
	UpdatedAt  string             `json:"updated_at"`
}

// WebhookStateDetail is the nested state object in a webhook issue payload.
type WebhookStateDetail struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Group string `json:"group"`
}

// WebhookAssignee is a nested assignee object in a webhook issue payload.
type WebhookAssignee struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// WebhookLabel is a nested label object in a webhook issue payload.
type WebhookLabel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// WebhookCommentData represents a comment event in webhook payload format.
type WebhookCommentData struct {
	ID          string `json:"id"`
	CommentHTML string `json:"comment_html"`
	ActorDetail struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
	} `json:"actor_detail"`
	Issue     string `json:"issue"`
	Project   string `json:"project"`
	CreatedAt string `json:"created_at"`
}

// handlePlaneWebhook handles incoming Plane webhook events.
// It verifies the HMAC signature, deduplicates via delivery ID, parses the event,
// and routes it to the appropriate handler.
func (p *Plugin) handlePlaneWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		p.API.LogError("Failed to read webhook body", "error", err.Error())
		writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// Verify HMAC signature
	signature := r.Header.Get("X-Plane-Signature")
	if !p.verifyWebhookSignature(body, signature) {
		writeError(w, http.StatusForbidden, "Invalid webhook signature")
		return
	}

	// Deduplicate via delivery ID
	deliveryID := r.Header.Get("X-Plane-Delivery")
	if p.isWebhookDuplicate(deliveryID) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	p.markWebhookProcessed(deliveryID)

	// Parse event
	var event PlaneWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		p.API.LogError("Failed to parse webhook event", "error", err.Error())
		writeError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	// Route event to handler
	switch {
	case event.Event == "issue" && event.Action == "updated":
		p.handleIssueWebhook(&event)
	case event.Event == "issue_comment" && event.Action == "created":
		p.handleIssueCommentWebhook(&event)
	default:
		p.API.LogInfo("Ignoring unhandled webhook event", "event", event.Event, "action", event.Action)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleIssueWebhook processes an issue update webhook event.
// It detects state changes and assignee changes, then posts notification cards
// to all bound channels where notifications are enabled.
func (p *Plugin) handleIssueWebhook(event *PlaneWebhookEvent) {
	var issueData WebhookIssueData
	if err := json.Unmarshal(event.Data, &issueData); err != nil {
		p.API.LogError("Failed to parse issue data from webhook", "error", err.Error())
		return
	}

	// Self-notification suppression: skip if this was a plugin-originated change
	actionKey, _ := p.API.KVGet("plugin_action_" + issueData.ID)
	if actionKey != nil {
		p.API.LogInfo("Skipping self-notification for plugin-originated change", "issue_id", issueData.ID)
		return
	}

	// Find bound channels for this project
	channels, err := p.store.GetProjectChannels(issueData.Project)
	if err != nil {
		p.API.LogError("Failed to get project channels", "error", err.Error())
		return
	}
	if len(channels) == 0 {
		return // No bound channels -- silently ignore
	}

	// Detect change types by comparing with cached state
	cfg := p.getConfiguration()
	taskURL := buildTaskURL(cfg.PlaneURL, cfg.PlaneWorkspace, "", issueData.Project, issueData.ID, issueData.SequenceID)

	// Read cached state
	var cachedState *store.WorkItemStateCache
	cachedData, _ := p.API.KVGet("work_item_state_" + issueData.ID)
	if cachedData != nil {
		cachedState = &store.WorkItemStateCache{}
		_ = json.Unmarshal(cachedData, cachedState)
	}

	// Detect state change
	var stateChanged bool
	if cachedState != nil && cachedState.StateGroup != issueData.State.Group {
		stateChanged = true
	}

	// Detect assignee change via hash comparison
	var assigneeChanged bool
	currentAssigneeHash := computeAssigneeHash(issueData.Assignees)
	previousHash, _ := p.API.KVGet("work_item_assignees_" + issueData.ID)
	if previousHash != nil && string(previousHash) != currentAssigneeHash {
		assigneeChanged = true
	}

	// Build appropriate notification card(s)
	var attachments []*model.SlackAttachment

	if stateChanged {
		oldState := ""
		oldGroup := ""
		if cachedState != nil {
			oldState = cachedState.StateName
			oldGroup = cachedState.StateGroup
		}
		att := buildStateChangeAttachment(
			issueData.Name,
			oldState, issueData.State.Name,
			oldGroup, issueData.State.Group,
			"", // actor name not available from webhook issue event
			taskURL,
		)
		attachments = append(attachments, att)
	}

	if assigneeChanged {
		att := buildAssigneeChangeAttachment(issueData.Name, issueData.Assignees, taskURL)
		attachments = append(attachments, att)
	}

	// If no specific change detected but we have previous cached state,
	// it might be another type of update -- skip notification to avoid noise
	if len(attachments) == 0 {
		// Update caches and return
		p.cacheIssueState(issueData.ID, &issueData)
		return
	}

	// Post notifications to each bound channel (if enabled)
	for _, channelID := range channels {
		notifConfig, err := p.store.GetNotificationConfig(channelID)
		if err != nil {
			p.API.LogWarn("Failed to get notification config", "channel_id", channelID, "error", err.Error())
			continue
		}
		// nil config = default enabled; explicitly disabled = skip
		if notifConfig != nil && !notifConfig.Enabled {
			continue
		}

		for _, att := range attachments {
			post := &model.Post{
				UserId:    p.botUserID,
				ChannelId: channelID,
				Message:   "",
			}
			model.ParseSlackAttachment(post, []*model.SlackAttachment{att})

			if _, appErr := p.API.CreatePost(post); appErr != nil {
				p.API.LogError("Failed to post webhook notification",
					"channel_id", channelID, "error", appErr.Error())
			}
		}
	}

	// Update caches
	p.cacheIssueState(issueData.ID, &issueData)
}

// handleIssueCommentWebhook processes an issue comment webhook event.
// It posts a notification card with the truncated comment text to all bound channels.
func (p *Plugin) handleIssueCommentWebhook(event *PlaneWebhookEvent) {
	var commentData WebhookCommentData
	if err := json.Unmarshal(event.Data, &commentData); err != nil {
		p.API.LogError("Failed to parse comment data from webhook", "error", err.Error())
		return
	}

	// Get project ID for routing
	projectID := commentData.Project
	if projectID == "" {
		p.API.LogWarn("Comment webhook missing project field, cannot route", "comment_id", commentData.ID)
		return
	}

	// Find bound channels
	channels, err := p.store.GetProjectChannels(projectID)
	if err != nil {
		p.API.LogError("Failed to get project channels for comment", "error", err.Error())
		return
	}
	if len(channels) == 0 {
		return
	}

	// Build comment card
	cfg := p.getConfiguration()
	taskURL := buildTaskURL(cfg.PlaneURL, cfg.PlaneWorkspace, "", projectID, commentData.Issue, 0)

	actorName := commentData.ActorDetail.DisplayName
	if actorName == "" {
		actorName = strings.TrimSpace(commentData.ActorDetail.FirstName + " " + commentData.ActorDetail.LastName)
	}

	commentText := stripHTMLTags(commentData.CommentHTML)
	commentText = truncateText(commentText, 200)

	// Use issue ID as a fallback name since webhook comment data may not include issue name
	issueName := commentData.Issue
	att := buildCommentAttachment(issueName, commentText, actorName, taskURL)

	// Post to each bound channel
	for _, channelID := range channels {
		notifConfig, err := p.store.GetNotificationConfig(channelID)
		if err != nil {
			p.API.LogWarn("Failed to get notification config", "channel_id", channelID, "error", err.Error())
			continue
		}
		if notifConfig != nil && !notifConfig.Enabled {
			continue
		}

		post := &model.Post{
			UserId:    p.botUserID,
			ChannelId: channelID,
			Message:   "",
		}
		model.ParseSlackAttachment(post, []*model.SlackAttachment{att})

		if _, appErr := p.API.CreatePost(post); appErr != nil {
			p.API.LogError("Failed to post comment notification",
				"channel_id", channelID, "error", appErr.Error())
		}
	}
}

// cacheIssueState stores the current issue state and assignee hash in KV store with 7-day TTL.
func (p *Plugin) cacheIssueState(issueID string, issueData *WebhookIssueData) {
	cache := &store.WorkItemStateCache{
		StateGroup: issueData.State.Group,
		StateName:  issueData.State.Name,
		CachedAt:   time.Now().Unix(),
	}
	data, _ := json.Marshal(cache)
	_, _ = p.API.KVSetWithOptions("work_item_state_"+issueID, data,
		model.PluginKVSetOptions{ExpireInSeconds: 604800}) // 7 days

	// Cache assignee hash
	hash := computeAssigneeHash(issueData.Assignees)
	_, _ = p.API.KVSetWithOptions("work_item_assignees_"+issueID, []byte(hash),
		model.PluginKVSetOptions{ExpireInSeconds: 604800})
}

// computeAssigneeHash computes a deterministic hash of the assignee list for change detection.
func computeAssigneeHash(assignees []WebhookAssignee) string {
	if len(assignees) == 0 {
		return ""
	}
	ids := make([]string, len(assignees))
	for i, a := range assignees {
		ids[i] = a.ID
	}
	sort.Strings(ids)
	joined := strings.Join(ids, ",")
	h := sha512.Sum512_256([]byte(joined))
	return hex.EncodeToString(h[:])
}

// === Notification card builders ===

// buildStateChangeAttachment creates a SlackAttachment for a state change notification.
func buildStateChangeAttachment(taskName, oldState, newState, oldStateGroup, newStateGroup, actorName, taskURL string) *model.SlackAttachment {
	emoji := stateGroupEmoji(newStateGroup)

	changeValue := ""
	if oldState == "" {
		changeValue = fmt.Sprintf("-> %s", newState)
	} else {
		changeValue = fmt.Sprintf("%s -> %s", oldState, newState)
	}

	fields := []*model.SlackAttachmentField{
		{Title: "Cambio", Value: changeValue, Short: true},
	}
	if actorName != "" {
		fields = append(fields, &model.SlackAttachmentField{Title: "Por", Value: actorName, Short: true})
	}

	return &model.SlackAttachment{
		Color:     "#3f76ff",
		Title:     fmt.Sprintf("%s Estado cambiado: %s", emoji, taskName),
		TitleLink: taskURL,
		Fields:    fields,
		Footer:    "Plane",
	}
}

// buildAssigneeChangeAttachment creates a SlackAttachment for an assignee change notification.
func buildAssigneeChangeAttachment(taskName string, assignees []WebhookAssignee, taskURL string) *model.SlackAttachment {
	names := make([]string, 0, len(assignees))
	for _, a := range assignees {
		name := a.DisplayName
		if name == "" {
			name = strings.TrimSpace(a.FirstName + " " + a.LastName)
		}
		if name == "" {
			name = a.Email
		}
		if name != "" {
			names = append(names, name)
		}
	}
	assigneeList := strings.Join(names, ", ")
	if assigneeList == "" {
		assigneeList = "Sin asignar"
	}

	return &model.SlackAttachment{
		Color:     "#3f76ff",
		Title:     fmt.Sprintf("Asignacion cambiada: %s", taskName),
		TitleLink: taskURL,
		Fields: []*model.SlackAttachmentField{
			{Title: "Asignados", Value: assigneeList, Short: true},
		},
		Footer: "Plane",
	}
}

// buildCommentAttachment creates a SlackAttachment for a new comment notification.
func buildCommentAttachment(taskName, commentText, actorName, taskURL string) *model.SlackAttachment {
	fields := []*model.SlackAttachmentField{}
	if actorName != "" {
		fields = append(fields, &model.SlackAttachmentField{Title: "Por", Value: actorName, Short: true})
	}

	return &model.SlackAttachment{
		Color:     "#3f76ff",
		Title:     fmt.Sprintf("Nuevo comentario: %s", taskName),
		TitleLink: taskURL,
		Text:      commentText,
		Fields:    fields,
		Footer:    "Plane",
	}
}

// === Helper functions ===

// buildTaskURL constructs a URL for a work item in Plane.
// If projectIdentifier is known, builds browse URL; otherwise builds direct project URL.
func buildTaskURL(planeURL, workspace, projectIdentifier, projectID, workItemID string, sequenceID int) string {
	base := strings.TrimRight(planeURL, "/")
	if projectIdentifier != "" && sequenceID > 0 {
		return fmt.Sprintf("%s/%s/browse/%s-%d", base, workspace, projectIdentifier, sequenceID)
	}
	return fmt.Sprintf("%s/%s/projects/%s/work-items/%s", base, workspace, projectID, workItemID)
}

// stripHTMLTags removes HTML tags from a string using a simple regex.
func stripHTMLTags(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(s, "")
}

// truncateText truncates a string to maxLen characters, appending "..." if truncated.
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// verifyWebhookSignature validates the HMAC-SHA256 signature of a webhook payload.
// If PlaneWebhookSecret is empty, all signatures are accepted (permissive mode).
func (p *Plugin) verifyWebhookSignature(body []byte, signature string) bool {
	cfg := p.getConfiguration()
	if cfg.PlaneWebhookSecret == "" {
		return true
	}
	mac := hmac.New(func() hash.Hash { return sha256.New() }, []byte(cfg.PlaneWebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// isWebhookDuplicate checks if a webhook delivery has already been processed.
func (p *Plugin) isWebhookDuplicate(deliveryID string) bool {
	if deliveryID == "" {
		return false
	}
	data, _ := p.API.KVGet("webhook_dedup_" + deliveryID)
	return data != nil
}

// markWebhookProcessed records a webhook delivery ID to prevent duplicate processing.
// The entry expires after 1 hour.
func (p *Plugin) markWebhookProcessed(deliveryID string) {
	if deliveryID == "" {
		return
	}
	_, _ = p.API.KVSetWithOptions("webhook_dedup_"+deliveryID, []byte("1"), model.PluginKVSetOptions{ExpireInSeconds: 3600})
}
