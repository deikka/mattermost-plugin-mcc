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

// === Digest command tests (Plan 03-02) ===

func TestHandlePlaneDigest_Daily(t *testing.T) {
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

	// SaveDigestConfig should be called with daily, hour=9 default
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "digest_config_")
	}), mock.MatchedBy(func(data []byte) bool {
		var cfg store.DigestConfig
		_ = json.Unmarshal(data, &cfg)
		return cfg.Frequency == "daily" && cfg.Hour == 9 && cfg.UpdatedBy == "user-1"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane digest daily",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneDigest(p, nil, args, []string{"daily"})
	require.NotNil(t, resp)

	// Verify ephemeral confirmation contains the configured hour and project name
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "diario") &&
			strings.Contains(post.Message, "9:00") &&
			strings.Contains(post.Message, "Alpha")
	}))
}

func TestHandlePlaneDigest_DailyWithHour(t *testing.T) {
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

	// SaveDigestConfig with hour=14
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "digest_config_")
	}), mock.MatchedBy(func(data []byte) bool {
		var cfg store.DigestConfig
		_ = json.Unmarshal(data, &cfg)
		return cfg.Frequency == "daily" && cfg.Hour == 14 && cfg.UpdatedBy == "user-1"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane digest daily 14",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneDigest(p, nil, args, []string{"daily", "14"})
	require.NotNil(t, resp)

	// Verify confirmation mentions 14:00
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "14:00")
	}))
}

func TestHandlePlaneDigest_Weekly(t *testing.T) {
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

	// SaveDigestConfig with weekly, weekday=1 (Monday)
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "digest_config_")
	}), mock.MatchedBy(func(data []byte) bool {
		var cfg store.DigestConfig
		_ = json.Unmarshal(data, &cfg)
		return cfg.Frequency == "weekly" && cfg.Hour == 9 && cfg.Weekday == 1 && cfg.UpdatedBy == "user-1"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane digest weekly",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneDigest(p, nil, args, []string{"weekly"})
	require.NotNil(t, resp)

	// Verify confirmation mentions weekly and Monday
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "semanal") &&
			strings.Contains(post.Message, "lunes")
	}))
}

func TestHandlePlaneDigest_Off(t *testing.T) {
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

	// SaveDigestConfig with frequency="off"
	api.On("KVSet", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "digest_config_")
	}), mock.MatchedBy(func(data []byte) bool {
		var cfg store.DigestConfig
		_ = json.Unmarshal(data, &cfg)
		return cfg.Frequency == "off"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task plane digest off",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneDigest(p, nil, args, []string{"off"})
	require.NotNil(t, resp)

	// Verify confirmation
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "desactivado")
	}))
}

func TestHandlePlaneDigest_RequiresBinding(t *testing.T) {
	p, api := setupNotifyTestPlugin(t)

	// Channel is NOT bound
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil)

	args := &model.CommandArgs{
		Command:   "/task plane digest daily",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneDigest(p, nil, args, []string{"daily"})
	require.NotNil(t, resp)

	// Should get binding required message
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no esta vinculado") &&
			strings.Contains(post.Message, "/task plane link")
	}))
}

func TestHandlePlaneDigest_InvalidFrequency(t *testing.T) {
	p, api := setupNotifyTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task plane digest hourly",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp := handlePlaneDigest(p, nil, args, []string{"hourly"})
	require.NotNil(t, resp)

	// Should get usage help
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "daily|weekly|off")
	}))
}
