package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moses-platform/fullstack-showcase/internal/model"
)

// TestGetCapability_RootMount verifies the handler resolves the ID
// correctly when the route is mounted at its bare /api/v1/capabilities/{id}
// path (standalone deploy, no MOSES_BASE_PATH).
func TestGetCapability_RootMount(t *testing.T) {
	known := model.GetAllCapabilities()
	if len(known) == 0 {
		t.Fatal("no capabilities to test against")
	}
	target := known[0]

	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/"+target.ID, nil)
	rec := httptest.NewRecorder()
	GetCapability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/capabilities/%s: expected 200, got %d body=%q", target.ID, rec.Code, rec.Body.String())
	}
	var got model.Capability
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != target.ID {
		t.Errorf("expected ID %q, got %q", target.ID, got.ID)
	}
}

// TestGetCapability_SubPath is the CHAT-pbup.17 regression: at the
// MOSES_BASE_PATH-prefixed mount the URL becomes
// /apps/<tenant>/<slug>/api/v1/capabilities/<id>. The previous
// strings.TrimPrefix(r.URL.Path, "/api/v1/capabilities/") implementation
// returned the literal sub-path-prefixed string and the ID lookup 404'd.
// FAILS on the old code, PASSES on the literal-segment fix.
func TestGetCapability_SubPath(t *testing.T) {
	known := model.GetAllCapabilities()
	if len(known) == 0 {
		t.Fatal("no capabilities to test against")
	}
	target := known[0]

	subPath := "/apps/test-tenant/test-app/api/v1/capabilities/" + target.ID
	req := httptest.NewRequest(http.MethodGet, subPath, nil)
	rec := httptest.NewRecorder()
	GetCapability(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: expected 200 (CHAT-pbup.17), got %d body=%q", subPath, rec.Code, rec.Body.String())
	}
	var got model.Capability
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != target.ID {
		t.Errorf("sub-path mount: expected ID %q, got %q", target.ID, got.ID)
	}
}

// TestGetCapability_TrailingSlash verifies the trailing-slash variant still
// resolves correctly at both mounts.
func TestGetCapability_TrailingSlash(t *testing.T) {
	known := model.GetAllCapabilities()
	if len(known) == 0 {
		t.Fatal("no capabilities to test against")
	}
	target := known[0]

	for _, urlPath := range []string{
		"/api/v1/capabilities/" + target.ID + "/",
		"/apps/t/x/api/v1/capabilities/" + target.ID + "/",
	} {
		req := httptest.NewRequest(http.MethodGet, urlPath, nil)
		rec := httptest.NewRecorder()
		GetCapability(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d", urlPath, rec.Code)
		}
	}
}

// TestGetCapability_Empty404 verifies the missing-ID error path still
// triggers (the literal-segment lookup must not silently 200 when there is
// no ID after the segment).
func TestGetCapability_Empty404(t *testing.T) {
	for _, urlPath := range []string{
		"/api/v1/capabilities/",
		"/apps/t/x/api/v1/capabilities/",
	} {
		req := httptest.NewRequest(http.MethodGet, urlPath, nil)
		rec := httptest.NewRecorder()
		GetCapability(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: expected 400 (empty ID), got %d", urlPath, rec.Code)
		}
	}
}
