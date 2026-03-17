package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// === Real tests for HMAC signature verification ===

func TestVerifyWebhookSignature_Valid(t *testing.T) {
	p := &Plugin{}
	p.configuration = &configuration{
		PlaneWebhookSecret: "my-secret-key",
	}

	body := []byte(`{"event":"issue","action":"updated"}`)
	signature := computeTestHMAC(body, "my-secret-key")

	result := p.verifyWebhookSignature(body, signature)
	assert.True(t, result, "Valid signature should be accepted")
}

func TestVerifyWebhookSignature_Invalid(t *testing.T) {
	p := &Plugin{}
	p.configuration = &configuration{
		PlaneWebhookSecret: "my-secret-key",
	}

	body := []byte(`{"event":"issue","action":"updated"}`)
	invalidSignature := "0000000000000000000000000000000000000000000000000000000000000000"

	result := p.verifyWebhookSignature(body, invalidSignature)
	assert.False(t, result, "Invalid signature should be rejected")
}

func TestVerifyWebhookSignature_NoSecret(t *testing.T) {
	p := &Plugin{}
	p.configuration = &configuration{
		PlaneWebhookSecret: "",
	}

	body := []byte(`{"event":"issue","action":"updated"}`)

	result := p.verifyWebhookSignature(body, "any-signature")
	assert.True(t, result, "Empty secret should accept all signatures (permissive mode)")
}

// computeTestHMAC generates the expected HMAC-SHA256 hex digest for test assertions.
func computeTestHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// setupWebhookTestPlugin creates a plugin configured for webhook handler tests.
// It sets up mock API, store, router, and configuration.
func setupWebhookTestPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()

	api := &plugintest.API{}

	// Logging (permissive)
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	// Ephemeral post sending
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"
	p.store = store.New(api)
	p.configuration = &configuration{
		PlaneURL:           "https://plane.example.com",
		PlaneAPIKey:        "test-api-key",
		PlaneWorkspace:     "test-workspace",
		PlaneWebhookSecret: "test-secret",
	}
	p.initRouter()

	return p, api
}

// buildWebhookIssuePayload creates a JSON body for a webhook issue event with the given parameters.
func buildWebhookIssuePayload(t *testing.T, issueID, projectID, issueName string, state WebhookStateDetail, assignees []WebhookAssignee) []byte {
	t.Helper()

	issueData := WebhookIssueData{
		ID:         issueID,
		Name:       issueName,
		State:      state,
		Assignees:  assignees,
		Priority:   "medium",
		Project:    projectID,
		SequenceID: 42,
	}
	dataBytes, _ := json.Marshal(issueData)

	event := PlaneWebhookEvent{
		Event:       "issue",
		Action:      "updated",
		WebhookID:   "wh-1",
		WorkspaceID: "ws-1",
		Data:        json.RawMessage(dataBytes),
	}
	body, _ := json.Marshal(event)
	return body
}

// buildWebhookCommentPayload creates a JSON body for a webhook comment event.
func buildWebhookCommentPayload(t *testing.T, commentID, issueID, projectID, commentHTML, actorName string) []byte {
	t.Helper()

	// Comment data includes project field for routing
	commentData := map[string]interface{}{
		"id":           commentID,
		"comment_html": commentHTML,
		"actor_detail": map[string]string{
			"id":           "actor-1",
			"display_name": actorName,
			"first_name":   "Test",
			"last_name":    "Actor",
		},
		"issue":   issueID,
		"project": projectID,
	}
	dataBytes, _ := json.Marshal(commentData)

	event := PlaneWebhookEvent{
		Event:       "issue_comment",
		Action:      "created",
		WebhookID:   "wh-1",
		WorkspaceID: "ws-1",
		Data:        json.RawMessage(dataBytes),
	}
	body, _ := json.Marshal(event)
	return body
}

// === Full webhook handler tests ===

func TestHandlePlaneWebhook_ValidSignature(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-1", "proj-1", "Test task",
		WebhookStateDetail{ID: "s1", Name: "In Progress", Group: "started"},
		nil,
	)
	signature := computeTestHMAC(body, "test-secret")

	// No bound channels for this project -- event will be silently ignored
	api.On("KVGet", "project_channels_proj-1").Return(nil, nil)
	// No plugin action key
	api.On("KVGet", "plugin_action_issue-1").Return(nil, nil)
	// Dedup: not a duplicate
	api.On("KVGet", "webhook_dedup_delivery-1").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-1", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)
	// State cache read/write
	api.On("KVGet", "work_item_state_issue-1").Return(nil, nil).Maybe()
	api.On("KVSetWithOptions", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "work_item_state_")
	}), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-1")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.Equal(t, "ok", resp["status"])
}

