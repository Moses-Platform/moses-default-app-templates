package handler

import (
	"encoding/json"
	"net/http"

	"github.com/moses-platform/backend-template/internal/platform"
)

// PlatformHandler demonstrates Moses Platform API usage.
type PlatformHandler struct {
	client *platform.MosesPlatformClient
}

// NewPlatformHandler creates a handler with platform client from env.
func NewPlatformHandler() *PlatformHandler {
	return &PlatformHandler{
		client: platform.NewFromEnv(),
	}
}

// PlatformInfo returns tenant info from the Moses Platform API.
// GET /api/v1/platform/info
func (h *PlatformHandler) PlatformInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if h.client == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"configured": false,
			"message":    "Platform API not configured — approve the moses-platform integration grant to enable",
		})
		return
	}

	tenant, err := h.client.GetTenant(r.Context())
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"configured": true,
			"error":      err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"configured": true,
		"tenant":     tenant,
	})
}
