package handler

import (
	"encoding/json"
	"net/http"

	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

// MosesInfo returns the current Moses context from request headers
func MosesInfo(w http.ResponseWriter, r *http.Request) {
	mosesCtx := middleware.GetMosesContext(r.Context())

	// Determine deployment mode
	deploymentMode := "standalone"
	headersPresent := false
	if mosesCtx.TenantID != "" {
		deploymentMode = "mcp-proxy"
		headersPresent = true
	}

	response := map[string]interface{}{
		"tenant_id":       mosesCtx.TenantID,
		"user_id":         mosesCtx.UserID,
		"chart_id":        mosesCtx.ChartID,
		"tool_id":         mosesCtx.ToolID,
		"request_id":      mosesCtx.RequestID,
		"mcp_source":      mosesCtx.MCPSource,
		"api_key_id":      mosesCtx.APIKeyID,
		"headers_present": headersPresent,
		"deployment_mode": deploymentMode,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
