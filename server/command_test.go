package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
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

	// Initialize store with mock API
	p.store = store.New(api)

	// Initialize plane client (unconfigured by default for routing tests)
	p.planeClient = plane.NewClient("", "", "")

	return p, api
}

// setupConnectTestPlugin creates a plugin with a configured Plane client backed by a mock server.
func setupConnectTestPlugin(t *testing.T, planeHandler http.HandlerFunc) (*Plugin, *plugintest.API, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(planeHandler)
	t.Cleanup(server.Close)

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
	p.planeClient = plane.NewClient(server.URL, "test-api-key", "test-workspace")

	return p, api, server
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
		return strings.Contains(post.Message, "Comandos de Gestion de Tareas") &&
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
		return strings.Contains(post.Message, "Comandos de Gestion de Tareas")
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
			expect:  "no has vinculado",
		},
		{
			name:    "plane mine routes to handlePlaneMine",
			command: "/task plane mine",
			expect:  "no has vinculado",
		},
		{
			name:    "plane status routes to handlePlaneStatus",
			command: "/task plane status",
			expect:  "no has vinculado",
		},
		{
			name:    "help routes to handleHelp",
			command: "/task help",
			expect:  "Comandos de Gestion de Tareas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, api := setupCommandTestPlugin(t)
			// Mock KVGet for requirePlaneConnection (user not connected)
			api.On("KVGet", mock.AnythingOfType("string")).Return(nil, nil).Maybe()

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

func TestCommandRoutingConnect(t *testing.T) {
	// Connect handler now has real logic -- Plane not configured returns specific message
	p, api := setupCommandTestPlugin(t)

	args := &model.CommandArgs{
		Command:   "/task connect",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "Plane no esta configurado")
	}))
}

func TestCommandRoutingObsidianSetup(t *testing.T) {
	// Obsidian setup opens a dialog -- since TriggerID is empty, it will fail
	p, api := setupCommandTestPlugin(t)

	// OpenInteractiveDialog will be called
	api.On("OpenInteractiveDialog", mock.AnythingOfType("model.OpenDialogRequest")).
		Return(&model.AppError{Message: "invalid trigger"})

	args := &model.CommandArgs{
		Command:   "/task obsidian setup",
		UserId:    "user-1",
		ChannelId: "channel-1",
		TriggerId: "",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Should get error about dialog failing
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "No se pudo abrir el dialogo de configuracion")
	}))
}

func TestCommandRoutingWithArgs(t *testing.T) {
	// Verify that arguments after the command key are passed through
	p, api := setupCommandTestPlugin(t)
	// Mock: user not connected -- requirePlaneConnection will block
	api.On("KVGet", mock.AnythingOfType("string")).Return(nil, nil).Maybe()

	args := &model.CommandArgs{
		Command:   "/task plane create My Task Title",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Should route to handlePlaneCreate which requires connection
	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no has vinculado")
	}))
}

func TestCommandAliases(t *testing.T) {
	p, api := setupCommandTestPlugin(t)
	// Mock: user not connected -- all Plane commands require connection
	api.On("KVGet", mock.AnythingOfType("string")).Return(nil, nil).Maybe()

	tests := []struct {
		name     string
		alias    string
		expected string
	}{
		{
			name:     "p/c maps to plane/create",
			alias:    "/task p c",
			expected: "no has vinculado",
		},
		{
			name:     "p/m maps to plane/mine",
			alias:    "/task p m",
			expected: "no has vinculado",
		},
		{
			name:     "p/s maps to plane/status",
			alias:    "/task p s",
			expected: "no has vinculado",
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
	// Mock: user not connected
	api.On("KVGet", mock.AnythingOfType("string")).Return(nil, nil).Maybe()

	args := &model.CommandArgs{
		Command:   "/task p c Quick task title",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "no has vinculado")
	}))
}