func TestHandlePlaneWebhook_InvalidSignature(t *testing.T) {
	p, _ := setupWebhookTestPlugin(t)

	body := []byte(`{"event":"issue","action":"updated","data":{}}`)

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", "invalid-signature")
	req.Header.Set("X-Plane-Delivery", "delivery-2")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestHandlePlaneWebhook_NoSignature_NoSecret(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)
	// Set no secret -- permissive mode
	p.configuration.PlaneWebhookSecret = ""

	body := buildWebhookIssuePayload(t, "issue-1", "proj-1", "Test task",
		WebhookStateDetail{ID: "s1", Name: "Backlog", Group: "backlog"},
		nil,
	)

	api.On("KVGet", "webhook_dedup_delivery-3").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-3", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)
	api.On("KVGet", "plugin_action_issue-1").Return(nil, nil)
	api.On("KVGet", "project_channels_proj-1").Return(nil, nil)
	api.On("KVGet", "work_item_state_issue-1").Return(nil, nil).Maybe()
	api.On("KVSetWithOptions", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "work_item_state_")
	}), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	// No signature header at all
	req.Header.Set("X-Plane-Delivery", "delivery-3")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookDedup(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-1", "proj-1", "Test task",
		WebhookStateDetail{ID: "s1", Name: "In Progress", Group: "started"},
		nil,
	)
	signature := computeTestHMAC(body, "test-secret")

	// First call -- not duplicate
	api.On("KVGet", "webhook_dedup_delivery-dup").Return([]byte("1"), nil)

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-dup")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	// Returns 200 but no post should be created
	require.Equal(t, http.StatusOK, rr.Code)
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestWebhookIssueStateChange(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-1", "proj-1", "Fix login bug",
		WebhookStateDetail{ID: "s2", Name: "In Progress", Group: "started"},
		[]WebhookAssignee{{ID: "a1", DisplayName: "Alice"}},
	)
	signature := computeTestHMAC(body, "test-secret")

	// Dedup: not duplicate
	api.On("KVGet", "webhook_dedup_delivery-state").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-state", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)

	// No plugin action (not self-notification)
	api.On("KVGet", "plugin_action_issue-1").Return(nil, nil)

	// Bound channels for this project
	channelsData, _ := json.Marshal([]string{"channel-1"})
	api.On("KVGet", "project_channels_proj-1").Return(channelsData, nil)

	// Notification config: enabled (nil = enabled by default)
	api.On("KVGet", "notify_config_channel-1").Return(nil, nil)

	// Cached state: previous state was "backlog"
	cachedState := &store.WorkItemStateCache{
		StateGroup: "backlog",
		StateName:  "Backlog",
	}
	cachedData, _ := json.Marshal(cachedState)
	api.On("KVGet", "work_item_state_issue-1").Return(cachedData, nil)

	// Assignee hash cache (no previous = first event for assignees)
	api.On("KVGet", "work_item_assignees_issue-1").Return(nil, nil)

	// Update state/assignee cache
	api.On("KVSetWithOptions", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "work_item_state_") || strings.HasPrefix(key, "work_item_assignees_")
	}), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()

	// CreatePost should be called with a state change card
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		if post.ChannelId != "channel-1" || post.UserId != "bot-user-id" {
			return false
		}
		attachments := post.Attachments()
		if len(attachments) == 0 {
			return false
		}
		att := attachments[0]
		return strings.Contains(att.Title, "Estado cambiado") &&
			strings.Contains(att.Title, "Fix login bug") &&
			att.Color == "#3f76ff"
	})).Return(&model.Post{}, nil)

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-state")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	api.AssertCalled(t, "CreatePost", mock.Anything)
}

