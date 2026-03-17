package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/klab/mattermost-plugin-mcc/server/plane"
)

// === Plan 02-02 Tests: URL Extraction ===

func TestExtractPlaneURLsSingle(t *testing.T) {
	msg := "Check https://plane.example.com/ws/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456 please"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	require.Len(t, matches, 1)
	assert.Equal(t, "abc00000-0000-0000-0000-000000000123", matches[0].ProjectID)
	assert.Equal(t, "def00000-0000-0000-0000-000000000456", matches[0].WorkItemID)
}

func TestExtractPlaneURLsMultiple(t *testing.T) {
	msg := "See https://plane.example.com/ws/projects/11111111-1111-1111-1111-111111111111/work-items/22222222-2222-2222-2222-222222222222 and also https://plane.example.com/ws/projects/33333333-3333-3333-3333-333333333333/work-items/44444444-4444-4444-4444-444444444444"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	require.Len(t, matches, 2)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", matches[0].ProjectID)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", matches[0].WorkItemID)
	assert.Equal(t, "33333333-3333-3333-3333-333333333333", matches[1].ProjectID)
	assert.Equal(t, "44444444-4444-4444-4444-444444444444", matches[1].WorkItemID)
}

func TestExtractPlaneURLsNoMatch(t *testing.T) {
	msg := "This is a normal message with no plane URLs https://google.com"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	assert.Empty(t, matches)
}

func TestExtractPlaneURLsPartialMatch(t *testing.T) {
	// URL missing work-items segment
	msg := "See https://plane.example.com/ws/projects/abc00000-0000-0000-0000-000000000123/settings"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com", "ws")
	assert.Empty(t, matches)
}

func TestExtractPlaneURLsTrailingSlash(t *testing.T) {
	// Plane URL with trailing slash should still work
	msg := "Check https://plane.example.com/ws/projects/abc00000-0000-0000-0000-000000000123/work-items/def00000-0000-0000-0000-000000000456"
	matches := extractPlaneWorkItemURLs(msg, "https://plane.example.com/", "ws")
	require.Len(t, matches, 1)
	assert.Equal(t, "abc00000-0000-0000-0000-000000000123", matches[0].ProjectID)
}

// === Plan 02-02 Tests: Attachment Builder ===

func TestBuildWorkItemAttachment(t *testing.T) {
	item := &plane.WorkItem{
		ID:          "wi-1",
		Name:        "Fix login page",
		StateName:   "In Progress",
		StateGroup:  "started",
		Priority:    "high",
		ProjectName: "Backend",
	}
	item.AssigneeName = "Alice Smith"

	attachment := buildWorkItemAttachment(item, "https://plane.example.com", "my-ws", "proj-1")

	assert.Equal(t, "Fix login page", attachment.Title)
	assert.Equal(t, "https://plane.example.com/my-ws/projects/proj-1/work-items/wi-1", attachment.TitleLink)
	assert.Equal(t, "#3f76ff", attachment.Color)
	assert.Equal(t, "Plane", attachment.Footer)

	// Verify fields
	require.Len(t, attachment.Fields, 4) // Status, Priority, Assigned, Project
	assert.Equal(t, "Status", attachment.Fields[0].Title)
	assert.Contains(t, attachment.Fields[0].Value, "In Progress")
	assert.Contains(t, attachment.Fields[0].Value, ":large_blue_circle:")
	assert.Equal(t, "Priority", attachment.Fields[1].Title)
	assert.Contains(t, attachment.Fields[1].Value, "High")
	assert.Equal(t, "Assigned", attachment.Fields[2].Title)
	assert.Equal(t, "Alice Smith", attachment.Fields[2].Value)
	assert.Equal(t, "Project", attachment.Fields[3].Title)
	assert.Equal(t, "Backend", attachment.Fields[3].Value)
}

func TestBuildWorkItemAttachmentNoAssignee(t *testing.T) {
	item := &plane.WorkItem{
		ID:          "wi-1",
		Name:        "Unassigned task",
		StateName:   "Todo",
		StateGroup:  "unstarted",
		Priority:    "none",
		ProjectName: "Frontend",
	}

	attachment := buildWorkItemAttachment(item, "https://plane.example.com", "my-ws", "proj-1")

	// Without assignee, should have 3 fields: Status, Priority, Project
	require.Len(t, attachment.Fields, 3)
	assert.Equal(t, "Status", attachment.Fields[0].Title)
	assert.Equal(t, "Priority", attachment.Fields[1].Title)
	assert.Equal(t, "Project", attachment.Fields[2].Title)
}
