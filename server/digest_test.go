package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// setupDigestTestPlugin creates a Plugin with mock API, store, and planeClient for digest tests.
func setupDigestTestPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()

	api := &plugintest.API{}
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"
	p.store = store.New(api)
	p.planeClient = plane.NewClient("https://plane.example.com", "test-key", "test-workspace")

	return p, api
}

// sampleWorkItems returns a set of work items with mixed state groups for testing.
func sampleWorkItems() []plane.WorkItem {
	return []plane.WorkItem{
		{ID: "wi-1", Name: "Setup CI", StateGroup: "completed", Priority: "high", SequenceID: 1},
		{ID: "wi-2", Name: "Login page", StateGroup: "started", Priority: "medium", SequenceID: 2},
		{ID: "wi-3", Name: "Database schema", StateGroup: "completed", Priority: "high", SequenceID: 3},
		{ID: "wi-4", Name: "API docs", StateGroup: "backlog", Priority: "low", SequenceID: 4},
		{ID: "wi-5", Name: "Unit tests", StateGroup: "unstarted", Priority: "none", SequenceID: 5},
		{ID: "wi-6", Name: "Deploy script", StateGroup: "started", Priority: "medium", SequenceID: 6},
	}
}

func TestDigestExecution_Daily(t *testing.T) {
	p, api := setupDigestTestPlugin(t)

	channelID := "channel-daily"
	now := time.Now()

	// KVList returns digest config key for the channel
	api.On("KVList", 0, 100).Return([]string{
		"digest_config_" + channelID,
	}, nil)

	// Digest config: daily at current hour
	config := &store.DigestConfig{
		Frequency: "daily",
		Hour:      now.Hour(),
		UpdatedBy: "user-1",
		UpdatedAt: now.Unix(),
	}
	configData, _ := json.Marshal(config)
	api.On("KVGet", "digest_config_"+channelID).Return(configData, nil)

	// No previous digest run (never posted)
	api.On("KVGet", "digest_last_"+channelID).Return(nil, nil)

	// Channel is bound to a project
	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}
	bindingData, _ := json.Marshal(binding)
	api.On("KVGet", "channel_project_"+channelID).Return(bindingData, nil)

	// Mock CreatePost -- capture the digest post
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.ChannelId == channelID &&
			post.UserId == "bot-user-id" &&
			strings.Contains(post.Message, "Resumen del Proyecto") &&
			strings.Contains(post.Message, "Alpha")
	})).Return(&model.Post{}, nil)

	// Save last digest timestamp
	api.On("KVSetWithOptions", "digest_last_"+channelID, mock.Anything, mock.Anything).Return(true, nil)

	// Run digest check
	p.runDigestCheck()

	// Verify post was created
	api.AssertCalled(t, "CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.ChannelId == channelID &&
			strings.Contains(post.Message, "Resumen del Proyecto")
	}))
}

func TestDigestExecution_NotDueYet(t *testing.T) {
	p, api := setupDigestTestPlugin(t)

	channelID := "channel-not-due"
	now := time.Now()

	// KVList returns digest config key
	api.On("KVList", 0, 100).Return([]string{
		"digest_config_" + channelID,
	}, nil)

	// Digest config: daily at current hour
	config := &store.DigestConfig{
		Frequency: "daily",
		Hour:      now.Hour(),
		UpdatedBy: "user-1",
		UpdatedAt: now.Unix(),
	}
	configData, _ := json.Marshal(config)
	api.On("KVGet", "digest_config_"+channelID).Return(configData, nil)

	// Recent digest run (within the same hour) -- should NOT re-post
	recentTimestamp := fmt.Sprintf("%d", now.Unix()-60) // 1 minute ago
	api.On("KVGet", "digest_last_"+channelID).Return([]byte(recentTimestamp), nil)

	// Run digest check
	p.runDigestCheck()

	// Verify CreatePost was NOT called
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestDigestContent(t *testing.T) {
	p, _ := setupDigestTestPlugin(t)

	binding := &store.ChannelProjectBinding{
		ProjectID:   "proj-1",
		ProjectName: "Alpha",
		BoundBy:     "user-1",
		BoundAt:     1710000000,
	}

	items := sampleWorkItems()

	content := p.buildDigestPost(binding, items)

	// Verify content contains expected elements
	require.Contains(t, content, "Resumen del Proyecto: Alpha")
	require.Contains(t, content, "Abierto")
	require.Contains(t, content, "En Progreso")
	require.Contains(t, content, "Hecho")
	require.Contains(t, content, "Progreso:")
	require.Contains(t, content, "plane.example.com")
	require.Contains(t, content, "/task plane digest off")

	// Verify counts are correct: 2 completed, 2 started, 2 open (1 backlog + 1 unstarted)
	require.Contains(t, content, "| 2 |") // open and done both have 2
}
