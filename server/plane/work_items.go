package plane

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// CreateWorkItem creates a new work item in a Plane project.
// Returns the created work item. No caching.
func (c *Client) CreateWorkItem(projectID string, req *CreateWorkItemRequest) (*WorkItem, error) {
	path := fmt.Sprintf("/projects/%s/work-items/", projectID)
	resp, err := c.doRequest("POST", path, req)
	if err != nil {
		return nil, fmt.Errorf("create work item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseAPIError(resp)
	}

	var workItem WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&workItem); err != nil {
		return nil, fmt.Errorf("decode work item response: %w", err)
	}
	return &workItem, nil
}

// ListWorkItems returns work items for a project, optionally filtered by assignee.
// No caching -- always returns fresh data.
func (c *Client) ListWorkItems(projectID, assigneeID string) ([]WorkItem, error) {
	path := fmt.Sprintf("/projects/%s/work-items/?assignees=%s&expand=state_detail,project_detail&per_page=10", projectID, assigneeID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("list work items: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var paginated PaginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginated); err != nil {
		return nil, fmt.Errorf("decode work items response: %w", err)
	}

	var workItems []WorkItem
	if err := json.Unmarshal(paginated.Results, &workItems); err != nil {
		return nil, fmt.Errorf("unmarshal work items: %w", err)
	}

	return workItems, nil
}

// GetWorkItem fetches a single work item by project ID and work item ID.
// Uses expand params to include state and project detail fields.
// No caching -- always returns fresh data.
func (c *Client) GetWorkItem(projectID, workItemID string) (*WorkItem, error) {
	path := fmt.Sprintf("/projects/%s/work-items/%s/?expand=state_detail,project_detail", projectID, workItemID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get work item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var workItem WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&workItem); err != nil {
		return nil, fmt.Errorf("decode work item response: %w", err)
	}
	return &workItem, nil
}

// GetWorkItemBySequence fetches a work item by its sequence ID (e.g. PROJ-42 → sequence 42).
// Lists work items filtered by sequence_id and returns the first match.
func (c *Client) GetWorkItemBySequence(projectID string, sequenceID int) (*WorkItem, error) {
	path := fmt.Sprintf("/projects/%s/work-items/?sequence_id=%d&expand=state_detail,project_detail", projectID, sequenceID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get work item by sequence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var paginated PaginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginated); err != nil {
		return nil, fmt.Errorf("decode work item response: %w", err)
	}

	var workItems []WorkItem
	if err := json.Unmarshal(paginated.Results, &workItems); err != nil {
		return nil, fmt.Errorf("unmarshal work items: %w", err)
	}

	if len(workItems) == 0 {
		return nil, fmt.Errorf("work item %d not found in project %s", sequenceID, projectID)
	}
	return &workItems[0], nil
}

// ListProjectWorkItems returns ALL work items in a project (no assignee filter).
// Used by /task plane status for project-wide counts.
// No caching -- always returns fresh data.
func (c *Client) ListProjectWorkItems(projectID string) ([]WorkItem, error) {
	path := fmt.Sprintf("/projects/%s/work-items/?expand=state_detail&per_page=100", projectID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("list project work items: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var paginated PaginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginated); err != nil {
		return nil, fmt.Errorf("decode project work items response: %w", err)
	}

	var workItems []WorkItem
	if err := json.Unmarshal(paginated.Results, &workItems); err != nil {
		return nil, fmt.Errorf("unmarshal project work items: %w", err)
	}

	return workItems, nil
}