func TestUnknownCommandSuggestion(t *testing.T) {
	p, api := setupCommandTestPlugin(t)

	tests := []struct {
		name          string
		command       string
		shouldSuggest string
	}{
		{
			name:          "close match suggests help",
			command:       "/task halp",
			shouldSuggest: "help",
		},
		{
			name:          "close match suggests connect",
			command:       "/task conect",
			shouldSuggest: "connect",
		},
		{
			name:          "far match shows generic message",
			command:       "/task xyzabc",
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

// === Plan 01-02 Tests ===

func TestConnectCommand(t *testing.T) {
	t.Run("successful email match", func(t *testing.T) {
		p, api, _ := setupConnectTestPlugin(t, func(w http.ResponseWriter, r *http.Request) {
			// Workspace members endpoint
			w.WriteHeader(http.StatusOK)
			members := []plane.MemberWrapper{
				{ID: "plane-u1", Email: "alice@example.com", DisplayName: "Alice Smith"},
				{ID: "plane-u2", Email: "bob@example.com", DisplayName: "Bob Jones"},
			}
			_ = json.NewEncoder(w).Encode(members)
		})

		// Mock: user not yet connected
		api.On("KVGet", "user_plane_user-1").Return(nil, nil)
		// Mock: GetUser returns user with matching email
		api.On("GetUser", "user-1").Return(&model.User{
			Id:    "user-1",
			Email: "alice@example.com",
		}, nil)
		// Mock: SavePlaneUser
		api.On("KVSet", "user_plane_user-1", mock.AnythingOfType("[]uint8")).Return(nil)

		args := &model.CommandArgs{
			Command:   "/task connect",
			UserId:    "user-1",
			ChannelId: "channel-1",
		}

		resp, appErr := p.ExecuteCommand(nil, args)
		require.Nil(t, appErr)
		require.NotNil(t, resp)

		// Verify success message
		api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "Conectado!") &&
				strings.Contains(post.Message, "Alice Smith") &&
				strings.Contains(post.Message, "alice@example.com")
		}))

		// Verify mapping was saved
		api.AssertCalled(t, "KVSet", "user_plane_user-1", mock.MatchedBy(func(data []byte) bool {
			var mapping store.PlaneUserMapping
			_ = json.Unmarshal(data, &mapping)
			return mapping.PlaneUserID == "plane-u1" &&
				mapping.PlaneEmail == "alice@example.com"
		}))
	})

	t.Run("no email match", func(t *testing.T) {
		p, api, _ := setupConnectTestPlugin(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			members := []plane.MemberWrapper{
				{ID: "plane-u1", Email: "other@example.com", DisplayName: "Other"},
			}
			_ = json.NewEncoder(w).Encode(members)
		})

		api.On("KVGet", "user_plane_user-1").Return(nil, nil)
		api.On("GetUser", "user-1").Return(&model.User{
			Id:    "user-1",
			Email: "alice@example.com",
		}, nil)

		args := &model.CommandArgs{
			Command:   "/task connect",
			UserId:    "user-1",
			ChannelId: "channel-1",
		}

		resp, appErr := p.ExecuteCommand(nil, args)
		require.Nil(t, appErr)
		require.NotNil(t, resp)

		api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "No se encontro una cuenta de Plane") &&
				strings.Contains(post.Message, "alice@example.com")
		}))
	})

	t.Run("plane not configured", func(t *testing.T) {
		p, api := setupCommandTestPlugin(t)
		// planeClient is already unconfigured from setupCommandTestPlugin

		args := &model.CommandArgs{
			Command:   "/task connect",
			UserId:    "user-1",
			ChannelId: "channel-1",
		}

		resp, appErr := p.ExecuteCommand(nil, args)
		require.Nil(t, appErr)
		require.NotNil(t, resp)

		api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "Plane no esta configurado")
		}))
	})
}

func TestConnectAlreadyConnected(t *testing.T) {
	p, api, _ := setupConnectTestPlugin(t, func(w http.ResponseWriter, r *http.Request) {
		// Not expected to be called
		w.WriteHeader(http.StatusOK)
	})

	// Mock: user already connected
	existingMapping := &store.PlaneUserMapping{
		PlaneUserID:      "plane-u1",
		PlaneEmail:       "alice@example.com",
		PlaneDisplayName: "Alice Smith",
		ConnectedAt:      1234567890,
	}
	data, _ := json.Marshal(existingMapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)

	args := &model.CommandArgs{
		Command:   "/task connect",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "ya esta vinculada") &&
			strings.Contains(post.Message, "Alice Smith") &&
			strings.Contains(post.Message, "alice@example.com")
	}))
}

