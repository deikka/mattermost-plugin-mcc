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
