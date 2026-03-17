package plane

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPlaneServer creates a mock Plane API server for testing.
// The handler validates the X-API-Key header and dispatches based on path.
func testPlaneServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewClient(server.URL, "test-api-key", "test-workspace")
	return server, client
}

func TestPlaneClientDoRequest(t *testing.T) {
	var capturedReq *http.Request
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	})

	resp, err := client.doRequest("GET", "/projects/", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify URL construction
	assert.Equal(t, "/api/v1/workspaces/test-workspace/projects/", capturedReq.URL.Path)

	// Verify auth header
	assert.Equal(t, "test-api-key", capturedReq.Header.Get("X-API-Key"))

	// Verify content type
	assert.Equal(t, "application/json", capturedReq.Header.Get("Content-Type"))

	// Verify status
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestPlaneClientDoRequestWithBody(t *testing.T) {
	var capturedBody map[string]interface{}
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": "new-item"}`))
	})

	body := &CreateWorkItemRequest{
		Name:     "Test task",
		Priority: "high",
	}
	resp, err := client.doRequest("POST", "/projects/p1/work-items/", body)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "Test task", capturedBody["name"])
	assert.Equal(t, "high", capturedBody["priority"])
}

func TestPlaneClientListProjects(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		resp := PaginatedResponse{
			Results: json.RawMessage(`[
				{"id": "p1", "name": "Backend", "identifier": "BACK"},
				{"id": "p2", "name": "Frontend", "identifier": "FRNT"}
			]`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	projects, err := client.ListProjects()
	require.NoError(t, err)
	assert.Len(t, projects, 2)
	assert.Equal(t, "Backend", projects[0].Name)
	assert.Equal(t, "BACK", projects[0].Identifier)
	assert.Equal(t, "Frontend", projects[1].Name)

	// Verify cache hit (second call should not hit server)
	projects2, err := client.ListProjects()
	require.NoError(t, err)
	assert.Equal(t, projects, projects2)
}

func TestPlaneClientListProjectStates(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := PaginatedResponse{
			Results: json.RawMessage(`[
				{"id": "s3", "name": "Done", "group": "completed", "sequence": 3000},
				{"id": "s1", "name": "Backlog", "group": "backlog", "sequence": 1000},
				{"id": "s2", "name": "In Progress", "group": "started", "sequence": 2000}
			]`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	states, err := client.ListProjectStates("proj1")
	require.NoError(t, err)
	assert.Len(t, states, 3)

	// Should be sorted by sequence
	assert.Equal(t, "Backlog", states[0].Name)
	assert.Equal(t, "In Progress", states[1].Name)
	assert.Equal(t, "Done", states[2].Name)
}

func TestPlaneClientListProjectLabels(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := PaginatedResponse{
			Results: json.RawMessage(`[
				{"id": "l1", "name": "bug", "color": "#EF4444"},
				{"id": "l2", "name": "enhancement", "color": "#3B82F6"}
			]`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	labels, err := client.ListProjectLabels("proj1")
	require.NoError(t, err)
	assert.Len(t, labels, 2)
	assert.Equal(t, "bug", labels[0].Name)
}

func TestPlaneClientListProjectMembers(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := PaginatedResponse{
			Results: json.RawMessage(`[
				{"id": "m1", "member": {"id": "u1", "email": "alice@example.com", "display_name": "Alice"}, "role": 20}
			]`),
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	members, err := client.ListProjectMembers("proj1")
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "Alice", members[0].Member.DisplayName)
	assert.Equal(t, "alice@example.com", members[0].Member.Email)
}

func TestPlaneClientListWorkspaceMembers(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		members := []MemberWrapper{
			{Member: Member{ID: "u1", Email: "alice@example.com", DisplayName: "Alice"}, Role: 20},
			{Member: Member{ID: "u2", Email: "bob@example.com", DisplayName: "Bob"}, Role: 15},
		}
		_ = json.NewEncoder(w).Encode(members)
	})

	members, err := client.ListWorkspaceMembers()
	require.NoError(t, err)
	assert.Len(t, members, 2)
	assert.Equal(t, "alice@example.com", members[0].Member.Email)
	assert.Equal(t, "bob@example.com", members[1].Member.Email)
}

func TestPlaneClientCreateWorkItem(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var req CreateWorkItemRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "New task", req.Name)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(WorkItem{
			ID:        "wi-new",
			Name:      "New task",
			State:     "s1",
			Priority:  "none",
			ProjectID: "p1",
		})
	})

	item, err := client.CreateWorkItem("p1", &CreateWorkItemRequest{
		Name: "New task",
	})
	require.NoError(t, err)
	assert.Equal(t, "wi-new", item.ID)
	assert.Equal(t, "New task", item.Name)
}

func TestPlaneClientCache(t *testing.T) {
	cache := NewCache()

	// Set and get
	cache.Set("key1", "value1", 1*time.Minute)
	val, ok := cache.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// Expired entry
	cache.Set("key2", "value2", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	val, ok = cache.Get("key2")
	assert.False(t, ok)
	assert.Nil(t, val)

	// Invalidate specific key
	cache.Set("key3", "value3", 1*time.Minute)
	cache.Invalidate("key3")
	_, ok = cache.Get("key3")
	assert.False(t, ok)

	// InvalidateAll
	cache.Set("a", 1, 1*time.Minute)
	cache.Set("b", 2, 1*time.Minute)
	cache.InvalidateAll()
	_, ok = cache.Get("a")
	assert.False(t, ok)
	_, ok = cache.Get("b")
	assert.False(t, ok)
}

func TestPlaneClientErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			body:       `{"error": "Invalid API key"}`,
			wantMsg:    "Invalid API key",
		},
		{
			name:       "404 not found",
			statusCode: http.StatusNotFound,
			body:       `{"error": "Not found"}`,
			wantMsg:    "Not found",
		},
		{
			name:       "429 rate limited",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error": "Rate limit exceeded", "detail": "Try again in 30 seconds"}`,
			wantMsg:    "Rate limit exceeded",
		},
		{
			name:       "500 with no body",
			statusCode: http.StatusInternalServerError,
			body:       "",
			wantMsg:    "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			})

			_, err := client.ListProjects()
			require.Error(t, err)
			apiErr, ok := err.(*APIError)
			require.True(t, ok, "expected *APIError, got %T", err)
			assert.Equal(t, tt.statusCode, apiErr.StatusCode)
			assert.Contains(t, apiErr.Message, tt.wantMsg)
		})
	}
}

