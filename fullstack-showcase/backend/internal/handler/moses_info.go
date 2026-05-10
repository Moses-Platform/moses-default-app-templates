package handler

import (
	"encoding/json"
	"net/http"

	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

// MosesInfo returns the current Moses context from request headers.
//
// CHAT-pxeo.12: distinguishes self_tenant_id (deploy-pinned, used for
// storage) from caller_tenant_id (audit/header value). The
// deployment_mode flag still reflects whether ANY caller header is
// present, which is the historical signal for "running behind the
// platform proxy".
func MosesInfo(w http.ResponseWriter, r *http.Request) {
	mosesCtx := middleware.GetMosesContext(r.Context())

	// Determine deployment mode
	deploymentMode := "standalone"
	headersPresent := false
	if mosesCtx.CallerTenantID != "" {
		deploymentMode = "mcp-proxy"
		headersPresent = true
	}

	response := map[string]interface{}{
		"self_tenant_id":   mosesCtx.SelfTenantID,
		"caller_tenant_id": mosesCtx.CallerTenantID,
		"user_id":          mosesCtx.UserID,
		"chart_id":         mosesCtx.ChartID,
		"tool_id":          mosesCtx.ToolID,
		"request_id":       mosesCtx.RequestID,
		"mcp_source":       mosesCtx.MCPSource,
		"api_key_id":       mosesCtx.APIKeyID,
		"headers_present":  headersPresent,
		"deployment_mode":  deploymentMode,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
