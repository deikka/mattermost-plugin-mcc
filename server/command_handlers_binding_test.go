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
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

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