func TestObsidianSetup(t *testing.T) {
	p, api := setupCommandTestPlugin(t)

	// Mock dialog opening
	api.On("OpenInteractiveDialog", mock.MatchedBy(func(req model.OpenDialogRequest) bool {
		return req.Dialog.Title == "Configurar Obsidian REST API" &&
			len(req.Dialog.Elements) == 3 &&
			req.Dialog.Elements[0].Name == "host" &&
			req.Dialog.Elements[1].Name == "port" &&
			req.Dialog.Elements[2].Name == "api_key" &&
			req.Dialog.Elements[0].Default == "127.0.0.1" &&
			req.Dialog.Elements[1].Default == "27124" &&
			req.Dialog.Elements[2].SubType == "password"
	})).Return(nil)

	args := &model.CommandArgs{
		Command:   "/task obsidian setup",
		UserId:    "user-1",
		ChannelId: "channel-1",
		TriggerId: "trigger-123",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	// Dialog should have been opened
	api.AssertCalled(t, "OpenInteractiveDialog", mock.AnythingOfType("model.OpenDialogRequest"))
}

func TestRequirePlaneConnection(t *testing.T) {
	t.Run("connected user passes", func(t *testing.T) {
		p, api := setupCommandTestPlugin(t)

		mapping := &store.PlaneUserMapping{
			PlaneUserID:      "plane-u1",
			PlaneEmail:       "alice@example.com",
			PlaneDisplayName: "Alice",
		}
		data, _ := json.Marshal(mapping)
		api.On("KVGet", "user_plane_user-1").Return(data, nil)

		args := &model.CommandArgs{
			UserId:    "user-1",
			ChannelId: "channel-1",
		}

		result, ok := requirePlaneConnection(p, args)
		assert.True(t, ok)
		assert.NotNil(t, result)
		assert.Equal(t, "plane-u1", result.PlaneUserID)
	})

	t.Run("unconnected user blocked", func(t *testing.T) {
		p, api := setupCommandTestPlugin(t)

		api.On("KVGet", "user_plane_user-2").Return(nil, nil)

		args := &model.CommandArgs{
			UserId:    "user-2",
			ChannelId: "channel-1",
		}

		result, ok := requirePlaneConnection(p, args)
		assert.False(t, ok)
		assert.Nil(t, result)

		// Verify guidance message sent
		api.AssertCalled(t, "SendEphemeralPost", "user-2", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "no has vinculado tu cuenta de Plane") &&
				strings.Contains(post.Message, "/task connect")
		}))
	})
}

// setupMineStatusTestPlugin creates a plugin with a mock Plane server that serves
// projects, work items, and states for /task plane mine and /task plane status tests.
func setupMineStatusTestPlugin(t *testing.T, workItems []plane.WorkItem) (*Plugin, *plugintest.API, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/states/"):
			resp := map[string]interface{}{
				"results": []plane.State{
					{ID: "s1", Name: "Backlog", Group: "backlog", Sequence: 1},
					{ID: "s2", Name: "Todo", Group: "unstarted", Sequence: 2},
					{ID: "s3", Name: "In Progress", Group: "started", Sequence: 3},
					{ID: "s4", Name: "Done", Group: "completed", Sequence: 4},
					{ID: "s5", Name: "Cancelled", Group: "cancelled", Sequence: 5},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/work-items/"):
			resp := map[string]interface{}{
				"results": workItems,
			}
			_ = json.NewEncoder(w).Encode(resp)

		case strings.Contains(r.URL.Path, "/projects/"):
			resp := map[string]interface{}{
				"results": []plane.Project{
					{ID: "proj-1", Name: "Alpha", Identifier: "ALP"},
					{ID: "proj-2", Name: "Beta", Identifier: "BET"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	api := &plugintest.API{}
	api.On("SendEphemeralPost", mock.Anything, mock.AnythingOfType("*model.Post")).Return(nil).Maybe()
	api.On("LogInfo", mock.Anything).Maybe()
	api.On("LogInfo", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogWarn", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()

	p := &Plugin{}
	p.SetAPI(api)
	p.botUserID = "bot-user-id"
	p.store = store.New(api)
	p.planeClient = plane.NewClient(server.URL, "test-api-key", "test-workspace")

	// Set configuration for URL/workspace access
	p.configuration = &configuration{
		PlaneURL:       server.URL,
		PlaneAPIKey:    "test-api-key",
		PlaneWorkspace: "test-workspace",
	}

	return p, api, server
}

func TestPlaneMine(t *testing.T) {
	items := []plane.WorkItem{
		{ID: "wi-1", Name: "Fix login bug", StateGroup: "started", StateName: "In Progress", Priority: "high", UpdatedAt: "2026-03-17T06:00:00Z"},
		{ID: "wi-2", Name: "Add tests", StateGroup: "unstarted", StateName: "Todo", Priority: "medium", UpdatedAt: "2026-03-17T05:00:00Z"},
		{ID: "wi-3", Name: "Deploy v2", StateGroup: "completed", StateName: "Done", Priority: "none", UpdatedAt: "2026-03-17T04:00:00Z"},
	}

	p, api, _ := setupMineStatusTestPlugin(t, items)

	// Mock: user connected
	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil).Maybe()

	args := &model.CommandArgs{
		Command:   "/task plane mine",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		msg := post.Message
		return strings.Contains(msg, "Tus tareas asignadas") &&
			strings.Contains(msg, "Fix login bug") &&
			strings.Contains(msg, "Add tests") &&
			strings.Contains(msg, "Deploy v2") &&
			strings.Contains(msg, "🔵") && // started emoji (Unicode)
			strings.Contains(msg, "⚪") && // unstarted emoji (Unicode)
			strings.Contains(msg, "✅") && // completed emoji (Unicode)
			strings.Contains(msg, "High :red_circle:") && // priority
			strings.Contains(msg, "Abrir Plane")
	}))
}

func TestPlaneMineNoTasks(t *testing.T) {
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil).Maybe()

	args := &model.CommandArgs{
		Command:   "/task plane mine",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		return strings.Contains(post.Message, "No tienes tareas asignadas") &&
			strings.Contains(post.Message, "/task plane create")
	}))
}

