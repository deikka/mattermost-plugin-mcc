package store

import (
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	// prefixUserPlane is the KV store key prefix for Plane user mappings.
	prefixUserPlane = "user_plane_"

	// prefixUserObsidian is the KV store key prefix for Obsidian configurations.
	prefixUserObsidian = "user_obsidian_"
)

// PlaneUserMapping stores the mapping between a Mattermost user and their Plane account.
type PlaneUserMapping struct {
	PlaneUserID      string `json:"plane_user_id"`
	PlaneEmail       string `json:"plane_email"`
	PlaneDisplayName string `json:"plane_display_name"`
	ConnectedAt      int64  `json:"connected_at"`
}

// ObsidianConfig stores the Obsidian REST API configuration for a user.
type ObsidianConfig struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	APIKey  string `json:"api_key"`
	SetupAt int64  `json:"setup_at"`
}

// Store wraps the Mattermost KV store for plugin-specific data operations.
type Store struct {
	api plugin.API
}

// New creates a new Store backed by the given plugin API.
func New(api plugin.API) *Store {
	return &Store{api: api}
}

// GetPlaneUser retrieves the Plane user mapping for a Mattermost user ID.
func (s *Store) GetPlaneUser(mmUserID string) (*PlaneUserMapping, error) {
	data, appErr := s.api.KVGet(prefixUserPlane + mmUserID)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
	}
	if data == nil {
		return nil, nil
	}

	var mapping PlaneUserMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, fmt.Errorf("unmarshal PlaneUserMapping: %w", err)
	}
	return &mapping, nil
}

// SavePlaneUser stores the Plane user mapping for a Mattermost user ID.
func (s *Store) SavePlaneUser(mmUserID string, mapping *PlaneUserMapping) error {
	data, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshal PlaneUserMapping: %w", err)
	}

	if appErr := s.api.KVSet(prefixUserPlane+mmUserID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}
	return nil
}

// DeletePlaneUser removes the Plane user mapping for a Mattermost user ID.
func (s *Store) DeletePlaneUser(mmUserID string) error {
	if appErr := s.api.KVDelete(prefixUserPlane + mmUserID); appErr != nil {
		return fmt.Errorf("KVDelete failed: %s", appErr.Error())
	}
	return nil
}

// GetObsidianConfig retrieves the Obsidian configuration for a Mattermost user ID.
func (s *Store) GetObsidianConfig(mmUserID string) (*ObsidianConfig, error) {
	data, appErr := s.api.KVGet(prefixUserObsidian + mmUserID)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
	}
	if data == nil {
		return nil, nil
	}

	var config ObsidianConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal ObsidianConfig: %w", err)
	}
	return &config, nil
}

// SaveObsidianConfig stores the Obsidian configuration for a Mattermost user ID.
func (s *Store) SaveObsidianConfig(mmUserID string, config *ObsidianConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal ObsidianConfig: %w", err)
	}

	if appErr := s.api.KVSet(prefixUserObsidian+mmUserID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}
	return nil
}

// === Phase 2: Channel-Project Binding ===

const (
	// prefixChannelProject is the KV store key prefix for channel-project bindings.
	prefixChannelProject = "channel_project_"
)

// ChannelProjectBinding stores the 1:1 mapping between a Mattermost channel and a Plane project.
type ChannelProjectBinding struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	BoundBy     string `json:"bound_by"`  // Mattermost user ID who created binding
	BoundAt     int64  `json:"bound_at"`  // Unix timestamp
}

// GetChannelBinding retrieves the project binding for a Mattermost channel ID.
// Returns nil, nil if no binding exists.
func (s *Store) GetChannelBinding(channelID string) (*ChannelProjectBinding, error) {
	data, appErr := s.api.KVGet(prefixChannelProject + channelID)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
	}
	if data == nil {
		return nil, nil
	}

	var binding ChannelProjectBinding
	if err := json.Unmarshal(data, &binding); err != nil {
		return nil, fmt.Errorf("unmarshal ChannelProjectBinding: %w", err)
	}
	return &binding, nil
}

// SaveChannelBinding stores the project binding for a Mattermost channel ID.
func (s *Store) SaveChannelBinding(channelID string, binding *ChannelProjectBinding) error {
	data, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("marshal ChannelProjectBinding: %w", err)
	}

	if appErr := s.api.KVSet(prefixChannelProject+channelID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}
	return nil
}

// DeleteChannelBinding removes the project binding for a Mattermost channel ID.
func (s *Store) DeleteChannelBinding(channelID string) error {
	if appErr := s.api.KVDelete(prefixChannelProject + channelID); appErr != nil {
		return fmt.Errorf("KVDelete failed: %s", appErr.Error())
	}
	return nil
}

// IsPlaneConnected returns true if the given Mattermost user has a Plane account linked.
func (s *Store) IsPlaneConnected(mmUserID string) (bool, error) {
	mapping, err := s.GetPlaneUser(mmUserID)
	if err != nil {
		return false, err
	}
	return mapping != nil, nil
}
