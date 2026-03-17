package plane

import "encoding/json"

// WorkItem represents a Plane work item (formerly "issue").
type WorkItem struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description_html,omitempty"`
	State        string   `json:"state"`
	StateName    string   `json:"state__name,omitempty"`
	StateGroup   string   `json:"state__group,omitempty"`
	Priority     string   `json:"priority"`
	ProjectID    string   `json:"project"`
	ProjectName  string   `json:"project__name,omitempty"`
	Assignees    []string `json:"assignees"`
	Labels       []string `json:"labels"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	SequenceID   int      `json:"sequence_id"`
	AssigneeName string   `json:"-"` // Populated by caller after resolving from workspace members
}

// CreateWorkItemRequest is the payload for creating a new work item.
type CreateWorkItemRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description_html,omitempty"`
	State       string   `json:"state,omitempty"`
	Priority    string   `json:"priority,omitempty"`
	Assignees   []string `json:"assignees,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// Project represents a Plane project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
	Network     int    `json:"network"` // 0=secret, 2=public
	CreatedAt   string `json:"created_at"`
}

// State represents a Plane workflow state.
type State struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Color    string  `json:"color"`
	Group    string  `json:"group"` // backlog, unstarted, started, completed, cancelled
	Sequence float64 `json:"sequence"`
}

// Label represents a Plane label.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// MemberWrapper represents a member in workspace/project members API responses.
// Plane API returns flat objects with fields at the top level.
type MemberWrapper struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Role        int    `json:"role"`
}

// PaginatedResponse wraps Plane API paginated results.
type PaginatedResponse struct {
	Results         json.RawMessage `json:"results"`
	TotalCount      int             `json:"total_count"`
	NextCursor      string          `json:"next_cursor"`
	PrevCursor      string          `json:"prev_cursor"`
	NextPageResults bool            `json:"next_page_results"`
	PrevPageResults bool            `json:"prev_page_results"`
}

// APIError represents a Plane API error response.
type APIError struct {
	StatusCode int
	Message    string
	Detail     string
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}
