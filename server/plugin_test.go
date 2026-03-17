package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupTestPlugin creates a Plugin instance with a mock API configured for
// configuration testing only (no OnActivate).
func setupTestPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()

	api := &plugintest.API{}
	api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).
		Return(nil).
		Run(func(args mock.Arguments) {
			cfg := args.Get(0).(*configuration)
			cfg.PlaneURL = "http://localhost:8765"
			cfg.PlaneAPIKey = "test-api-key"
			cfg.PlaneWorkspace = "test-workspace"
		})

	p := &Plugin{}
	p.SetAPI(api)

	return p, api
}

// setupActivatedPlugin creates a Plugin that has gone through OnActivate successfully.
// Uses permissive mocking (Maybe()) for the pluginapi internals since we don't
// want tests to be coupled to the pluginapi implementation details.
func setupActivatedPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()

	api := &plugintest.API{}

	// Config loading
	api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).
		Return(nil).
		Run(func(args mock.Arguments) {
			cfg := args.Get(0).(*configuration)
			cfg.PlaneURL = ""
			cfg.PlaneAPIKey = ""
			cfg.PlaneWorkspace = ""
		})

	// pluginapi.Client internals
	api.On("GetServerVersion").Return("10.0.0").Maybe()
	api.On("GetBundlePath").Return("/tmp/test-bundle", nil).Maybe()

	// Bot creation via pluginapi: EnsureBotUser is the actual API method called
	api.On("EnsureBotUser", mock.AnythingOfType("*model.Bot")).
		Return("bot-user-id", nil)

	// cluster.Mutex and cluster.Schedule KV operations
	api.On("KVSetWithOptions", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Maybe()
	api.On("KVGet", mock.Anything).Return(nil, nil).Maybe()
	api.On("KVDelete", mock.Anything).Return(nil).Maybe()
	api.On("KVList", mock.Anything, mock.Anything).Return([]string{}, nil).Maybe()

	// RegisterCommand
	api.On("RegisterCommand", mock.AnythingOfType("*model.Command")).Return(nil)

	// Logging (permissive -- don't care about exact log calls)
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.SetDriver(nil)

	return p, api
}

func TestConfiguration(t *testing.T) {
	p, api := setupTestPlugin(t)

	err := p.OnConfigurationChange()
	require.NoError(t, err)

	cfg := p.getConfiguration()
	assert.Equal(t, "http://localhost:8765", cfg.PlaneURL)
	assert.Equal(t, "test-api-key", cfg.PlaneAPIKey)
	assert.Equal(t, "test-workspace", cfg.PlaneWorkspace)

	api.AssertExpectations(t)
}

func TestConfigurationDefaults(t *testing.T) {
	p := &Plugin{}
	cfg := p.getConfiguration()
	assert.Equal(t, "", cfg.PlaneURL)
	assert.Equal(t, "", cfg.PlaneAPIKey)
	assert.Equal(t, "", cfg.PlaneWorkspace)
}

func TestOnActivate(t *testing.T) {
	p, api := setupActivatedPlugin(t)

	err := p.OnActivate()
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.OnDeactivate() })

	// Verify bot was created
	assert.Equal(t, "bot-user-id", p.botUserID)

	// Verify router was initialized
	assert.NotNil(t, p.router)

	// Verify config was loaded
	cfg := p.getConfiguration()
	assert.NotNil(t, cfg)

	api.AssertExpectations(t)
}

func TestOnActivateBotCreation(t *testing.T) {
	p, api := setupActivatedPlugin(t)

	err := p.OnActivate()
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.OnDeactivate() })

	// Verify bot was created with correct params
	api.AssertCalled(t, "EnsureBotUser", mock.MatchedBy(func(bot *model.Bot) bool {
		return bot.Username == "task-bot" &&
			bot.DisplayName == "Task Bot" &&
			bot.Description == "Mattermost Command Center - Task management bot"
	}))

	assert.Equal(t, "bot-user-id", p.botUserID)
	api.AssertExpectations(t)
}
