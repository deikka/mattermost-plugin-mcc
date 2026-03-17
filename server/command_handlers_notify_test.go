package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// setupNotifyTestPlugin creates a Plugin with mock API and store for notification command tests.
func setupNotifyTestPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()

	api := &plugintest.API{}
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"
	p.store = store.New(api)

	return p, api
}

func TestHandlePlaneNotifications_On(t *testing.T) {
	p, api := setupNotifyTestPlugin(t)

	// Channel is bound to a project
	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	// SaveNotificationConfig should be called
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "notify_config_")
	}), mock.MatchedBy(func(data []byte) bool {
		var cfg store.NotificationConfig
		_ = json.Unmarshal(data, &cfg)
		return cfg.Enabled == true && cfg.UpdatedBy == "user-1"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane notifications on",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneNotifications(p, nil, args, []string{"on"})
	require.NotNil(t, resp)

	// Verify ephemeral confirmation
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "activadas") &&
			strings.Contains(post.Message, "Alpha")
	}))
}

func TestHandlePlaneNotifications_Off(t *testing.T) {
	p, api := setupNotifyTestPlugin(t)

	// Channel is bound
	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)

	// SaveNotificationConfig with Enabled=false
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "notify_config_")
	}), mock.MatchedBy(func(data []byte) bool {
		var cfg store.NotificationConfig
		_ = json.Unmarshal(data, &cfg)
		return cfg.Enabled == false && cfg.UpdatedBy == "user-1"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane notifications off",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneNotifications(p, nil, args, []string{"off"})
	require.NotNil(t, resp)

	// Verify ephemeral confirmation
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "desactivadas")
	}))
}

func TestHandlePlaneNotifications_RequiresBinding(t *testing.T) {
	p, api := setupNotifyTestPlugin(t)

	// Channel is NOT bound
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane notifications on",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneNotifications(p, nil, args, []string{"on"})
	require.NotNil(t, resp)

	// Should get binding required message
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no esta vinculado") &&
			strings.Contains(post.Message, "/task plane link")
	}))
}

func TestHandlePlaneNotifications_NoArgs(t *testing.T) {
	p, api := setupNotifyTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task plane notifications",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneNotifications(p, nil, args, nil)
	require.NotNil(t, resp)

	// Should get usage help
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "/task plane notifications on|off")
	}))
}

// === Skipped test stubs for Plan 03-02 ===

func TestHandlePlaneDigest_Daily(t *testing.T) {
	t.Skip("Plan 03-02: /task plane digest daily [hour] sets daily digest")
}

func TestHandlePlaneDigest_Weekly(t *testing.T) {
	t.Skip("Plan 03-02: /task plane digest weekly [hour] sets weekly digest")
}

func TestHandlePlaneDigest_Off(t *testing.T) {
	t.Skip("Plan 03-02: /task plane digest off disables digest")
}
