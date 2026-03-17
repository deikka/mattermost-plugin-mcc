package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// MockPlaneServer wraps an httptest.Server that simulates the Plane API.
// It provides configurable responses for all endpoints needed by the plugin.
type MockPlaneServer struct {
	Server *httptest.Server

	mu        sync.RWMutex
	responses map[string]mockResponse
	apiKey    string
}

type mockResponse struct {
	statusCode int
	body       interface{}
}

// NewMockPlaneServer creates and starts an httptest.Server that handles
// Plane API endpoints with test data. The server validates the X-API-Key
// header on all requests and returns 401 if missing or incorrect.
//
// Default endpoints:
//   - GET /api/v1/workspaces/test-workspace/projects/ -> 2 test projects
//   - GET /api/v1/workspaces/test-workspace/members/ -> 2 test workspace members
//   - GET /api/v1/workspaces/test-workspace/projects/{pid}/states/ -> 3 test states
//   - GET /api/v1/workspaces/test-workspace/projects/{pid}/labels/ -> 2 test labels
//   - GET /api/v1/workspaces/test-workspace/projects/{pid}/members/ -> 2 test members
//   - GET /api/v1/workspaces/test-workspace/projects/{pid}/work-items/ -> 2 test work items
//   - POST /api/v1/workspaces/test-workspace/projects/{pid}/work-items/ -> created work item
//   - Default: 404 for unmatched paths
func NewMockPlaneServer(t *testing.T) *MockPlaneServer {
	t.Helper()

	mock := &MockPlaneServer{
		responses: make(map[string]mockResponse),
		apiKey:    TestPlaneAPIKey,
	}

	// Set up default responses
	mock.setDefaultResponses()

	mux := http.NewServeMux()
	mux.HandleFunc("/", mock.handler)

	mock.Server = httptest.NewServer(mux)
	t.Cleanup(func() {
		mock.Server.Close()
	})

	return mock
}

// URL returns the base URL of the mock server, suitable for use as PlaneURL.
func (m *MockPlaneServer) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockPlaneServer) Close() {
	m.Server.Close()
}

// SetResponse allows tests to override specific endpoint responses.
// The path should match the full API path (e.g., "/api/v1/workspaces/test-workspace/projects/").
// The method should be "GET", "POST", etc. Use "GET:/path" as the key format.
func (m *MockPlaneServer) SetResponse(method, path string, statusCode int, body interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := method + ":" + path
	m.responses[key] = mockResponse{statusCode: statusCode, body: body}
}

// SetAPIKey changes the expected API key for authentication.
func (m *MockPlaneServer) SetAPIKey(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apiKey = key
}

func (m *MockPlaneServer) handler(w http.ResponseWriter, r *http.Request) {
	// Validate API key
	m.mu.RLock()
	expectedKey := m.apiKey
	m.mu.RUnlock()

	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" || apiKey != expectedKey {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]string{"error": "Invalid API key"})
		return
	}

	// Check for exact match first, then pattern match
	m.mu.RLock()
	exactKey := r.Method + ":" + r.URL.Path
	resp, found := m.responses[exactKey]
	if !found {
		// Try pattern matching for project-scoped endpoints
		resp, found = m.findPatternMatch(r.Method, r.URL.Path)
	}
	m.mu.RUnlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		writeJSON(w, map[string]string{"error": "Not found"})
		return
	}

	w.WriteHeader(resp.statusCode)
	writeJSON(w, resp.body)
}

func (m *MockPlaneServer) findPatternMatch(method, path string) (mockResponse, bool) {
	ws := TestPlaneWorkspace
	prefix := fmt.Sprintf("/api/v1/workspaces/%s/projects/", ws)

	if !strings.HasPrefix(path, prefix) {
		return mockResponse{}, false
	}

	// Extract what comes after /projects/{pid}/
	remainder := path[len(prefix):]
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) < 2 {
		return mockResponse{}, false
	}

	// parts[0] is the project ID, parts[1] is the sub-resource
	subResource := parts[1]

	// Look for a wildcard pattern match
	patternKey := method + ":" + prefix + "{pid}/" + subResource
	resp, found := m.responses[patternKey]
	return resp, found
}

