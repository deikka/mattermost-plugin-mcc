package plane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client is the HTTP client for the Plane API.
type Client struct {
	mu         sync.RWMutex
	baseURL    string
	apiKey     string
	workspace  string
	httpClient *http.Client
	cache      *Cache
}

// NewClient creates a new Plane API client.
// The baseURL trailing slash is trimmed. A 10-second HTTP timeout is set.
// An in-memory cache is initialized.
func NewClient(baseURL, apiKey, workspace string) *Client {
	return &Client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		workspace: workspace,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: NewCache(),
	}
}

// IsConfigured returns true if the client has all required configuration fields set.
func (c *Client) IsConfigured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL != "" && c.apiKey != "" && c.workspace != ""
}

// UpdateConfig updates the client's connection settings. Thread-safe.
func (c *Client) UpdateConfig(baseURL, apiKey, workspace string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.apiKey = apiKey
	c.workspace = workspace
}

// InvalidateCache clears all cached data. Useful when configuration changes.
func (c *Client) InvalidateCache() {
	c.cache.InvalidateAll()
}

// getConfig returns the current config values under lock.
func (c *Client) getConfig() (baseURL, apiKey, workspace string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL, c.apiKey, c.workspace
}

// doRequest constructs and executes an authenticated HTTP request to the Plane API.
// The URL is built as: {baseURL}/api/v1/workspaces/{workspace}{path}
// The X-API-Key header is set for authentication. Body is JSON-marshaled if non-nil.
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	baseURL, apiKey, workspace := c.getConfig()

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	url := fmt.Sprintf("%s/api/v1/workspaces/%s%s", baseURL, workspace, path)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// parseAPIError reads the response body and returns a structured API error.
func parseAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(resp.Body)
	message := http.StatusText(resp.StatusCode)
	detail := ""

	if len(body) > 0 {
		var errResp map[string]interface{}
		if err := json.Unmarshal(body, &errResp); err == nil {
			if msg, ok := errResp["error"].(string); ok {
				message = msg
			}
			if d, ok := errResp["detail"].(string); ok {
				detail = d
			}
		}
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    message,
		Detail:     detail,
	}
}

// GetWorkItemURL constructs a browser-accessible URL for a work item.
func (c *Client) GetWorkItemURL(projectID, workItemID string) string {
	baseURL, _, workspace := c.getConfig()
	return fmt.Sprintf("%s/%s/projects/%s/work-items/%s", baseURL, workspace, projectID, workItemID)
}
