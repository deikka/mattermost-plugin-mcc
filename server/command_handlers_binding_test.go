package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// === Task 1 Tests: Link/Unlink Commands ===

func TestPlaneLinkSuccess(t *testing.T) {
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "channel_project_")
	}), mock.AnythingOfType("[]uint8")).Return(nil)
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.ChannelId == "channel-1" &&
			post.UserId == "bot-user-id" &&
			strings.Contains(post.Message, "vinculado") &&
			strings.Contains(post.Message, "Alpha")
	})).Return(&model.Post{}, nil)

	args := &model.CommandArgs{
		Command:   "/task plane link Alpha",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "vinculado") && strings.Contains(post.Message, "Alpha")
	}))
}

func TestPlaneLinkNotConnected(t *testing.T) {
	p, api := setupCommandTestPlugin(t)
	api.On("KVGet", "user_plane_user-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane link Alpha",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no has vinculado")
	}))
}

func TestPlaneUnlinkSuccess(t *testing.T) {
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID: "proj-1", ProjectName: "Alpha", BoundBy: "user-1", BoundAt: 1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)
	api.On("KVDelete", "channel_project_channel-1").Return(nil)
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.ChannelId == "channel-1" && strings.Contains(post.Message, "desvinculado")
	})).Return(&model.Post{}, nil)

	args := &model.CommandArgs{
		Command:   "/task plane unlink",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "desvinculado") && strings.Contains(post.Message, "Alpha")
	}))
}

func TestPlaneUnlinkNoBind(t *testing.T) {
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane unlink",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no esta vinculado")
	}))
}

// === Task 2 Tests: Binding-Aware Commands ===

func TestBindingAwareCreateInline(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice Smith",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID: "proj-2", ProjectName: "Another Project", BoundBy: "user-1", BoundAt: 1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	args := &model.CommandArgs{
		Command:   "/task plane create My bound task",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Tarea creada") &&
			strings.Contains(post.Message, "My bound task") &&
			strings.Contains(post.Message, "(Proyecto: Another Project)")
	}))
}

func TestBindingAwareMine(t *testing.T) {
	items := []plane.WorkItem{
		{ID: "wi-1", Name: "Task in Alpha", StateGroup: "started", StateName: "In Progress", Priority: "high", UpdatedAt: "2026-03-17T06:00:00Z"},
	}
	p, api, _ := setupMineStatusTestPlugin(t, items)

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	mdata, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(mdata, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID: "proj-1", ProjectName: "Alpha", BoundBy: "user-1", BoundAt: 1710000000,
	}
	bdata, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bdata, nil)

	args := &model.CommandArgs{
		Command:   "/task plane mine",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Tus tareas asignadas") &&
			strings.Contains(post.Message, "(Proyecto: Alpha)")
	}))
}

func TestBindingAwareStatus(t *testing.T) {
	items := []plane.WorkItem{
		{ID: "wi-1", Name: "Task 1", StateGroup: "started"},
	}
	p, api, _ := setupMineStatusTestPlugin(t, items)

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	mdata, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(mdata, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID: "proj-1", ProjectName: "Alpha", BoundBy: "user-1", BoundAt: 1710000000,
	}
	bdata, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bdata, nil)

	args := &model.CommandArgs{
		Command:   "/task plane status",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "**Proyecto: Alpha**") &&
			strings.Contains(post.Message, "(Proyecto: Alpha)")
	}))
}

func TestDialogPreselectBoundProject(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice Smith",
	}
	mdata, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(mdata, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID: "proj-2", ProjectName: "Another Project", BoundBy: "user-1", BoundAt: 1710000000,
	}
	bdata, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bdata, nil)

	api.On("OpenInteractiveDialog", mock.MatchedBy(func(req model.OpenDialogRequest) bool {
		for _, el := range req.Dialog.Elements {
			if el.Name == "project_id" {
				return el.Default == "proj-2"
			}
		}
		return false
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
	api.AssertCalled(t, "OpenInteractiveDialog", mock.MatchedBy(func(req model.OpenDialogRequest) bool {
		for _, el := range req.Dialog.Elements {
			if el.Name == "project_id" {
				return el.Default == "proj-2"
			}
		}
		return false
	}))
}

func TestUnboundChannelNoChange(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice Smith",
	}
	mdata, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(mdata, nil)
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane create My unbound task",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}
	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		msg := post.Message
		return strings.Contains(msg, "Tarea creada") &&
			strings.Contains(msg, "My unbound task") &&
			strings.Contains(msg, "My Project") &&
			!strings.Contains(msg, "(Proyecto:")
	}))
}

// === Context Menu Tests ===