func (m *MockPlaneServer) setDefaultResponses() {
	ws := TestPlaneWorkspace

	// GET /api/v1/workspaces/{ws}/projects/
	m.responses["GET:/api/v1/workspaces/"+ws+"/projects/"] = mockResponse{
		statusCode: http.StatusOK,
		body: &resultsWrapper{Results: []map[string]interface{}{
			{
				"id":          "proj-uuid-001",
				"name":        "Backend",
				"identifier":  "BACK",
				"description": "Backend services project",
			},
			{
				"id":          "proj-uuid-002",
				"name":        "Frontend",
				"identifier":  "FRNT",
				"description": "Frontend application project",
			},
		}},
	}

	// GET /api/v1/workspaces/{ws}/members/
	m.responses["GET:/api/v1/workspaces/"+ws+"/members/"] = mockResponse{
		statusCode: http.StatusOK,
		body: []map[string]interface{}{
			{
				"id":           "user-uuid-001",
				"email":        "alice@example.com",
				"display_name": "Alice Smith",
				"first_name":   "Alice",
				"last_name":    "Smith",
				"role":         20,
			},
			{
				"id":           "user-uuid-002",
				"email":        "bob@example.com",
				"display_name": "Bob Johnson",
				"first_name":   "Bob",
				"last_name":    "Johnson",
				"role":         15,
			},
		},
	}

	// Pattern-matched project-scoped endpoints
	prefix := "/api/v1/workspaces/" + ws + "/projects/{pid}/"

	// GET .../states/
	m.responses["GET:"+prefix+"states/"] = mockResponse{
		statusCode: http.StatusOK,
		body: &resultsWrapper{Results: []map[string]interface{}{
			{
				"id":       "state-uuid-001",
				"name":     "Backlog",
				"group":    "backlog",
				"color":    "#A3A3A3",
				"sequence": 1000,
			},
			{
				"id":       "state-uuid-002",
				"name":     "In Progress",
				"group":    "started",
				"color":    "#F59E0B",
				"sequence": 2000,
			},
			{
				"id":       "state-uuid-003",
				"name":     "Done",
				"group":    "completed",
				"color":    "#22C55E",
				"sequence": 3000,
			},
		}},
	}

	// GET .../labels/
	m.responses["GET:"+prefix+"labels/"] = mockResponse{
		statusCode: http.StatusOK,
		body: &resultsWrapper{Results: []map[string]interface{}{
			{
				"id":    "label-uuid-001",
				"name":  "bug",
				"color": "#EF4444",
			},
			{
				"id":    "label-uuid-002",
				"name":  "enhancement",
				"color": "#3B82F6",
			},
		}},
	}

	// GET .../members/
	m.responses["GET:"+prefix+"members/"] = mockResponse{
		statusCode: http.StatusOK,
		body: &resultsWrapper{Results: []map[string]interface{}{
			{
				"id":           "user-uuid-001",
				"email":        "alice@example.com",
				"display_name": "Alice Smith",
			},
			{
				"id":           "user-uuid-002",
				"email":        "bob@example.com",
				"display_name": "Bob Johnson",
			},
		}},
	}

	// GET .../work-items/
	m.responses["GET:"+prefix+"work-items/"] = mockResponse{
		statusCode: http.StatusOK,
		body: &resultsWrapper{Results: []map[string]interface{}{
			{
				"id":            "wi-uuid-001",
				"name":          "Fix login bug",
				"description_html": "<p>Login fails on Safari</p>",
				"state":         "state-uuid-002",
				"state__name":   "In Progress",
				"state__group":  "started",
				"priority":      "high",
				"project":       "proj-uuid-001",
				"project__name": "Backend",
				"assignees":     []string{"user-uuid-001"},
				"labels":        []string{"label-uuid-001"},
				"created_at":    "2026-03-15T10:00:00Z",
				"updated_at":    "2026-03-16T14:30:00Z",
			},
			{
				"id":            "wi-uuid-002",
				"name":          "Add dark mode",
				"description_html": "<p>Implement dark mode toggle</p>",
				"state":         "state-uuid-001",
				"state__name":   "Backlog",
				"state__group":  "backlog",
				"priority":      "medium",
				"project":       "proj-uuid-002",
				"project__name": "Frontend",
				"assignees":     []string{"user-uuid-002"},
				"labels":        []string{"label-uuid-002"},
				"created_at":    "2026-03-14T09:00:00Z",
				"updated_at":    "2026-03-15T11:00:00Z",
			},
		}},
	}

	// POST .../work-items/
	m.responses["POST:"+prefix+"work-items/"] = mockResponse{
		statusCode: http.StatusCreated,
		body: map[string]interface{}{
			"id":            "wi-uuid-new",
			"name":          "New task",
			"description_html": "",
			"state":         "state-uuid-001",
			"state__name":   "Backlog",
			"state__group":  "backlog",
			"priority":      "none",
			"project":       "proj-uuid-001",
			"project__name": "Backend",
			"assignees":     []string{},
			"labels":        []string{},
			"created_at":    "2026-03-17T06:00:00Z",
			"updated_at":    "2026-03-17T06:00:00Z",
		},
	}
}

// resultsWrapper wraps response arrays in a {"results": [...]} envelope
// matching the Plane API pagination format.
type resultsWrapper struct {
	Results interface{} `json:"results"`
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
