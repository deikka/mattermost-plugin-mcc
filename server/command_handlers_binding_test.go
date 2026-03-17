package main

import (
	"encoding/json"
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
		return strings.Contains(post.Message, "haven't linked")
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
		return strings.Contains(post.Message, "Your assigned tasks") &&
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
		return strings.Contains(post.Message, "**Project: Alpha**") &&
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
