package main

import (
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

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// === Plan 02-02 Tests: URL Extraction ===

func TestExtractPlaneURLsSingle(t *testing.T) {
	msg := "Check https://plane.example.com/ws/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456 please"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	require.Len(t, matches, 1)
	assert.Equal(t, "abc00000-0000-0000-0000-000000000123", matches[0].ProjectID)
	assert.Equal(t, "def00000-0000-0000-0000-000000000456", matches[0].WorkItemID)
}

func TestExtractPlaneURLsMultiple(t *testing.T) {
	msg := "See https://plane.example.com/ws/projects/11111111-1111-1111-1111-111111111111/work-items/22222222-2222-2222-2222-222222222222 and also https://plane.example.com/ws/projects/33333333-3333-3333-3333-333333333333/work-items/44444444-4444-4444-4444-444444444444"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	require.Len(t, matches, 2)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", matches[0].ProjectID)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", matches[0].WorkItemID)
	assert.Equal(t, "33333333-3333-3333-3333-333333333333", matches[1].ProjectID)
	assert.Equal(t, "44444444-4444-4444-4444-444444444444", matches[1].WorkItemID)
}

func TestExtractPlaneURLsNoMatch(t *testing.T) {
	msg := "This is a normal message with no plane URLs https://google.com"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	assert.Empty(t, matches)
}

func TestExtractPlaneURLsPartialMatch(t *testing.T) {
	msg := "See https://plane.example.com/ws/projects/abc00000-0000-0000-0000-000000000123/settings"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	assert.Empty(t, matches)
}

func TestExtractPlaneURLsTrailingSlash(t *testing.T) {
	msg := "Check https://plane.example.com/ws/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com/", "ws")
	require.Len(t, matches, 1)
	assert.Equal(t, "abc00000-0000-0000-0000-000000000123", matches[0].ProjectID)
}

// === Plan 02-02 Tests: Attachment Builder ===

func TestBuildWorkItemAttachment(t *testing.T) {
	item := &plane.WorkItem{
		ID:          "wi-1",
		Name:        "Fix login page",
		StateName:   "In Progress",
		StateGroup:  "started",
		Priority:    "high",
		ProjectName: "Backend",
	}
	item.AssigneeName = "Alice Smith"

	attachment := buildWorkItemAttachment(item, "https://plane.example.com", "my-ws", "proj-1")

	assert.Equal(t, "Fix login page", attachment.Title)
	assert.Equal(t, "https://plane.example.com/my-ws/projects/proj-1/work-items/wi-1", attachment.TitleLink)
	assert.Equal(t, "#3f76ff", attachment.Color)
	assert.Equal(t, "Plane", attachment.Footer)

	require.Len(t, attachment.Fields, 4)
	assert.Equal(t, "Status", attachment.Fields[0].Title)
	assert.Contains(t, attachment.Fields[0].Value, "In Progress")
	assert.Contains(t, attachment.Fields[0].Value, ":large_blue_circle:")
	assert.Equal(t, "Priority", attachment.Fields[1].Title)
	assert.Contains(t, attachment.Fields[1].Value, "High")
	assert.Equal(t, "Assigned", attachment.Fields[2].Title)
	assert.Equal(t, "Alice Smith", attachment.Fields[2].Value)
	assert.Equal(t, "Project", attachment.Fields[3].Title)
	assert.Equal(t, "Backend", attachment.Fields[3].Value)
}

func TestBuildWorkItemAttachmentNoAssignee(t *testing.T) {
	item := &plane.WorkItem{
		ID:          "wi-1",
		Name:        "Unassigned task",
		StateName:   "Todo",
		StateGroup:  "unstarted",
		Priority:    "none",
		ProjectName: "Frontend",
	}

	attachment := buildWorkItemAttachment(item, "https://plane.example.com", "my-ws", "proj-1")

	require.Len(t, attachment.Fields, 3)
	assert.Equal(t, "Status", attachment.Fields[0].Title)
	assert.Equal(t, "Priority", attachment.Fields[1].Title)
	assert.Equal(t, "Project", attachment.Fields[2].Title)
}

