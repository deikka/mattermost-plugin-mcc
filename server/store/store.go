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
// Also maintains the reverse index (project -> channels).
func (s *Store) SaveChannelBinding(channelID string, binding *ChannelProjectBinding) error {
	data, err := json.Marshal(binding)
	if err != nil {
		return fmt.Errorf("marshal ChannelProjectBinding: %w", err)
	}

	if appErr := s.api.KVSet(prefixChannelProject+channelID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}

	// Maintain reverse index
	if err := s.AddProjectChannel(binding.ProjectID, channelID); err != nil {
		return fmt.Errorf("update reverse index: %w", err)
	}

	return nil
}

// DeleteChannelBinding removes the project binding for a Mattermost channel ID.
// Also maintains the reverse index (project -> channels).
func (s *Store) DeleteChannelBinding(channelID string) error {
	// Read existing binding to get projectID for reverse index
	binding, err := s.GetChannelBinding(channelID)
	if err != nil {
		return err
	}

	if appErr := s.api.KVDelete(prefixChannelProject + channelID); appErr != nil {
		return fmt.Errorf("KVDelete failed: %s", appErr.Error())
	}

	// Maintain reverse index if binding existed
	if binding != nil {
		if err := s.RemoveProjectChannel(binding.ProjectID, channelID); err != nil {
			return fmt.Errorf("update reverse index: %w", err)
		}
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

// === Phase 3: Notifications + Automation ===

const (
	// prefixNotifyConfig is the KV store key prefix for channel notification settings.
	prefixNotifyConfig = "notify_config_"

	// prefixDigestConfig is the KV store key prefix for channel digest settings.
	prefixDigestConfig = "digest_config_"

	// prefixProjectChannels is the KV store key prefix for the reverse index (project -> channels).
	prefixProjectChannels = "project_channels_"

	// prefixWebhookDedup is the KV store key prefix for webhook delivery deduplication.
	prefixWebhookDedup = "webhook_dedup_"

	// prefixWorkItemState is the KV store key prefix for cached work item state.
	prefixWorkItemState = "work_item_state_"

	// prefixPluginAction is the KV store key prefix for tracking plugin-originated actions.
	prefixPluginAction = "plugin_action_"

	// prefixDigestLast is the KV store key prefix for the last digest run timestamp.
	prefixDigestLast = "digest_last_"
)

// NotificationConfig stores per-channel notification settings.
type NotificationConfig struct {
	Enabled   bool   `json:"enabled"`
	UpdatedBy string `json:"updated_by"`
	UpdatedAt int64  `json:"updated_at"`
}

// DigestConfig stores per-channel digest settings.
type DigestConfig struct {
	Frequency string `json:"frequency"` // "daily", "weekly", "off"
	Hour      int    `json:"hour"`      // 0-23, default 9
	Weekday   int    `json:"weekday"`   // 0=Sunday..6=Saturday (only for weekly)
	UpdatedBy string `json:"updated_by"`
	UpdatedAt int64  `json:"updated_at"`
}

// WorkItemStateCache stores a cached snapshot of a work item's state.
type WorkItemStateCache struct {
	StateGroup string `json:"state_group"`
	StateName  string `json:"state_name"`
	Priority   string `json:"priority,omitempty"`
	TargetDate string `json:"target_date,omitempty"`
	CachedAt   int64  `json:"cached_at"`
}

// GetNotificationConfig retrieves the notification config for a channel.
// Returns nil, nil if no config exists.
func (s *Store) GetNotificationConfig(channelID string) (*NotificationConfig, error) {
	data, appErr := s.api.KVGet(prefixNotifyConfig + channelID)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
	}
	if data == nil {
		return nil, nil
	}

	var config NotificationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal NotificationConfig: %w", err)
	}
	return &config, nil
}

// SaveNotificationConfig stores the notification config for a channel.
func (s *Store) SaveNotificationConfig(channelID string, config *NotificationConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal NotificationConfig: %w", err)
	}

	if appErr := s.api.KVSet(prefixNotifyConfig+channelID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}
	return nil
}