func TestPlaneStatus(t *testing.T) {
	items := []plane.WorkItem{
		{ID: "wi-1", Name: "Task 1", StateGroup: "backlog"},
		{ID: "wi-2", Name: "Task 2", StateGroup: "unstarted"},
		{ID: "wi-3", Name: "Task 3", StateGroup: "started"},
		{ID: "wi-4", Name: "Task 4", StateGroup: "completed"},
		{ID: "wi-5", Name: "Task 5", StateGroup: "completed"},
	}

	p, api, _ := setupMineStatusTestPlugin(t, items)

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil).Maybe()

	// Specify project name to avoid multi-project ambiguity
	args := &model.CommandArgs{
		Command:   "/task plane status Alpha",
		UserId:    "user-1",
		ChannelId: "channel-1",
	}

	resp, appErr := p.ExecuteCommand(nil, args)
	require.Nil(t, appErr)
	require.NotNil(t, resp)

	api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
		msg := post.Message
		return strings.Contains(msg, "**Proyecto: Alpha**") &&
			strings.Contains(msg, "ALP") &&
			strings.Contains(msg, ":white_circle: Abierto") &&
			strings.Contains(msg, ":large_blue_circle: En Progreso") &&
			strings.Contains(msg, ":white_check_mark: Hecho") &&
			strings.Contains(msg, "| 2 |") && // Open = backlog(1) + unstarted(1)
			strings.Contains(msg, "| 1 |") && // In Progress
			strings.Contains(msg, "Progreso:") &&
			strings.Contains(msg, "40%") && // 2 done / 5 total
			strings.Contains(msg, "**Total:** 5 tareas") &&
			strings.Contains(msg, "Abrir en Plane")
	}))
}

func TestPlaneStatusProjectSelection(t *testing.T) {
	p, api, _ := setupMineStatusTestPlugin(t, []plane.WorkItem{})

	mapping := &store.PlaneUserMapping{PlaneUserID: "plane-u1", PlaneEmail: "alice@example.com", PlaneDisplayName: "Alice"}
	data, _ := json.Marshal(mapping)
	api.On("KVGet", "user_plane_user-1").Return(data, nil)
	api.On("KVGet", "channel_project_channel-1").Return(nil, nil).Maybe()

	t.Run("matches by identifier", func(t *testing.T) {
		args := &model.CommandArgs{
			Command:   "/task plane status BET",
			UserId:    "user-1",
			ChannelId: "channel-1",
		}
		resp, appErr := p.ExecuteCommand(nil, args)
		require.Nil(t, appErr)
		require.NotNil(t, resp)

		api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "**Proyecto: Beta**")
		}))
	})

	t.Run("not found shows available", func(t *testing.T) {
		args := &model.CommandArgs{
			Command:   "/task plane status NonExistent",
			UserId:    "user-1",
			ChannelId: "channel-1",
		}
		resp, appErr := p.ExecuteCommand(nil, args)
		require.Nil(t, appErr)
		require.NotNil(t, resp)

		api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "no encontrado") &&
				strings.Contains(post.Message, "Alpha") &&
				strings.Contains(post.Message, "Beta")
		}))
	})

	t.Run("no project specified with multiple projects", func(t *testing.T) {
		args := &model.CommandArgs{
			Command:   "/task plane status",
			UserId:    "user-1",
			ChannelId: "channel-1",
		}
		resp, appErr := p.ExecuteCommand(nil, args)
		require.Nil(t, appErr)
		require.NotNil(t, resp)

		api.AssertCalled(t, "SendEphemeralPost", "user-1", mock.MatchedBy(func(post *model.Post) bool {
			return strings.Contains(post.Message, "Cual proyecto?") &&
				strings.Contains(post.Message, "Alpha") &&
				strings.Contains(post.Message, "Beta")
		}))
	})
}
