package plane

import (
	"net/http"
	"strings"
	"time"
)

// Client is the HTTP client for the Plane API.
type Client struct {
	BaseURL    string
	APIKey     string
	Workspace  string
	HTTPClient *http.Client
}

// NewClient creates a new Plane API client.
func NewClient(baseURL, apiKey, workspace string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		APIKey:    apiKey,
		Workspace: workspace,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsConfigured returns true if the client has all required configuration fields set.
func (c *Client) IsConfigured() bool {
	return c.BaseURL != "" && c.APIKey != "" && c.Workspace != ""
}