func TestWebhookAssigneeChange(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-2", "proj-1", "Add tests",
		WebhookStateDetail{ID: "s1", Name: "In Progress", Group: "started"},
		[]WebhookAssignee{
			{ID: "a1", DisplayName: "Alice"},
			{ID: "a2", DisplayName: "Bob"},
		},
	)
	signature := computeTestHMAC(body, "test-secret")

	// Dedup
	api.On("KVGet", "webhook_dedup_delivery-assign").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-assign", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)

	// No plugin action
	api.On("KVGet", "plugin_action_issue-2").Return(nil, nil)

	// Bound channels
	channelsData, _ := json.Marshal([]string{"channel-1"})
	api.On("KVGet", "project_channels_proj-1").Return(channelsData, nil)

	// Notification config: enabled
	api.On("KVGet", "notify_config_channel-1").Return(nil, nil)

	// Cached state: same group (no state change) but we'll cache an assignee hash
	// First event for this item -- no state cache
	cachedState := &store.WorkItemStateCache{
		StateGroup: "started",
		StateName:  "In Progress",
	}
	cachedData, _ := json.Marshal(cachedState)
	api.On("KVGet", "work_item_state_issue-2").Return(cachedData, nil)

	// Assignee hash cache: different from current
	api.On("KVGet", "work_item_assignees_issue-2").Return([]byte("old-hash"), nil)

	// Cache updates
	api.On("KVSetWithOptions", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "work_item_state_") || strings.HasPrefix(key, "work_item_assignees_")
	}), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()

	// CreatePost with assignee change card
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		if post.ChannelId != "channel-1" || post.UserId != "bot-user-id" {
			return false
		}
		attachments := post.Attachments()
		if len(attachments) == 0 {
			return false
		}
		att := attachments[0]
		return strings.Contains(att.Title, "Asignacion cambiada") &&
			strings.Contains(att.Title, "Add tests") &&
			att.Color == "#3f76ff"
	})).Return(&model.Post{}, nil)

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-assign")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	api.AssertCalled(t, "CreatePost", mock.Anything)
}

func TestWebhookIssueComment(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	longComment := "<p>This is a really long comment that should be truncated. " +
		strings.Repeat("Lorem ipsum dolor sit amet. ", 10) + "</p>"
	body := buildWebhookCommentPayload(t, "comment-1", "issue-1", "proj-1", longComment, "Charlie")
	signature := computeTestHMAC(body, "test-secret")

	// Dedup
	api.On("KVGet", "webhook_dedup_delivery-comment").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-comment", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)

	// Bound channels
	channelsData, _ := json.Marshal([]string{"channel-1"})
	api.On("KVGet", "project_channels_proj-1").Return(channelsData, nil)

	// Notification config: enabled
	api.On("KVGet", "notify_config_channel-1").Return(nil, nil)

	// CreatePost with comment card
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		if post.ChannelId != "channel-1" || post.UserId != "bot-user-id" {
			return false
		}
		attachments := post.Attachments()
		if len(attachments) == 0 {
			return false
		}
		att := attachments[0]
		return strings.Contains(att.Title, "Nuevo comentario") &&
			att.Color == "#3f76ff" &&
			len(att.Text) <= 210 // truncated
	})).Return(&model.Post{}, nil)

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-comment")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	api.AssertCalled(t, "CreatePost", mock.Anything)
}

func TestWebhookUnboundProject(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-1", "proj-unbound", "Task in unbound",
		WebhookStateDetail{ID: "s1", Name: "Backlog", Group: "backlog"},
		nil,
	)
	signature := computeTestHMAC(body, "test-secret")

	// Dedup
	api.On("KVGet", "webhook_dedup_delivery-unbound").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-unbound", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)

	// No plugin action
	api.On("KVGet", "plugin_action_issue-1").Return(nil, nil)

	// No bound channels -- project is not linked to any channel
	api.On("KVGet", "project_channels_proj-unbound").Return(nil, nil)

	// State cache
	api.On("KVGet", "work_item_state_issue-1").Return(nil, nil).Maybe()
	api.On("KVSetWithOptions", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "work_item_state_")
	}), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-unbound")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	// No post created -- project is unbound
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestWebhookSelfNotificationSuppressed(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-self", "proj-1", "Plugin created task",
		WebhookStateDetail{ID: "s1", Name: "Backlog", Group: "backlog"},
		nil,
	)
	signature := computeTestHMAC(body, "test-secret")

	// Dedup
	api.On("KVGet", "webhook_dedup_delivery-self").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-self", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)

	// Plugin action key IS present -- this was a plugin-originated change
	api.On("KVGet", "plugin_action_issue-self").Return([]byte("1"), nil)

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-self")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	// No post created -- self-notification is suppressed
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestWebhookNotificationsDisabled(t *testing.T) {
	p, api := setupWebhookTestPlugin(t)

	body := buildWebhookIssuePayload(t, "issue-disabled", "proj-1", "Task in disabled channel",
		WebhookStateDetail{ID: "s2", Name: "In Progress", Group: "started"},
		nil,
	)
	signature := computeTestHMAC(body, "test-secret")

	// Dedup
	api.On("KVGet", "webhook_dedup_delivery-disabled").Return(nil, nil)
	api.On("KVSetWithOptions", "webhook_dedup_delivery-disabled", []byte("1"),
		mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil)

	// No plugin action
	api.On("KVGet", "plugin_action_issue-disabled").Return(nil, nil)

	// Bound channels
	channelsData, _ := json.Marshal([]string{"channel-disabled"})
	api.On("KVGet", "project_channels_proj-1").Return(channelsData, nil)

	// Notification config: disabled
	disabledConfig := &store.NotificationConfig{Enabled: false}
	disabledData, _ := json.Marshal(disabledConfig)
	api.On("KVGet", "notify_config_channel-disabled").Return(disabledData, nil)

	// State cache
	cachedState := &store.WorkItemStateCache{StateGroup: "backlog", StateName: "Backlog"}
	cachedData, _ := json.Marshal(cachedState)
	api.On("KVGet", "work_item_state_issue-disabled").Return(cachedData, nil)

	// Assignee hash cache
	api.On("KVGet", "work_item_assignees_issue-disabled").Return(nil, nil)

	api.On("KVSetWithOptions", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "work_item_state_") || strings.HasPrefix(key, "work_item_assignees_")
	}), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("model.PluginKVSetOptions")).Return(true, nil).Maybe()

	req := httptest.NewRequest("POST", "/api/v1/webhook/plane", bytes.NewReader(body))
	req.Header.Set("X-Plane-Signature", signature)
	req.Header.Set("X-Plane-Delivery", "delivery-disabled")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	// No post created -- notifications disabled for this channel
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