// GetDigestConfig retrieves the digest config for a channel.
// Returns nil, nil if no config exists.
func (s *Store) GetDigestConfig(channelID string) (*DigestConfig, error) {
	data, appErr := s.api.KVGet(prefixDigestConfig + channelID)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
	}
	if data == nil {
		return nil, nil
	}

	var config DigestConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal DigestConfig: %w", err)
	}
	return &config, nil
}

// SaveDigestConfig stores the digest config for a channel.
func (s *Store) SaveDigestConfig(channelID string, config *DigestConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal DigestConfig: %w", err)
	}

	if appErr := s.api.KVSet(prefixDigestConfig+channelID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}
	return nil
}

// GetProjectChannels retrieves the list of channel IDs bound to a project (reverse index).
// Returns nil, nil if no channels are bound.
func (s *Store) GetProjectChannels(projectID string) ([]string, error) {
	data, appErr := s.api.KVGet(prefixProjectChannels + projectID)
	if appErr != nil {
		return nil, fmt.Errorf("KVGet failed: %s", appErr.Error())
	}
	if data == nil {
		return nil, nil
	}

	var channels []string
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, fmt.Errorf("unmarshal project channels: %w", err)
	}
	return channels, nil
}

// SaveProjectChannels stores the list of channel IDs bound to a project (reverse index).
func (s *Store) SaveProjectChannels(projectID string, channelIDs []string) error {
	data, err := json.Marshal(channelIDs)
	if err != nil {
		return fmt.Errorf("marshal project channels: %w", err)
	}

	if appErr := s.api.KVSet(prefixProjectChannels+projectID, data); appErr != nil {
		return fmt.Errorf("KVSet failed: %s", appErr.Error())
	}
	return nil
}

// AddProjectChannel adds a channel to the reverse index for a project.
// If the channel is already present, no duplicate is added.
func (s *Store) AddProjectChannel(projectID, channelID string) error {
	channels, err := s.GetProjectChannels(projectID)
	if err != nil {
		return err
	}

	// Check for duplicate
	for _, ch := range channels {
		if ch == channelID {
			return nil
		}
	}

	channels = append(channels, channelID)
	return s.SaveProjectChannels(projectID, channels)
}

// RebuildReverseIndex scans all channel_project_ bindings and rebuilds the
// project_channels_ reverse index. This ensures bindings created before the
// reverse index existed are properly indexed for webhook routing.
func (s *Store) RebuildReverseIndex() (int, error) {
	page := 0
	perPage := 100
	count := 0

	for {
		keys, appErr := s.api.KVList(page, perPage)
		if appErr != nil {
			return count, fmt.Errorf("KVList failed: %s", appErr.Error())
		}

		for _, key := range keys {
			if len(key) <= len(prefixChannelProject) {
				continue
			}
			if key[:len(prefixChannelProject)] != prefixChannelProject {
				continue
			}
			channelID := key[len(prefixChannelProject):]
			binding, err := s.GetChannelBinding(channelID)
			if err != nil || binding == nil {
				continue
			}
			if err := s.AddProjectChannel(binding.ProjectID, channelID); err != nil {
				continue
			}
			count++
		}

		if len(keys) < perPage {
			break
		}
		page++
	}

	return count, nil
}

// RemoveProjectChannel removes a channel from the reverse index for a project.
// If the list becomes empty, the key is deleted.
func (s *Store) RemoveProjectChannel(projectID, channelID string) error {
	channels, err := s.GetProjectChannels(projectID)
	if err != nil {
		return err
	}

	filtered := make([]string, 0, len(channels))
	for _, ch := range channels {
		if ch != channelID {
			filtered = append(filtered, ch)
		}
	}

	if len(filtered) == 0 {
		if appErr := s.api.KVDelete(prefixProjectChannels + projectID); appErr != nil {
			return fmt.Errorf("KVDelete failed: %s", appErr.Error())
		}
		return nil
	}

	return s.SaveProjectChannels(projectID, filtered)
}
