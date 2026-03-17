package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
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
	Issue        string `json:"issue"`
	IssueName    string `json:"-"` // Populated by handler if available
	IssueProject string `json:"-"` // Populated by handler if available
	CreatedAt    string `json:"created_at"`
}

// handlePlaneWebhook handles incoming Plane webhook events.
// Stub -- will be fully implemented in Plan 03-01.
func (p *Plugin) handlePlaneWebhook(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// verifyWebhookSignature validates the HMAC-SHA256 signature of a webhook payload.
// If PlaneWebhookSecret is empty, all signatures are accepted (permissive mode).
func (p *Plugin) verifyWebhookSignature(body []byte, signature string) bool {
	cfg := p.getConfiguration()
	if cfg.PlaneWebhookSecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(cfg.PlaneWebhookSecret))
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