func TestPlaneClientIsConfigured(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiKey    string
		workspace string
		want      bool
	}{
		{"all set", "http://plane.example.com", "key", "ws", true},
		{"empty base URL", "", "key", "ws", false},
		{"empty api key", "http://plane.example.com", "", "ws", false},
		{"empty workspace", "http://plane.example.com", "key", "", false},
		{"all empty", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(tt.baseURL, tt.apiKey, tt.workspace)
			assert.Equal(t, tt.want, c.IsConfigured())
		})
	}
}

func TestPlaneClientGetWorkItemURL(t *testing.T) {
	c := NewClient("https://plane.example.com", "key", "my-team")
	url := c.GetWorkItemURL("proj-1", "wi-1")
	assert.Equal(t, "https://plane.example.com/my-team/projects/proj-1/work-items/wi-1", url)
}

func TestPlaneClientUpdateConfig(t *testing.T) {
	c := NewClient("http://old.example.com", "old-key", "old-ws")
	assert.True(t, c.IsConfigured())

	c.UpdateConfig("http://new.example.com", "new-key", "new-ws")
	baseURL, apiKey, workspace := c.getConfig()
	assert.Equal(t, "http://new.example.com", baseURL)
	assert.Equal(t, "new-key", apiKey)
	assert.Equal(t, "new-ws", workspace)
}

// === Plan 02-02 Tests: GetWorkItem ===

func TestGetWorkItem(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/projects/proj-abc/work-items/wi-def/")

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(WorkItem{
			ID:          "wi-def",
			Name:        "Fix login page",
			State:       "state-1",
			StateName:   "In Progress",
			StateGroup:  "started",
			Priority:    "high",
			ProjectID:   "proj-abc",
			ProjectName: "Backend",
			Assignees:   []string{"user-1"},
			SequenceID:  42,
		})
	})

	item, err := client.GetWorkItem("proj-abc", "wi-def")
	require.NoError(t, err)
	assert.Equal(t, "wi-def", item.ID)
	assert.Equal(t, "Fix login page", item.Name)
	assert.Equal(t, "In Progress", item.StateName)
	assert.Equal(t, "started", item.StateGroup)
	assert.Equal(t, "high", item.Priority)
	assert.Equal(t, "Backend", item.ProjectName)
	assert.Equal(t, []string{"user-1"}, item.Assignees)
	assert.Equal(t, 42, item.SequenceID)
}

func TestGetWorkItemNotFound(t *testing.T) {
	_, client := testPlaneServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error": "Not found"}`))
	})

	item, err := client.GetWorkItem("proj-abc", "nonexistent-id")
	assert.Nil(t, item)
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok, "expected *APIError, got %T", err)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
}
