package plane

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// Cache TTL constants for Plane API data.
const (
	cacheTTLProjects = 5 * time.Minute
	cacheTTLStates   = 10 * time.Minute
	cacheTTLLabels   = 5 * time.Minute
	cacheTTLMembers  = 5 * time.Minute
)

// ListProjects returns all projects in the workspace.
// Results are cached for 5 minutes.
func (c *Client) ListProjects() ([]Project, error) {
	cacheKey := "projects"
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]Project), nil
	}

	resp, err := c.doRequest("GET", "/projects/", nil)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var paginated PaginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginated); err != nil {
		return nil, fmt.Errorf("decode projects response: %w", err)
	}

	var projects []Project
	if err := json.Unmarshal(paginated.Results, &projects); err != nil {
		return nil, fmt.Errorf("unmarshal projects: %w", err)
	}

	c.cache.Set(cacheKey, projects, cacheTTLProjects)
	return projects, nil
}

// ListProjectStates returns all workflow states for a project, sorted by sequence.
// Results are cached for 10 minutes.
func (c *Client) ListProjectStates(projectID string) ([]State, error) {
	cacheKey := "states_" + projectID
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]State), nil
	}

	path := fmt.Sprintf("/projects/%s/states/", projectID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("list project states: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var paginated PaginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginated); err != nil {
		return nil, fmt.Errorf("decode states response: %w", err)
	}

	var states []State
	if err := json.Unmarshal(paginated.Results, &states); err != nil {
		return nil, fmt.Errorf("unmarshal states: %w", err)
	}

	// Sort by sequence for correct ordering
	sort.Slice(states, func(i, j int) bool {
		return states[i].Sequence < states[j].Sequence
	})

	c.cache.Set(cacheKey, states, cacheTTLStates)
	return states, nil
}

// ListProjectLabels returns all labels for a project.
// Results are cached for 5 minutes.
func (c *Client) ListProjectLabels(projectID string) ([]Label, error) {
	cacheKey := "labels_" + projectID
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]Label), nil
	}

	path := fmt.Sprintf("/projects/%s/labels/", projectID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("list project labels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var paginated PaginatedResponse
	if err := json.NewDecoder(resp.Body).Decode(&paginated); err != nil {
		return nil, fmt.Errorf("decode labels response: %w", err)
	}

	var labels []Label
	if err := json.Unmarshal(paginated.Results, &labels); err != nil {
		return nil, fmt.Errorf("unmarshal labels: %w", err)
	}

	c.cache.Set(cacheKey, labels, cacheTTLLabels)
	return labels, nil
}

// ListProjectMembers returns all members of a project.
// Results are cached for 5 minutes.
func (c *Client) ListProjectMembers(projectID string) ([]MemberWrapper, error) {
	cacheKey := "members_" + projectID
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]MemberWrapper), nil
	}

	path := fmt.Sprintf("/projects/%s/members/", projectID)
	resp, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("list project members: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	// Project members return as a direct array (not paginated)
	var members []MemberWrapper
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return nil, fmt.Errorf("decode project members: %w", err)
	}

	c.cache.Set(cacheKey, members, cacheTTLMembers)
	return members, nil
}

// ListWorkspaceMembers returns all members of the workspace.
// Used by /task connect for email matching.
// Results are cached for 5 minutes.
func (c *Client) ListWorkspaceMembers() ([]MemberWrapper, error) {
	cacheKey := "ws_members"
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.([]MemberWrapper), nil
	}

	resp, err := c.doRequest("GET", "/members/", nil)
	if err != nil {
		return nil, fmt.Errorf("list workspace members: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	// Workspace members may return as a direct array (not paginated)
	var members []MemberWrapper
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		return nil, fmt.Errorf("decode workspace members: %w", err)
	}

	c.cache.Set(cacheKey, members, cacheTTLMembers)
	return members, nil
}