// === Unit tests for helper functions ===

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple paragraph", input: "<p>Hello world</p>", expected: "Hello world"},
		{name: "nested tags", input: "<div><p>Hello <b>world</b></p></div>", expected: "Hello world"},
		{name: "no tags", input: "plain text", expected: "plain text"},
		{name: "empty", input: "", expected: ""},
		{name: "self-closing tags", input: "Line1<br/>Line2", expected: "Line1Line2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTMLTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{name: "short text unchanged", input: "Hello", maxLen: 200, expected: "Hello"},
		{name: "exact length unchanged", input: "12345", maxLen: 5, expected: "12345"},
		{name: "long text truncated", input: strings.Repeat("a", 300), maxLen: 200, expected: strings.Repeat("a", 197) + "..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), tt.maxLen)
		})
	}
}

func TestBuildStateChangeAttachment(t *testing.T) {
	att := buildStateChangeAttachment("Fix bug", "Backlog", "In Progress", "backlog", "started", "Alice", "https://plane.example.com/ws/browse/PROJ-1")
	assert.Contains(t, att.Title, "Estado cambiado")
	assert.Contains(t, att.Title, "Fix bug")
	assert.Equal(t, "https://plane.example.com/ws/browse/PROJ-1", att.TitleLink)
	assert.Equal(t, "#3f76ff", att.Color)
	assert.Equal(t, "Plane", att.Footer)

	// Check fields
	require.GreaterOrEqual(t, len(att.Fields), 1)
	assert.Equal(t, "Cambio", att.Fields[0].Title)
	assert.Contains(t, att.Fields[0].Value, "Backlog")
	assert.Contains(t, att.Fields[0].Value, "In Progress")
}

func TestBuildStateChangeAttachmentNoOldState(t *testing.T) {
	att := buildStateChangeAttachment("Fix bug", "", "In Progress", "", "started", "Alice", "https://example.com")
	require.GreaterOrEqual(t, len(att.Fields), 1)
	value := att.Fields[0].Value.(string)
	assert.Contains(t, value, "In Progress")
	assert.NotContains(t, value, "-> ->") // Should be "-> In Progress" not "-> -> In Progress"
}

func TestBuildCommentAttachment(t *testing.T) {
	att := buildCommentAttachment("Fix bug", "This is a comment", "Alice", "https://example.com")
	assert.Contains(t, att.Title, "Nuevo comentario")
	assert.Contains(t, att.Title, "Fix bug")
	assert.Equal(t, "This is a comment", att.Text)
	assert.Equal(t, "#3f76ff", att.Color)
	assert.Equal(t, "Plane", att.Footer)
}

func TestBuildAssigneeChangeAttachment(t *testing.T) {
	assignees := []WebhookAssignee{
		{ID: "a1", DisplayName: "Alice"},
		{ID: "a2", DisplayName: "Bob"},
	}
	att := buildAssigneeChangeAttachment("Add tests", assignees, "https://example.com")
	assert.Contains(t, att.Title, "Asignacion cambiada")
	assert.Contains(t, att.Title, "Add tests")
	assert.Equal(t, "#3f76ff", att.Color)
	assert.Equal(t, "Plane", att.Footer)
	// Verify assignee names appear
	require.GreaterOrEqual(t, len(att.Fields), 1)
	assert.Contains(t, att.Fields[0].Value, "Alice")
	assert.Contains(t, att.Fields[0].Value, "Bob")
}
