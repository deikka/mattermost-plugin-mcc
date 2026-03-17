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

func TestExtractPlaneURLsSingle(t *testing.T) {
	msg := "Check https://plane.example.com/ws/browse/BACKEND-42 please"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	require.Len(t, matches, 1)
	assert.Equal(t, "BACKEND", matches[0].Identifier)
	assert.Equal(t, 42, matches[0].SequenceID)
}

func TestExtractPlaneURLsMultiple(t *testing.T) {
	msg := "See https://plane.example.com/ws/browse/PROJ-1 and also https://plane.example.com/ws/browse/PROJ-99"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	require.Len(t, matches, 2)
	assert.Equal(t, "PROJ", matches[0].Identifier)
	assert.Equal(t, 1, matches[0].SequenceID)
	assert.Equal(t, "PROJ", matches[1].Identifier)
	assert.Equal(t, 99, matches[1].SequenceID)
}

func TestExtractPlaneURLsNoMatch(t *testing.T) {
	msg := "This is a normal message with no plane URLs https://google.com"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	assert.Empty(t, matches)
}

func TestExtractPlaneURLsTrailingSlash(t *testing.T) {
	msg := "Check https://plane.example.com/ws/browse/BACKEND-1"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com/", "ws")
	require.Len(t, matches, 1)
	assert.Equal(t, "BACKEND", matches[0].Identifier)
	assert.Equal(t, 1, matches[0].SequenceID)
}

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

	attachment := buildWorkItemAttachment(item, "https://plane.example.com", "my-ws", "BACKEND", 42)

	assert.Equal(t, "Fix login page", attachment.Title)
	assert.Equal(t, "https://plane.example.com/my-ws/browse/BACKEND-42", attachment.TitleLink)
	assert.Equal(t, "#3f76ff", attachment.Color)
	assert.Equal(t, "Plane", attachment.Footer)

	require.Len(t, attachment.Fields, 4)
	assert.Equal(t, "Estado", attachment.Fields[0].Title)
	assert.Contains(t, attachment.Fields[0].Value, "In Progress")
	assert.Contains(t, attachment.Fields[0].Value, ":large_blue_circle:")
	assert.Equal(t, "Prioridad", attachment.Fields[1].Title)
	assert.Contains(t, attachment.Fields[1].Value, "High")
	assert.Equal(t, "Asignado", attachment.Fields[2].Title)
	assert.Equal(t, "Alice Smith", attachment.Fields[2].Value)
	assert.Equal(t, "Proyecto", attachment.Fields[3].Title)
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

	attachment := buildWorkItemAttachment(item, "https://plane.example.com", "my-ws", "FRONT", 1)

	require.Len(t, attachment.Fields, 3)
	assert.Equal(t, "Estado", attachment.Fields[0].Title)
	assert.Equal(t, "Prioridad", attachment.Fields[1].Title)
	assert.Equal(t, "Proyecto", attachment.Fields[2].Title)
}

func setupUnfurlTestPlugin(t *testing.T) (*Plugin, *plugintest.API, *httptest.Server) {
	t.Helper()

	planeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/work-items/"):
			// Return paginated response for sequence_id filter
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"results": []plane.WorkItem{
					{
						ID:          "def00000-0000-0000-0000-000000000456",
						Name:        "Fix login page",
						State:       "state-1",
						StateName:   "In Progress",
						StateGroup:  "started",
						Priority:    "high",
						ProjectID:   "proj-1",
						ProjectName: "Backend",
						Assignees:   []string{"assignee-1"},
						SequenceID:  42,
					},
				},
				"total_count": 1,
			}
			_ = json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/members/"):
			w.WriteHeader(http.StatusOK)
			members := []plane.MemberWrapper{
				{ID: "assignee-1", Email: "alice@example.com", DisplayName: "Alice Smith"},
			}
			_ = json.NewEncoder(w).Encode(members)
		case strings.Contains(r.URL.Path, "/projects/"):
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"results": []plane.Project{
					{ID: "proj-1", Name: "Backend", Identifier: "BACKEND"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
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
		Message:   "Check this " + ts.URL + "/test-workspace/browse/BACKEND-42",
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
		Message:   "Check this " + ts.URL + "/test-workspace/browse/BACKEND-42",
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
