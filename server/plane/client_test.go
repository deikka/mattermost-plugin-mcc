package plane

import "testing"

func TestPlaneClientDoRequest(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify URL construction and auth headers")
}

func TestPlaneClientListProjects(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify project list parsing and caching")
}

func TestPlaneClientListWorkItems(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify work items query with assignee filter")
}

func TestPlaneClientCreateWorkItem(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify work item creation with correct payload")
}

func TestPlaneClientCache(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify TTL cache hit/miss/expiration")
}

func TestPlaneClientErrorHandling(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify API error parsing for 401, 404, 429")
}

func TestPlaneClientIsConfigured(t *testing.T) {
	t.Skip("TODO: implement after Plan 01-02 — verify IsConfigured returns false when fields empty")
}