func TestContextMenuAction(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	// Initialize router for HTTP handler
	p.initRouter()

	// Mock: user is connected
	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice Smith",
	}
	mdata, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(mdata, nil)

	// Mock: no channel binding
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	// Mock: GetPost returns a message
	siteURL := "https://mattermost.example.com"
	api.On("GetPost", "post-123").Return(&model.Post{
		Id:        "post-123",
		ChannelId: "channel-1",
		Message:   "We need to fix the login flow because users are getting stuck on the redirect page after OAuth",
	}, nil)

	// Mock: GetConfig for permalink
	api.On("GetConfig").Return(&model.Config{
		ServiceSettings: model.ServiceSettings{
			SiteURL: &siteURL,
		},
	})

	// Mock: GetChannel for permalink
	api.On("GetChannel", "channel-1").Return(&model.Channel{
		Id:     "channel-1",
		TeamId: "team-1",
	}, nil)

	// Mock: GetTeam for permalink
	api.On("GetTeam", "team-1").Return(&model.Team{
		Id:   "team-1",
		Name: "myteam",
	}, nil)

	// Build and send request
	body, _ := json.Marshal(map[string]string{"post_id": "post-123"})
	req := httptest.NewRequest("POST", "/api/v1/action/create-task-from-message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mattermost-User-Id", "user-1")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var dialogConfig map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &dialogConfig)
	require.NoError(t, err)

	// Verify callback URL includes source_post_id
	url, ok := dialogConfig["url"].(string)
	require.True(t, ok)
	require.Contains(t, url, "source_post_id=post-123")
	require.Contains(t, url, "/dialog/create-task")

	// Verify dialog structure
	dialog, ok := dialogConfig["dialog"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "Crear Tarea en Plane", dialog["title"])
	require.Equal(t, "create_task_from_message", dialog["callback_id"])

	// Verify elements have pre-populated title and description
	elements, ok := dialog["elements"].([]interface{})
	require.True(t, ok)
	require.GreaterOrEqual(t, len(elements), 2)

	// Title element
	titleEl := elements[0].(map[string]interface{})
	require.Equal(t, "title", titleEl["name"])
	titleDefault := titleEl["default"].(string)
	require.Contains(t, titleDefault, "We need to fix the login flow")
	require.LessOrEqual(t, len(titleDefault), 80)

	// Description element
	descEl := elements[1].(map[string]interface{})
	require.Equal(t, "description", descEl["name"])
	descDefault := descEl["default"].(string)
	require.Contains(t, descDefault, "We need to fix the login flow because users are getting stuck")
	require.Contains(t, descDefault, "[Original message]")
	require.Contains(t, descDefault, "myteam/pl/post-123")
}

func TestContextMenuActionBoundChannel(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	p.initRouter()

	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice Smith",
	}
	mdata, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(mdata, nil)

	// Mock: channel bound to "Another Project" (proj-2)
	binding := &store.ChannelProjectBinding{
		ProjectID: "proj-2", ProjectName: "Another Project", BoundBy: "user-1", BoundAt: 1710000000,
	}
	bdata, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bdata, nil)

	api.On("GetPost", "post-456").Return(&model.Post{
		Id:        "post-456",
		ChannelId: "channel-1",
		Message:   "Short message",
	}, nil)

	siteURL := "https://mm.example.com"
	api.On("GetConfig").Return(&model.Config{
		ServiceSettings: model.ServiceSettings{SiteURL: &siteURL},
	})
	api.On("GetChannel", "channel-1").Return(&model.Channel{Id: "channel-1", TeamId: "team-1"}, nil)
	api.On("GetTeam", "team-1").Return(&model.Team{Id: "team-1", Name: "myteam"}, nil)

	body, _ := json.Marshal(map[string]string{"post_id": "post-456"})
	req := httptest.NewRequest("POST", "/api/v1/action/create-task-from-message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mattermost-User-Id", "user-1")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var dialogConfig map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &dialogConfig)
	require.NoError(t, err)

	dialog := dialogConfig["dialog"].(map[string]interface{})
	elements := dialog["elements"].([]interface{})

	// Project element should default to bound project
	projectEl := elements[2].(map[string]interface{})
	require.Equal(t, "project_id", projectEl["name"])
	require.Equal(t, "proj-2", projectEl["default"])
}

func TestContextMenuActionNotConnected(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	p.initRouter()

	// Mock: user NOT connected
	api.On("KVGet", "user_plane_user-1").Return(nil, nil)

	body, _ := json.Marshal(map[string]string{"post_id": "post-123"})
	req := httptest.NewRequest("POST", "/api/v1/action/create-task-from-message", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mattermost-User-Id", "user-1")
	rr := httptest.NewRecorder()

	p.router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusForbidden, rr.Code)

	var errResp map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &errResp)
	require.Contains(t, errResp["error"], "/task connect")
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		maxLen   int
		expected string
	}{
		{
			name:     "short text unchanged",
			text:     "Fix bug",
			maxLen:   80,
			expected: "Fix bug",
		},
		{
			name:     "long text truncated with ellipsis",
			text:     "This is a very long message that exceeds the maximum length for a task title in the system",
			maxLen:   80,
			expected: "This is a very long message that exceeds the maximum length for a task title ...",
		},
		{
			name:     "newlines replaced with spaces",
			text:     "Line one\nLine two\nLine three",
			maxLen:   80,
			expected: "Line one Line two Line three",
		},
		{
			name:     "exact length unchanged",
			text:     "12345678",
			maxLen:   8,
			expected: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateTitle(tt.text, tt.maxLen)
			require.Equal(t, tt.expected, result)
			require.LessOrEqual(t, len(result), tt.maxLen)
		})
	}
}
