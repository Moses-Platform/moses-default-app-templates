package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/moses-platform/backend-template/internal/middleware"
	"github.com/moses-platform/backend-template/internal/model"
)

// ItemHandler handles HTTP requests for the Item resource
type ItemHandler struct {
	store *model.ItemStore
}

// NewItemHandler creates a new ItemHandler
func NewItemHandler(store *model.ItemStore) *ItemHandler {
	return &ItemHandler{store: store}
}

// CHAT-pxeo.12: storage tenant_id is deploy-pinned (env). The
// X-Moses-Tenant-ID header is caller-context only and drives the 403
// cross-check on writes.

// strictTenantCheckEnabled gates the 403 cross-check. Default true.
func strictTenantCheckEnabled() bool {
	v := strings.TrimSpace(os.Getenv("MOSES_STRICT_TENANT_CHECK"))
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// enforceTenantMatch returns true when it has written a 403 response.
// Body intentionally omits the actual UUIDs.
func enforceTenantMatch(w http.ResponseWriter, mc *middleware.MosesContext) bool {
	if !strictTenantCheckEnabled() {
		return false
	}
	caller := strings.TrimSpace(mc.CallerTenantID)
	if caller == "" || caller == mc.SelfTenantID {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`))
	return true
}

// ListItems handles GET /api/v1/items
//
// CHAT-pxeo.12: scoped to the deploy-pinned self tenant. The historical
// "no Moses headers → return all items" branch is gone — running without
// MOSES_TENANT_ID falls back to the 'local-dev' sentinel which scopes to
// rows tagged 'local-dev' (the same value the in-memory seed uses for
// pre-populated samples when SelfTenantID() == 'local-dev').
func (h *ItemHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	mosesCtx := middleware.GetMosesContext(r)
	if enforceTenantMatch(w, mosesCtx) {
		return
	}

	items := h.store.GetByTenant(mosesCtx.SelfTenantID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetItem handles GET /api/v1/items/{id}
// Returns a single item by ID, scoped to the deploy-pinned self tenant.
func (h *ItemHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Item ID is required", http.StatusBadRequest)
		return
	}

	mosesCtx := middleware.GetMosesContext(r)
	if enforceTenantMatch(w, mosesCtx) {
		return
	}

	item, found := h.store.GetByID(id)
	if !found {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	// Enforce tenant isolation against the deploy-pinned self tenant.
	if item.TenantID != mosesCtx.SelfTenantID {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(item); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// CreateItemRequest represents the request body for creating an item
type CreateItemRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateItem handles POST /api/v1/items
// Creates a new item and associates it with the deploy-pinned self tenant.
func (h *ItemHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	mosesCtx := middleware.GetMosesContext(r)
	if enforceTenantMatch(w, mosesCtx) {
		return
	}

	var req CreateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	// CHAT-pxeo.12: tenant id from env, NOT header.
	item := h.store.Add(req.Name, req.Description, mosesCtx.SelfTenantID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(item); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
