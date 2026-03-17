package testutil

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
)

// TestPlaneURL is the default Plane URL used in tests. It can be overridden
// by pointing to a MockPlaneServer's URL.
const TestPlaneURL = "http://localhost:8765"

// TestPlaneAPIKey is the default Plane API key used in tests.
const TestPlaneAPIKey = "test-api-key"

// TestPlaneWorkspace is the default Plane workspace slug used in tests.
const TestPlaneWorkspace = "test-workspace"

// TestBotUserID is the default bot user ID used in tests.
const TestBotUserID = "bot-user-id-1234"

// TestConfig holds plugin configuration values for use in test setup.
// Since the configuration struct lives in package main, we cannot import it
// directly from a sub-package. Callers in package main_test should use these
// constants to populate their configuration structs.
type TestConfig struct {
	PlaneURL       string
	PlaneAPIKey    string
	PlaneWorkspace string
}

// DefaultTestConfig returns a TestConfig with standard test values.
func DefaultTestConfig() TestConfig {
	return TestConfig{
		PlaneURL:       TestPlaneURL,
		PlaneAPIKey:    TestPlaneAPIKey,
		PlaneWorkspace: TestPlaneWorkspace,
	}
}

// NewMockAPI creates a bare plugintest.API mock with cleanup registered
// on the test. No default expectations are set.
func NewMockAPI(t *testing.T) *plugintest.API {
	t.Helper()
	api := &plugintest.API{}
	t.Cleanup(func() {
		api.AssertExpectations(t)
	})
	return api
}

// SetupMockAPI creates a plugintest.API mock with common default expectations
// suitable for most test scenarios:
//   - LoadPluginConfiguration returns nil (success)
//   - RegisterCommand returns nil (success)
//   - EnsureBotUser returns TestBotUserID
//   - LogDebug, LogInfo, LogWarn, LogError are accepted silently
//
// The returned mock can be extended with additional .On() calls in each test.
func SetupMockAPI(t *testing.T) *plugintest.API {
	t.Helper()
	api := NewMockAPI(t)

	// Accept any LoadPluginConfiguration call
	api.On("LoadPluginConfiguration", mock.AnythingOfType("*main.configuration")).
		Return(nil).Maybe()

	// Accept command registration
	api.On("RegisterCommand", mock.Anything).Return(nil).Maybe()

	// Accept bot creation
	api.On("EnsureBotUser", mock.Anything).Return(TestBotUserID, nil).Maybe()

	// Accept logging at all levels silently
	api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

	return api
}
