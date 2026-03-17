package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
	"github.com/klab/mattermost-plugin-mcc/server/store"
)

// selectOption represents an option in a Mattermost dialog dynamic select response.
type selectOption struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

// initAPI sets up HTTP routes for the plugin on the gorilla/mux router.
// Mattermost strips the /plugins/{pluginID} prefix before calling ServeHTTP,
// so routes are registered without that prefix.
func (p *Plugin) initAPI() {
	s := p.router.PathPrefix("/api/v1").Subrouter()

	// Dynamic select data sources for dialogs
	s.HandleFunc("/select/projects", p.mattermostAuthMiddleware(p.handleSelectProjects)).Methods("GET")
	s.HandleFunc("/select/members", p.mattermostAuthMiddleware(p.handleSelectMembers)).Methods("GET")
	s.HandleFunc("/select/labels", p.mattermostAuthMiddleware(p.handleSelectLabels)).Methods("GET")

	// Dialog submission handlers
	s.HandleFunc("/dialog/obsidian-setup", p.handleObsidianSetupDialog).Methods("POST")
	s.HandleFunc("/dialog/create-task", p.handleCreateTaskDialog).Methods("POST")
}

// mattermostAuthMiddleware validates that the request comes from an authenticated
// Mattermost user by checking the Mattermost-User-Id header.
func (p *Plugin) mattermostAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-Id")
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}
		next(w, r)
	}
}

// handleSelectProjects returns projects for dialog dynamic select.
// Response format: [{text: "Project Name", value: "project-uuid"}, ...]
func (p *Plugin) handleSelectProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := p.planeClient.ListProjects()
	if err != nil {
		p.API.LogError("Failed to list projects for select", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "Failed to fetch projects")
		return
	}

	options := make([]selectOption, 0, len(projects))
	for _, proj := range projects {
		options = append(options, selectOption{
			Text:  proj.Name,
			Value: proj.ID,
		})
	}

	writeJSON(w, http.StatusOK, options)
}

// handleSelectMembers returns project members for dialog dynamic select.
// Requires project_id query parameter.
func (p *Plugin) handleSelectMembers(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	members, err := p.planeClient.ListProjectMembers(projectID)
	if err != nil {
		p.API.LogError("Failed to list members for select", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "Failed to fetch members")
		return
	}

	options := make([]selectOption, 0, len(members))
	for _, m := range members {
		displayName := m.Member.DisplayName
		if displayName == "" {
			displayName = m.Member.Email
		}
		options = append(options, selectOption{
			Text:  displayName,
			Value: m.Member.ID,
		})
	}

	writeJSON(w, http.StatusOK, options)
}

// handleSelectLabels returns project labels for dialog dynamic select.
// Requires project_id query parameter.
func (p *Plugin) handleSelectLabels(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	labels, err := p.planeClient.ListProjectLabels(projectID)
	if err != nil {
		p.API.LogError("Failed to list labels for select", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "Failed to fetch labels")
		return
	}

	options := make([]selectOption, 0, len(labels))
	for _, label := range labels {
		options = append(options, selectOption{
			Text:  label.Name,
			Value: label.ID,
		})
	}

	writeJSON(w, http.StatusOK, options)
}

// handleObsidianSetupDialog processes the Obsidian setup dialog submission.
// Validates port, saves config to KV store, and sends ephemeral confirmation.
func (p *Plugin) handleObsidianSetupDialog(w http.ResponseWriter, r *http.Request) {
	var request model.SubmitDialogRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	host, _ := request.Submission["host"].(string)
	portStr, _ := request.Submission["port"].(string)
	apiKey, _ := request.Submission["api_key"].(string)

	// Validate port is numeric and positive
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		// Return validation error in dialog format
		writeJSON(w, http.StatusOK, map[string]map[string]string{
			"errors": {
				"port": "Port must be a positive number.",
			},
		})
		return
	}

	if host == "" {
		host = "127.0.0.1"
	}

	// Save to store
	cfg := &store.ObsidianConfig{
		Host:    host,
		Port:    port,
		APIKey:  apiKey,
		SetupAt: time.Now().Unix(),
	}
	if err := p.store.SaveObsidianConfig(request.UserId, cfg); err != nil {
		p.API.LogError("Failed to save Obsidian config", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}

	// Send ephemeral confirmation
	p.sendEphemeral(request.UserId, request.ChannelId,
		fmt.Sprintf("Obsidian REST API configured! Host: %s:%d", host, port))

	// Return 200 with empty JSON to dismiss dialog
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// handleCreateTaskDialog processes the task creation dialog submission.
// Validates fields, resolves label names to IDs, creates work item via Plane API,
// and sends an ephemeral confirmation.
func (p *Plugin) handleCreateTaskDialog(w http.ResponseWriter, r *http.Request) {
	var request model.SubmitDialogRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	title, _ := request.Submission["title"].(string)
	description, _ := request.Submission["description"].(string)
	projectID, _ := request.Submission["project_id"].(string)
	priority, _ := request.Submission["priority"].(string)
	assigneeID, _ := request.Submission["assignee_id"].(string)
	labelsText, _ := request.Submission["labels"].(string)

	// Validate title
	title = strings.TrimSpace(title)
	if title == "" {
		writeJSON(w, http.StatusOK, map[string]map[string]string{
			"errors": {
				"title": "Title is required",
			},
		})
		return
	}

	// Resolve label names to IDs
	var labelIDs []string
	if labelsText = strings.TrimSpace(labelsText); labelsText != "" {
		labelNames := strings.Split(labelsText, ",")
		projectLabels, err := p.planeClient.ListProjectLabels(projectID)
		if err != nil {
			p.API.LogWarn("Failed to fetch labels for resolution", "error", err.Error())
		} else {
			for _, name := range labelNames {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				matched := false
				for _, label := range projectLabels {
					if strings.EqualFold(label.Name, name) {
						labelIDs = append(labelIDs, label.ID)
						matched = true
						break
					}
				}
				if !matched {
					p.API.LogWarn("Label not found, skipping", "label", name, "project", projectID)
				}
			}
		}
	}

	// Build request
	var assignees []string
	if assigneeID != "" {
		assignees = []string{assigneeID}
	}
	if priority == "" {
		priority = "none"
	}

	req := &plane.CreateWorkItemRequest{
		Name:        title,
		Description: description,
		Priority:    priority,
		Assignees:   assignees,
		Labels:      labelIDs,
	}

	workItem, err := p.planeClient.CreateWorkItem(projectID, req)
	if err != nil {
		p.API.LogError("Failed to create work item from dialog", "error", err.Error())
		p.sendEphemeral(request.UserId, request.ChannelId,
			"Error al comunicarse con Plane: "+err.Error()+". Intenta de nuevo.")
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}

	// Find project name from cached projects
	projectName := projectID
	projects, err := p.planeClient.ListProjects()
	if err == nil {
		for _, proj := range projects {
			if proj.ID == projectID {
				projectName = proj.Name
				break
			}
		}
	}

	workItemURL := p.planeClient.GetWorkItemURL(projectID, workItem.ID)
	msg := formatTaskCreatedMessage(title, projectName, workItemURL)
	p.sendEphemeral(request.UserId, request.ChannelId, msg)

	// Return 200 with empty JSON to dismiss dialog
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// writeJSON marshals data as JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
