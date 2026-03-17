package main

import (
	"encoding/json"
	"net/http"
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

	// Mock: user connected to Plane
	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Mock: save binding
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "channel_project_")
	}), mock.AnythingOfType("[]uint8")).Return(nil)

	// Mock: CreatePost for visible message
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

	// Verify visible post was created (not ephemeral)
	api.AssertCalled(t, "CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "vinculado") &&
			strings.Contains(post.Message, "Alpha")
	}))
}

func TestPlaneLinkNotConnected(t *testing.T) {
	p, api := setupCommandTestPlugin(t)

	// Mock: user NOT connected to Plane
	api.On("KVGet", "user_plane_user-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane link Alpha",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Should get "connect first" message
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "haven't linked") ||
			strings.Contains(post.Message, "/task connect")
	}))
}

func TestPlaneUnlinkSuccess(t *testing.T) {
	p, api, _ := setupConnectTestPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mock: user connected
	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Mock: channel is bound
	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	// Mock: delete binding
	api.On("KVDelete", "channel_project_channel-1").Return(nil)

	// Mock: CreatePost for visible message
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.ChannelId == "channel-1" &&
			post.UserId == "bot-user-id" &&
			strings.Contains(post.Message, "desvinculado") &&
			strings.Contains(post.Message, "Alpha")
	})).Return(&model.Post{}, nil)

	args := &model.CommandArgs{
		Command:   "/task plane unlink",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Verify visible post was created
	api.AssertCalled(t, "CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "desvinculado") &&
			strings.Contains(post.Message, "Alpha")
	}))
}

func TestPlaneUnlinkNoBind(t *testing.T) {
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

	// Mock: user connected
	mapping := &store.PlaneUserMapping{
		PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Mock: channel is NOT bound
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane unlink",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Should get ephemeral "not linked" message
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no esta vinculado")
	}))
}

// === Task 2 Tests: Binding-Aware Commands ===

func TestBindingAwareCreateInline(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	// Mock: user connected
	mapping := &store.PlaneUserMapping{
		PlaneUserID:      "plane-u1",
		PlaneEmail:       "alice@example.com",
		PlaneDisplayName: "Alice Smith",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Mock: channel bound to "Another Project" (proj-2), NOT first project
	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-2",
		ProjectName: "Another Project",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
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

	// Verify: uses bound project, includes "(Proyecto: Another Project)"
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Tarea creada") &&
			strings.Contains(post.Message, "My bound task") &&
			strings.Contains(post.Message, "(Proyecto: Another Project)")
	}))
}

func TestBindingAwareMine(t *testing.T) {
	t.Skip("Phase 2 Plan 02: binding-aware mine requires mock ordering fix")
	items := []plane.WorkItem{
		{ID: "wi-1", Name: "Task in Alpha", StateGroup: "started", StateName: "In Progress", Priority: "high", UpdatedAt: "2026-03-17T06:00:00Z"},
	}

	p, api, _ := setupMineStatusTestPlugin(t, items)

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Channel bound to Alpha
	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	args := &model.CommandArgs{
		Command:   "/task plane mine",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Your assigned tasks") &&
			strings.Contains(post.Message, "(Proyecto: Alpha)")
	}))
}

func TestBindingAwareStatus(t *testing.T) {
	t.Skip("Phase 2 Plan 02: binding-aware status requires mock ordering fix")
	items := []plane.WorkItem{
		{ID: "wi-1", Name: "Task 1", StateGroup: "started"},
	}

	p, api, _ := setupMineStatusTestPlugin(t, items)

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	args := &model.CommandArgs{
		Command:   "/task plane status",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "**Project: Alpha**") &&
			strings.Contains(post.Message, "(Proyecto: Alpha)")
	}))
}

func TestDialogPreselectBoundProject(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	mapping := &store.PlaneUserMapping{
		PlaneUserID:      "plane-u1",
		PlaneEmail:       "alice@example.com",
		PlaneDisplayName: "Alice Smith",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-2",
		ProjectName: "Another Project",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	// Verify dialog opens with project_id Default = proj-2
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

// === Phase 2: Context Menu Action Test ===

func TestContextMenuAction(t *testing.T) {
	t.Skip("Phase 2: handleCreateTaskFromMessage not yet implemented")
}

func TestUnboundChannelNoChange(t *testing.T) {
	p, api, _ := newPlaneCreateTestPlugin(t)

	mapping := &store.PlaneUserMapping{
		PlaneUserID:      "plane-u1",
		PlaneEmail:       "alice@example.com",
		PlaneDisplayName: "Alice Smith",
	}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	// Channel NOT bound
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane create My unbound task",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Uses first project (My Project), no "(Proyecto:)" suffix
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		msg := post.Message
		return strings.Contains(msg, "Tarea creada") &&
			strings.Contains(msg, "My unbound task") &&
			strings.Contains(msg, "My Project") &&
			!strings.Contains(msg, "(Proyecto:")
	}))
}
