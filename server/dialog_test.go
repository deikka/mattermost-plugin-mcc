package main

import (
	"bytes"
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

// newPlaneCreateTestPlugin creates a plugin with a mock Plane server that handles
// projects, members, labels, and work-item creation endpoints.
func newPlaneCreateTestPlugin(t *testing.T) (*Plugin, *plugintest.API, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/members/"):
			// Project members
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"results": []plane.MemberWrapper{
					{ID: "mw1", Member: plane.Member{ID: "plane-u1", Email: "alice@example.com", DisplayName: "Alice Smith"}},
					{ID: "mw2", Member: plane.Member{ID: "plane-u2", Email: "bob@example.com", DisplayName: "Bob Jones"}},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/labels/"):
			// Project labels
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"results": []plane.Label{
					{ID: "label-1", Name: "Bug", Color: "#ff0000"},
					{ID: "label-2", Name: "Feature", Color: "#00ff00"},
					{ID: "label-3", Name: "Frontend", Color: "#0000ff"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/states/"):
			// Project states
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"results": []plane.State{
					{ID: "state-1", Name: "Backlog", Group: "backlog"},
					{ID: "state-2", Name: "Todo", Group: "unstarted"},
					{ID: "state-3", Name: "In Progress", Group: "started"},
					{ID: "state-4", Name: "Done", Group: "completed"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/work-items/") && r.Method == "POST":
			// Create work item
			w.WriteHeader(http.StatusCreated)
			var req plane.CreateWorkItemRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			item := plane.WorkItem{
				ID:        "wi-123",
				Name:      req.Name,
				ProjectID: "proj-1",
			}
			_ = json.NewEncoder(w).Encode(item)

		case strings.Contains(r.URL.Path, "/projects/"):
			// List projects
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"results": []plane.Project{
					{ID: "proj-1", Name: "My Project", Identifier: "MP"},
					{ID: "proj-2", Name: "Another Project", Identifier: "AP"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

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
	p.planeClient = plane.NewClient(server.URL, "test-api-key", "test-workspace")

	return p, api, server
}

func TestCreateTask(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	// Mock: user is connected to Plane
	mapping := &store.PlaneUserMapping{
		PlaneUserID:      "plane-u1",
		PlaneEmail:       "alice@example.com",
		PlaneDisplayName: "Alice Smith",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Mock: dialog opens successfully
	api.On("OpenInteractiveDialog", mock.MatchedBy(func(req model.OpenDialogRequest) bool {
		d := req.Dialog
		// Verify dialog has all 6 fields
		if len(d.Elements) != 6 {
			return false
		}
		// title, description, project_id, priority, assignee_id, labels
		names := make(map[string]bool)
		for _, e := range d.Elements {
			names[e.Name] = true
		}
		return d.Title == "Create Task in Plane" &&
			d.CallbackId == "create_task" &&
			names["title"] && names["description"] && names["project_id"] &&
			names["priority"] && names["assignee_id"] && names["labels"]
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane create",
		UserId:    "user-1",
		ChannelId: "channel-1",
		TriggerId: "trigger-123",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "OpenInteractiveDialog", mock.AnythingOfType("model.OpenDialogRequest"))
}

func TestCreateTaskInlineMode(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	// Mock: user is connected
	mapping := &store.PlaneUserMapping{
		PlaneUserID:      "plane-u1",
		PlaneEmail:       "alice@example.com",
		PlaneDisplayName: "Alice Smith",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	args := &model.CommandArgs{
		Command:   "/task plane create Fix the login bug",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Verify confirmation message was sent
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Tarea creada") &&
			strings.Contains(post.Message, "Fix the login bug") &&
			strings.Contains(post.Message, "My Project") &&
			strings.Contains(post.Message, "Ver en Plane")
	}))
}

func TestCreateTaskConfirmation(t *testing.T) {
	// Verify the exact format of the confirmation message
	msg := formatTaskCreatedMessage("Fix bug", "My Project", "https://plane.example.com/ws/projects/p1/work-items/wi1")
	assert.Contains(t, msg, ":white_check_mark:")
	assert.Contains(t, msg, "Tarea creada")
	assert.Contains(t, msg, "**Fix bug**")
	assert.Contains(t, msg, "My Project")
	assert.Contains(t, msg, "[Ver en Plane]")
	assert.Contains(t, msg, "https://plane.example.com/ws/projects/p1/work-items/wi1")
}

func TestCreateTaskDialogSubmission(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	// Setup router for HTTP handler
	p.initRouter()

	submission := model.SubmitDialogRequest{
		UserId:    "user-1",
		ChannelId: "channel-1",
		Submission: map[string]interface{}{
			"title":       "Dialog task title",
			"description": "Some description",
			"project_id":  "proj-1",
			"priority":    "high",
			"assignee_id": "plane-u1",
			"labels":      "",
		},
	}

	body, _ := json.Marshal(submission)
	req := httptest.NewRequest("POST", "/api/v1/dialog/create-task", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify ephemeral confirmation was sent
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Tarea creada") &&
			strings.Contains(post.Message, "Dialog task title") &&
			strings.Contains(post.Message, "Ver en Plane")
	}))
}

func TestCreateTaskLabelResolution(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	p.initRouter()

	submission := model.SubmitDialogRequest{
		UserId:    "user-1",
		ChannelId: "channel-1",
		Submission: map[string]interface{}{
			"title":       "Task with labels",
			"description": "",
			"project_id":  "proj-1",
			"priority":    "none",
			"assignee_id": "plane-u1",
			"labels":      "Bug, frontend, NonExistent",
		},
	}

	body, _ := json.Marshal(submission)
	req := httptest.NewRequest("POST", "/api/v1/dialog/create-task", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify task was created successfully (label resolution happens silently)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Tarea creada") &&
			strings.Contains(post.Message, "Task with labels")
	}))

	// Verify warning was logged for unmatched label
	api.AssertCalled(t, "LogWarn", "Label not found, skipping", "label", "NonExistent", "project", "proj-1")
}
