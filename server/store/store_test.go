package store

import (
	"encoding/json"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*Store, *plugintest.API) {
	t.Helper()
	api := &plugintest.API{}
	t.Cleanup(func() {
		api.AssertExpectations(t)
	})
	s := New(api)
	return s, api
}

func TestKVStoreGetPlaneUser(t *testing.T) {
	t.Run("user exists", func(t *testing.T) {
		s, api := setupTestStore(t)

		mapping := &PlaneUserMapping{
			PlaneUserID:      "plane-user-1",
			PlaneEmail:       "alice@example.com",
			PlaneDisplayName: "Alice",
			ConnectedAt:      1234567890,
		}
		data, _ := json.Marshal(mapping)

		api.On("KVGet", "user_plane_mm-user-1").Return(data, nil)

		result, err := s.GetPlaneUser("mm-user-1")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "plane-user-1", result.PlaneUserID)
		assert.Equal(t, "alice@example.com", result.PlaneEmail)
		assert.Equal(t, "Alice", result.PlaneDisplayName)
	})

	t.Run("user not found", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVGet", "user_plane_mm-user-2").Return(nil, nil)

		result, err := s.GetPlaneUser("mm-user-2")
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("kv error", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVGet", "user_plane_mm-user-3").
			Return(nil, &model.AppError{Message: "kv store error"})

		result, err := s.GetPlaneUser("mm-user-3")
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "KVGet failed")
	})
}

func TestKVStoreSavePlaneUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, api := setupTestStore(t)

		mapping := &PlaneUserMapping{
			PlaneUserID:      "plane-user-1",
			PlaneEmail:       "alice@example.com",
			PlaneDisplayName: "Alice",
			ConnectedAt:      1234567890,
		}

		api.On("KVSet", "user_plane_mm-user-1", mock.AnythingOfType("[]uint8")).Return(nil)

		err := s.SavePlaneUser("mm-user-1", mapping)
		require.NoError(t, err)

		// Verify the data that was saved
		api.AssertCalled(t, "KVSet", "user_plane_mm-user-1", mock.MatchedBy(func(data []byte) bool {
			var saved PlaneUserMapping
			_ = json.Unmarshal(data, &saved)
			return saved.PlaneUserID == "plane-user-1" && saved.PlaneEmail == "alice@example.com"
		}))
	})

	t.Run("kv error", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVSet", mock.Anything, mock.Anything).
			Return(&model.AppError{Message: "write error"})

		err := s.SavePlaneUser("mm-user-1", &PlaneUserMapping{PlaneUserID: "p1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "KVSet failed")
	})
}

func TestKVStoreDeletePlaneUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVDelete", "user_plane_mm-user-1").Return(nil)

		err := s.DeletePlaneUser("mm-user-1")
		require.NoError(t, err)
	})

	t.Run("kv error", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVDelete", "user_plane_mm-user-1").
			Return(&model.AppError{Message: "delete error"})

		err := s.DeletePlaneUser("mm-user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "KVDelete failed")
	})
}

