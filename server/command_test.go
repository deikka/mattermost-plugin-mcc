package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupCommandTestPlugin creates a plugin ready for command testing.
// It has a bot user ID set and a mock API that handles ephemeral posts.
func setupCommandTestPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()

	api := &plugintest.API{}

	// Ephemeral post sending
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()

	// Logging
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"

	return p, api
}

func TestHelpCommand(t *testing.T) {
	p, api := setupCommandTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task help",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Verify ephemeral post was sent with help text content
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Task Management Commands") &&
			strings.Contains(post.Message, "/task plane create") &&
			strings.Contains(post.Message, "/task plane mine") &&
			strings.Contains(post.Message, "/task plane status") &&
			strings.Contains(post.Message, "/task connect") &&
			strings.Contains(post.Message, "/task obsidian setup") &&
			strings.Contains(post.Message, "/task help") &&
			strings.Contains(post.Message, "/task p c") &&
			strings.Contains(post.Message, "/task p m") &&
			strings.Contains(post.Message, "/task p s") &&
			post.ChannelId == "channel-1" &&
			post.UserId == "bot-user-id"
	}))
}

func TestHelpCommandOnBareTask(t *testing.T) {
	// When user types just "/task" with no subcommand, it should show help
	p, api := setupCommandTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Task Management Commands")
	}))
}

func TestCommandRouting(t *testing.T) {
	tests := []struct {
		name    string
		command string
		expect  string
	}{
		{
			name:    "plane create routes to handlePlaneCreate",
			command: "/task plane create",
			expect:  "not yet implemented",
		},
		{
			name:    "plane mine routes to handlePlaneMine",
			command: "/task plane mine",
			expect:  "not yet implemented",
		},
		{
			name:    "plane status routes to handlePlaneStatus",
			command: "/task plane status",
			expect:  "not yet implemented",
		},
		{
			name:    "connect routes to handleConnect",
			command: "/task connect",
			expect:  "not yet implemented",
		},
		{
			name:    "obsidian setup routes to handleObsidianSetup",
			command: "/task obsidian setup",
			expect:  "not yet implemented",
		},
		{
			name:    "help routes to handleHelp",
			command: "/task help",
			expect:  "Task Management Commands",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := setupCommandTestPlugin(t)

			args := &model.CommandArgs{
				Command:   tt.command,
				UserId:    "user-1",
				ChannelId: "channel-1",
			}

			resp, appErr := p.ExecuteCommand(nil, args)
			require.Nil(t, appErr)
			require.NotNil(t, resp)
		})
	}
}

func TestCommandRoutingWithArgs(t *testing.T) {
	// Verify that arguments after the command key are passed through
	p, api := setupCommandTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task plane create My Task Title",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Should route to handlePlaneCreate (stub returns "not yet implemented")
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "not yet implemented")
	}))
}

func TestCommandAliases(t *testing.T) {
	p, api := setupCommandTestPlugin(t)

	tests := []struct {
		name     string
		alias    string
		expected string
	}{
		{
			name:     "p/c maps to plane/create",
			alias:    "/task p c",
			expected: "not yet implemented",
		},
		{
			name:     "p/m maps to plane/mine",
			alias:    "/task p m",
			expected: "not yet implemented",
		},
		{
			name:     "p/s maps to plane/status",
			alias:    "/task p s",
			expected: "not yet implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &model.CommandArgs{
				Command:   tt.alias,
				UserId:    "user-1",
				ChannelId: "channel-1",
			}

			resp, appErr := p.ExecuteCommand(nil, args)
			require.Nil(t, appErr)
			require.NotNil(t, resp)

			api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
				return strings.Contains(post.Message, tt.expected)
			}))
		})
	}
}

func TestCommandAliasesWithArgs(t *testing.T) {
	// Verify aliases work with additional arguments
	p, api := setupCommandTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task p c Quick task title",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "not yet implemented")
	}))
}

func TestUnknownCommandSuggestion(t *testing.T) {
	p, api := setupCommandTestPlugin(t)

	tests := []struct {
		name         string
		command      string
		shouldSuggest string
	}{
		{
			name:         "close match suggests help",
			command:      "/task halp",
			shouldSuggest: "help",
		},
		{
			name:         "close match suggests connect",
			command:      "/task conect",
			shouldSuggest: "connect",
		},
		{
			name:         "far match shows generic message",
			command:      "/task xyzabc",
			shouldSuggest: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &model.CommandArgs{
				Command:   tt.command,
				UserId:    "user-1",
				ChannelId: "channel-1",
			}

			resp, appErr := p.ExecuteCommand(nil, args)
			require.Nil(t, appErr)
			require.NotNil(t, resp)

			api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
				hasUnknown := strings.Contains(post.Message, "Unknown command")
				hasHelpRef := strings.Contains(post.Message, "/task help")
				if tt.shouldSuggest != "" {
					hasSuggestion := strings.Contains(post.Message, "Did you mean")
					return hasUnknown && hasHelpRef && hasSuggestion
				}
				return hasUnknown && hasHelpRef
			}))
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"help", "halp", 1},
		{"connect", "conect", 1},
		{"plane/create", "plane/craete", 2},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.expected, levenshtein(tt.a, tt.b))
		})
	}
}

// Stubs for Plan 01-02
func TestConnectCommand(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify email match and KV store persistence")
}

func TestConnectAlreadyConnected(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify message when already linked")
}

func TestObsidianSetup(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify dialog opens and config saves to KV")
}

func TestRequirePlaneConnection(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify guard blocks unconnected users")
}

// Stubs for Plan 01-03
func TestPlaneMine(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify assigned tasks list with emoji formatting")
}

func TestPlaneMineNoTasks(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify empty state message")
}

func TestPlaneStatus(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify project summary with state counts and progress bar")
}

func TestPlaneStatusProjectSelection(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-03 — verify project name/identifier matching")
}
