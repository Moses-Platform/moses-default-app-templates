package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/moses-platform/fullstack-showcase/internal/model"
)

// ListCapabilities returns all Moses platform capabilities
func ListCapabilities(w http.ResponseWriter, r *http.Request) {
	capabilities := model.GetAllCapabilities()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(capabilities)
}

// GetCapability returns a single capability by ID. The route is mounted
// twice when MOSES_BASE_PATH is set (root + sub-path) — extract the ID by
// trimming everything up to and including the literal "/api/v1/capabilities/"
// segment regardless of any leading sub-path prefix.
//
// CHAT-pbup.17: previous implementation TrimPrefix'd the literal
// "/api/v1/capabilities/" against r.URL.Path. When mounted at
// /apps/<tenant>/<slug>/api/v1/capabilities/<id> the literal didn't match
// the sub-path-prefixed URL and the parsed ID was wrong. Use Index to find
// the segment instead so both mounts produce the same ID.
func GetCapability(w http.ResponseWriter, r *http.Request) {
	const segment = "/api/v1/capabilities/"
	idx := strings.Index(r.URL.Path, segment)
	if idx < 0 {
		http.Error(w, "Capability ID required", http.StatusBadRequest)
		return
	}
	id := strings.TrimSuffix(r.URL.Path[idx+len(segment):], "/")

	if id == "" {
		http.Error(w, "Capability ID required", http.StatusBadRequest)
		return
	}

	capability := model.GetCapabilityByID(id)
	if capability == nil {
		http.Error(w, "Capability not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(capability)
}