func TestKVStoreGetObsidianConfig(t *testing.T) {
	t.Run("config exists", func(t *testing.T) {
		s, api := setupTestStore(t)

		cfg := &ObsidianConfig{
			Host:    "127.0.0.1",
			Port:    27124,
			APIKey:  "obs-key-123",
			SetupAt: 1234567890,
		}
		data, _ := json.Marshal(cfg)

		api.On("KVGet", "user_obsidian_mm-user-1").Return(data, nil)

		result, err := s.GetObsidianConfig("mm-user-1")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "127.0.0.1", result.Host)
		assert.Equal(t, 27124, result.Port)
		assert.Equal(t, "obs-key-123", result.APIKey)
	})

	t.Run("config not found", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVGet", "user_obsidian_mm-user-2").Return(nil, nil)

		result, err := s.GetObsidianConfig("mm-user-2")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestKVStoreSaveObsidianConfig(t *testing.T) {
	s, api := setupTestStore(t)

	cfg := &ObsidianConfig{
		Host:    "192.168.1.10",
		Port:    27125,
		APIKey:  "my-key",
		SetupAt: 9999999999,
	}

	api.On("KVSet", "user_obsidian_mm-user-1", mock.AnythingOfType("[]uint8")).Return(nil)

	err := s.SaveObsidianConfig("mm-user-1", cfg)
	require.NoError(t, err)

	api.AssertCalled(t, "KVSet", "user_obsidian_mm-user-1", mock.MatchedBy(func(data []byte) bool {
		var saved ObsidianConfig
		_ = json.Unmarshal(data, &saved)
		return saved.Host == "192.168.1.10" && saved.Port == 27125
	}))
}

// === Phase 2: ChannelProjectBinding CRUD Tests ===

func TestChannelBindingSaveAndGet(t *testing.T) {
	s, api := setupTestStore(t)

	binding := &ChannelProjectBinding{
		ProjectID:   "proj-uuid-001",
		ProjectName: "Backend",
		BoundBy:     "mm-user-1",
		BoundAt:     1710000000,
	}
	data, _ := json.Marshal(binding)

	// SaveChannelBinding: KVSet forward key + AddProjectChannel (KVGet reverse + KVSet reverse)
	api.On("KVSet", "channel_project_channel-1", mock.AnythingOfType("[]uint8")).Return(nil)
	api.On("KVGet", "project_channels_proj-uuid-001").Return(nil, nil)
	api.On("KVSet", "project_channels_proj-uuid-001", mock.AnythingOfType("[]uint8")).Return(nil)

	// GetChannelBinding: KVGet forward key
	api.On("KVGet", "channel_project_channel-1").Return(data, nil)

	err := s.SaveChannelBinding("channel-1", binding)
	require.NoError(t, err)

	result, err := s.GetChannelBinding("channel-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "proj-uuid-001", result.ProjectID)
	assert.Equal(t, "Backend", result.ProjectName)
	assert.Equal(t, "mm-user-1", result.BoundBy)
	assert.Equal(t, int64(1710000000), result.BoundAt)
}

func TestChannelBindingGetNotFound(t *testing.T) {
	s, api := setupTestStore(t)

	api.On("KVGet", "channel_project_channel-nonexistent").Return(nil, nil)

	result, err := s.GetChannelBinding("channel-nonexistent")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestChannelBindingDelete(t *testing.T) {
	s, api := setupTestStore(t)

	binding := &ChannelProjectBinding{
		ProjectID:   "proj-uuid-001",
		ProjectName: "Backend",
		BoundBy:     "mm-user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)

	// SaveChannelBinding mocks: forward KVSet + reverse index (KVGet + KVSet)
	api.On("KVSet", "channel_project_channel-1", mock.AnythingOfType("[]uint8")).Return(nil)
	api.On("KVGet", "project_channels_proj-uuid-001").Return(nil, nil).Once()
	api.On("KVSet", "project_channels_proj-uuid-001", mock.AnythingOfType("[]uint8")).Return(nil)

	// DeleteChannelBinding mocks: GetChannelBinding (KVGet) + KVDelete + RemoveProjectChannel
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)
	api.On("KVDelete", "channel_project_channel-1").Return(nil)
	// RemoveProjectChannel: KVGet reverse (now has ["channel-1"]) + KVDelete (becomes empty)
	channelsData, _ := json.Marshal([]string{"channel-1"})
	api.On("KVGet", "project_channels_proj-uuid-001").Return(channelsData, nil)
	api.On("KVDelete", "project_channels_proj-uuid-001").Return(nil)

	// Save the binding
	err := s.SaveChannelBinding("channel-1", binding)
	require.NoError(t, err)

	// Delete it
	err = s.DeleteChannelBinding("channel-1")
	require.NoError(t, err)

	api.AssertCalled(t, "KVDelete", "channel_project_channel-1")
}

func TestChannelBindingOverwrite(t *testing.T) {
	s, api := setupTestStore(t)

	binding1 := &ChannelProjectBinding{
		ProjectID:   "proj-uuid-001",
		ProjectName: "Backend",
		BoundBy:     "mm-user-1",
		BoundAt:     1710000000,
	}
	binding2 := &ChannelProjectBinding{
		ProjectID:   "proj-uuid-002",
		ProjectName: "Frontend",
		BoundBy:     "mm-user-2",
		BoundAt:     1710001000,
	}
	data2, _ := json.Marshal(binding2)

	// First SaveChannelBinding: forward KVSet + reverse index for proj-001
	api.On("KVSet", "channel_project_channel-1", mock.AnythingOfType("[]uint8")).Return(nil)
	api.On("KVGet", "project_channels_proj-uuid-001").Return(nil, nil)
	api.On("KVSet", "project_channels_proj-uuid-001", mock.AnythingOfType("[]uint8")).Return(nil)

	// Second SaveChannelBinding: forward KVSet (already mocked) + reverse index for proj-002
	api.On("KVGet", "project_channels_proj-uuid-002").Return(nil, nil)
	api.On("KVSet", "project_channels_proj-uuid-002", mock.AnythingOfType("[]uint8")).Return(nil)

	// GetChannelBinding: KVGet forward key returns binding2
	api.On("KVGet", "channel_project_channel-1").Return(data2, nil)

	// Save first binding
	err := s.SaveChannelBinding("channel-1", binding1)
	require.NoError(t, err)

	// Overwrite with second binding
	err = s.SaveChannelBinding("channel-1", binding2)
	require.NoError(t, err)

	// Get should return the latest binding
	result, err := s.GetChannelBinding("channel-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "proj-uuid-002", result.ProjectID)
	assert.Equal(t, "Frontend", result.ProjectName)
	assert.Equal(t, "mm-user-2", result.BoundBy)
}

func TestKVStoreIsPlaneConnected(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		s, api := setupTestStore(t)

		mapping := &PlaneUserMapping{PlaneUserID: "p1"}
		data, _ := json.Marshal(mapping)
		api.On("KVGet", "user_plane_mm-user-1").Return(data, nil)

		connected, err := s.IsPlaneConnected("mm-user-1")
		require.NoError(t, err)
		assert.True(t, connected)
	})

	t.Run("not connected", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVGet", "user_plane_mm-user-2").Return(nil, nil)

		connected, err := s.IsPlaneConnected("mm-user-2")
		require.NoError(t, err)
		assert.False(t, connected)
	})
}

// === Phase 3: Reverse Index and New Config Tests ===

func TestGetProjectChannels_Empty(t *testing.T) {
	s, api := setupTestStore(t)

	api.On("KVGet", "project_channels_proj-nonexistent").Return(nil, nil)

	result, err := s.GetProjectChannels("proj-nonexistent")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestSaveAndGetProjectChannels(t *testing.T) {
	s, api := setupTestStore(t)

	channels := []string{"channel-1", "channel-2", "channel-3"}
	data, _ := json.Marshal(channels)

	api.On("KVSet", "project_channels_proj-001", mock.AnythingOfType("[]uint8")).Return(nil)
	api.On("KVGet", "project_channels_proj-001").Return(data, nil)

	err := s.SaveProjectChannels("proj-001", channels)
	require.NoError(t, err)

	result, err := s.GetProjectChannels("proj-001")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, channels, result)
}

func TestAddProjectChannel_NoDuplicates(t *testing.T) {
	s, api := setupTestStore(t)

	// First add: empty list, adds channel-1
	api.On("KVGet", "project_channels_proj-001").Return(nil, nil).Once()
	api.On("KVSet", "project_channels_proj-001", mock.AnythingOfType("[]uint8")).Return(nil).Once()

	err := s.AddProjectChannel("proj-001", "channel-1")
	require.NoError(t, err)

	// Second add: list already has channel-1, should not add again
	existingData, _ := json.Marshal([]string{"channel-1"})
	api.On("KVGet", "project_channels_proj-001").Return(existingData, nil)

	err = s.AddProjectChannel("proj-001", "channel-1")
	require.NoError(t, err)

	// KVSet should have been called exactly once (first add only)
	api.AssertNumberOfCalls(t, "KVSet", 1)
}

func TestRemoveProjectChannel(t *testing.T) {
	t.Run("removes channel and keeps others", func(t *testing.T) {
		s, api := setupTestStore(t)

		existingData, _ := json.Marshal([]string{"channel-1", "channel-2"})
		api.On("KVGet", "project_channels_proj-001").Return(existingData, nil)
		api.On("KVSet", "project_channels_proj-001", mock.AnythingOfType("[]uint8")).Return(nil)

		err := s.RemoveProjectChannel("proj-001", "channel-1")
		require.NoError(t, err)

		// Verify only channel-2 remains
		api.AssertCalled(t, "KVSet", "project_channels_proj-001", mock.MatchedBy(func(data []byte) bool {
			var saved []string
			_ = json.Unmarshal(data, &saved)
			return len(saved) == 1 && saved[0] == "channel-2"
		}))
	})

	t.Run("deletes key when last channel removed", func(t *testing.T) {
		s, api := setupTestStore(t)

		existingData, _ := json.Marshal([]string{"channel-1"})
		api.On("KVGet", "project_channels_proj-001").Return(existingData, nil)
		api.On("KVDelete", "project_channels_proj-001").Return(nil)

		err := s.RemoveProjectChannel("proj-001", "channel-1")
		require.NoError(t, err)

		api.AssertCalled(t, "KVDelete", "project_channels_proj-001")
	})
}

func TestSaveChannelBindingUpdatesReverseIndex(t *testing.T) {
	s, api := setupTestStore(t)

	binding := &ChannelProjectBinding{
		ProjectID:   "proj-001",
		ProjectName: "Backend",
		BoundBy:     "mm-user-1",
		BoundAt:     1710000000,
	}

	// SaveChannelBinding: forward KVSet + reverse index (KVGet + KVSet)
	api.On("KVSet", "channel_project_channel-1", mock.AnythingOfType("[]uint8")).Return(nil)
	api.On("KVGet", "project_channels_proj-001").Return(nil, nil)
	api.On("KVSet", "project_channels_proj-001", mock.AnythingOfType("[]uint8")).Return(nil)

	err := s.SaveChannelBinding("channel-1", binding)
	require.NoError(t, err)

	// Verify reverse index was updated with the channel
	api.AssertCalled(t, "KVSet", "project_channels_proj-001", mock.MatchedBy(func(data []byte) bool {
		var saved []string
		_ = json.Unmarshal(data, &saved)
		return len(saved) == 1 && saved[0] == "channel-1"
	}))
}

func TestDeleteChannelBindingUpdatesReverseIndex(t *testing.T) {
	s, api := setupTestStore(t)

	binding := &ChannelProjectBinding{
		ProjectID:   "proj-001",
		ProjectName: "Backend",
		BoundBy:     "mm-user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)

	// DeleteChannelBinding: GetChannelBinding (KVGet forward) + KVDelete forward + RemoveProjectChannel
	api.On("KVGet", "channel_project_channel-1").Return(bindingData, nil)
	api.On("KVDelete", "channel_project_channel-1").Return(nil)

	// RemoveProjectChannel: KVGet reverse (has channel-1) + KVDelete reverse (becomes empty)
	channelsData, _ := json.Marshal([]string{"channel-1"})
	api.On("KVGet", "project_channels_proj-001").Return(channelsData, nil)
	api.On("KVDelete", "project_channels_proj-001").Return(nil)

	err := s.DeleteChannelBinding("channel-1")
	require.NoError(t, err)

	api.AssertCalled(t, "KVDelete", "project_channels_proj-001")
}

func TestRebuildReverseIndex(t *testing.T) {
	t.Run("scans channel_project_ keys and populates project_channels_", func(t *testing.T) {
		s, api := setupTestStore(t)

		binding1 := &ChannelProjectBinding{
			ProjectID:   "proj-001",
			ProjectName: "Backend",
			BoundBy:     "mm-user-1",
			BoundAt:     1710000000,
		}
		binding1Data, _ := json.Marshal(binding1)

		binding2 := &ChannelProjectBinding{
			ProjectID:   "proj-001",
			ProjectName: "Backend",
			BoundBy:     "mm-user-2",
			BoundAt:     1710000001,
		}
		binding2Data, _ := json.Marshal(binding2)

		binding3 := &ChannelProjectBinding{
			ProjectID:   "proj-002",
			ProjectName: "Frontend",
			BoundBy:     "mm-user-1",
			BoundAt:     1710000002,
		}
		binding3Data, _ := json.Marshal(binding3)

		// KVList returns keys including channel_project_ prefixed ones and other keys
		api.On("KVList", 0, 100).Return([]string{
			"channel_project_channel-1",
			"channel_project_channel-2",
			"channel_project_channel-3",
			"user_plane_mm-user-1",
			"notify_config_channel-1",
		}, nil)

		// GetChannelBinding reads for each channel_project_ key
		api.On("KVGet", "channel_project_channel-1").Return(binding1Data, nil)
		api.On("KVGet", "channel_project_channel-2").Return(binding2Data, nil)
		api.On("KVGet", "channel_project_channel-3").Return(binding3Data, nil)

		// AddProjectChannel: for proj-001 (channel-1, then channel-2) and proj-002 (channel-3)
		// First call for proj-001: empty list -> adds channel-1
		api.On("KVGet", "project_channels_proj-001").Return(nil, nil).Once()
		api.On("KVSet", "project_channels_proj-001", mock.AnythingOfType("[]uint8")).Return(nil).Once()

		// Second call for proj-001: already has channel-1 -> adds channel-2
		existingCh1, _ := json.Marshal([]string{"channel-1"})
		api.On("KVGet", "project_channels_proj-001").Return(existingCh1, nil).Once()
		api.On("KVSet", "project_channels_proj-001", mock.AnythingOfType("[]uint8")).Return(nil).Once()

		// First call for proj-002: empty list -> adds channel-3
		api.On("KVGet", "project_channels_proj-002").Return(nil, nil).Once()
		api.On("KVSet", "project_channels_proj-002", mock.AnythingOfType("[]uint8")).Return(nil).Once()

		count, err := s.RebuildReverseIndex()
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Verify proj-001 reverse index was set with both channels
		api.AssertCalled(t, "KVSet", "project_channels_proj-001", mock.MatchedBy(func(data []byte) bool {
			var saved []string
			_ = json.Unmarshal(data, &saved)
			return len(saved) == 2
		}))

		// Verify proj-002 reverse index was set
		api.AssertCalled(t, "KVSet", "project_channels_proj-002", mock.MatchedBy(func(data []byte) bool {
			var saved []string
			_ = json.Unmarshal(data, &saved)
			return len(saved) == 1 && saved[0] == "channel-3"
		}))
	})

	t.Run("empty KV store returns zero", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVList", 0, 100).Return([]string{}, nil)

		count, err := s.RebuildReverseIndex()
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("KVList error returns error", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVList", 0, 100).Return(nil, &model.AppError{Message: "list error"})

		count, err := s.RebuildReverseIndex()
		require.Error(t, err)
		assert.Equal(t, 0, count)
		assert.Contains(t, err.Error(), "KVList failed")
	})

	t.Run("skips non channel_project_ keys", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVList", 0, 100).Return([]string{
			"user_plane_mm-user-1",
			"notify_config_channel-1",
			"digest_config_channel-1",
		}, nil)

		count, err := s.RebuildReverseIndex()
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestGetNotificationConfig(t *testing.T) {
	t.Run("config exists", func(t *testing.T) {
		s, api := setupTestStore(t)

		cfg := &NotificationConfig{
			Enabled:   true,
			UpdatedBy: "mm-user-1",
			UpdatedAt: 1710000000,
		}
		data, _ := json.Marshal(cfg)

		api.On("KVGet", "notify_config_channel-1").Return(data, nil)

		result, err := s.GetNotificationConfig("channel-1")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Enabled)
		assert.Equal(t, "mm-user-1", result.UpdatedBy)
	})

	t.Run("config not found", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVGet", "notify_config_channel-2").Return(nil, nil)

		result, err := s.GetNotificationConfig("channel-2")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestSaveNotificationConfig(t *testing.T) {
	s, api := setupTestStore(t)

	cfg := &NotificationConfig{
		Enabled:   true,
		UpdatedBy: "mm-user-1",
		UpdatedAt: 1710000000,
	}

	api.On("KVSet", "notify_config_channel-1", mock.AnythingOfType("[]uint8")).Return(nil)

	err := s.SaveNotificationConfig("channel-1", cfg)
	require.NoError(t, err)

	api.AssertCalled(t, "KVSet", "notify_config_channel-1", mock.MatchedBy(func(data []byte) bool {
		var saved NotificationConfig
		_ = json.Unmarshal(data, &saved)
		return saved.Enabled && saved.UpdatedBy == "mm-user-1"
	}))
}

func TestGetDigestConfig(t *testing.T) {
	t.Run("config exists", func(t *testing.T) {
		s, api := setupTestStore(t)

		cfg := &DigestConfig{
			Frequency: "daily",
			Hour:      9,
			UpdatedBy: "mm-user-1",
			UpdatedAt: 1710000000,
		}
		data, _ := json.Marshal(cfg)

		api.On("KVGet", "digest_config_channel-1").Return(data, nil)

		result, err := s.GetDigestConfig("channel-1")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "daily", result.Frequency)
		assert.Equal(t, 9, result.Hour)
		assert.Equal(t, "mm-user-1", result.UpdatedBy)
	})

	t.Run("config not found", func(t *testing.T) {
		s, api := setupTestStore(t)

		api.On("KVGet", "digest_config_channel-2").Return(nil, nil)

		result, err := s.GetDigestConfig("channel-2")
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestSaveDigestConfig(t *testing.T) {
	s, api := setupTestStore(t)

	cfg := &DigestConfig{
		Frequency: "weekly",
		Hour:      14,
		Weekday:   1,
		UpdatedBy: "mm-user-1",
		UpdatedAt: 1710000000,
	}

	api.On("KVSet", "digest_config_channel-1", mock.AnythingOfType("[]uint8")).Return(nil)

	err := s.SaveDigestConfig("channel-1", cfg)
	require.NoError(t, err)

	api.AssertCalled(t, "KVSet", "digest_config_channel-1", mock.MatchedBy(func(data []byte) bool {
		var saved DigestConfig
		_ = json.Unmarshal(data, &saved)
		return saved.Frequency == "weekly" && saved.Hour == 14 && saved.Weekday == 1
	}))
}
