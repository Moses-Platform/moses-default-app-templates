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

// GetCapability returns a single capability by ID
func GetCapability(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/v1/capabilities/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/capabilities/")
	id := strings.TrimSuffix(path, "/")

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
