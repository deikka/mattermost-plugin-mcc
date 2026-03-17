package plane

// Project represents a Plane project.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
}

// WorkItem represents a Plane work item (formerly "issue").
type WorkItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description_html"`
	State       string   `json:"state"`
	StateName   string   `json:"state__name"`
	StateGroup  string   `json:"state__group"`
	Priority    string   `json:"priority"`
	ProjectID   string   `json:"project"`
	ProjectName string   `json:"project__name"`
	Assignees   []string `json:"assignees"`
	Labels      []string `json:"labels"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
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

// State represents a Plane workflow state.
type State struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Group    string `json:"group"`
	Color    string `json:"color"`
	Sequence int    `json:"sequence"`
}

// Label represents a Plane label.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Member represents a workspace or project member.
type Member struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Role        int    `json:"role"`
}

// WorkspaceMember wraps a member in the workspace members response format.
type WorkspaceMember struct {
	Member Member `json:"member"`
}