// === Plan 02-02 Tests: MessageHasBeenPosted Hook ===

func setupUnfurlTestPlugin(t *testing.T) (*Plugin, *plugintest.API, *httptest.Server) {
	t.Helper()

	planeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/work-items/"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(plane.WorkItem{
				ID:          "def00000-0000-0000-0000-000000000456",
				Name:        "Fix login page",
				State:       "state-1",
				StateName:   "In Progress",
				StateGroup:  "started",
				Priority:    "high",
				ProjectID:   "abc00000-0000-0000-0000-000000000123",
				ProjectName: "Backend",
				Assignees:   []string{"assignee-1"},
			})
		case strings.Contains(r.URL.Path, "/members/"):
			w.WriteHeader(http.StatusOK)
			members := []plane.MemberWrapper{
				{Member: plane.Member{ID: "assignee-1", Email: "alice@example.com", DisplayName: "Alice Smith"}},
			}
			_ = json.NewEncoder(w).Encode(members)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(planeServer.Close)

	api := &plugintest.API{}
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"
	p.store = store.New(api)
	p.planeClient = plane.NewClient(planeServer.URL, "test-api-key", "test-workspace")
	p.configuration = &configuration{
		PlaneURL:       planeServer.URL,
		PlaneAPIKey:    "test-api-key",
		PlaneWorkspace: "test-workspace",
	}

	return p, api, planeServer
}

func TestMessageHasBeenPostedUnfurl(t *testing.T) {
	p, api, ts := setupUnfurlTestPlugin(t)

	api.On("CreatePost", mock.AnythingOfType("*model.Post")).Return(&model.Post{}, nil)

	post := &model.Post{
		Id:        "post-1",
		UserId:    "user-1",
		ChannelId: "channel-1",
		Message:   "Check this " + ts.URL + "/test-workspace/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456",
	}

	p.MessageHasBeenPosted(nil, post)

	api.AssertCalled(t, "CreatePost", mock.MatchedBy(func(replyPost *model.Post) bool {
		return replyPost.UserId == "bot-user-id" &&
			replyPost.ChannelId == "channel-1" &&
			replyPost.RootId == "post-1" &&
			len(replyPost.Attachments()) > 0 &&
			replyPost.Attachments()[0].Title == "Fix login page" &&
			strings.Contains(replyPost.Attachments()[0].Fields[0].Value.(string), "In Progress")
	}))
}

func TestMessageHasBeenPostedSkipBot(t *testing.T) {
	p, api, ts := setupUnfurlTestPlugin(t)

	post := &model.Post{
		Id:        "post-2",
		UserId:    "bot-user-id",
		ChannelId: "channel-1",
		Message:   "Check this " + ts.URL + "/test-workspace/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456",
	}

	p.MessageHasBeenPosted(nil, post)

	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestMessageHasBeenPostedNoURL(t *testing.T) {
	p, api, _ := setupUnfurlTestPlugin(t)

	post := &model.Post{
		Id:        "post-3",
		UserId:    "user-1",
		ChannelId: "channel-1",
		Message:   "Just a normal message with no Plane URLs",
	}

	p.MessageHasBeenPosted(nil, post)

	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestMessageHasBeenPostedAPIError(t *testing.T) {
	planeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "Internal server error"}`))
	}))
	defer planeServer.Close()

	api := &plugintest.API{}
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"
	p.store = store.New(api)
	p.planeClient = plane.NewClient(planeServer.URL, "test-api-key", "test-workspace")
	p.configuration = &configuration{
		PlaneURL:       planeServer.URL,
		PlaneAPIKey:    "test-api-key",
		PlaneWorkspace: "test-workspace",
	}

	post := &model.Post{
		Id:        "post-4",
		UserId:    "user-1",
		ChannelId: "channel-1",
		Message:   "Check this " + planeServer.URL + "/test-workspace/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456",
	}

	p.MessageHasBeenPosted(nil, post)

	api.AssertNotCalled(t, "CreatePost", mock.Anything)
	api.AssertCalled(t, "LogWarn", "Failed to fetch work item for unfurl",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
