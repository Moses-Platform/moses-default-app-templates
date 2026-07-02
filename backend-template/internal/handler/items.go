package handler

import (
	"encoding/json"
	"net/http"

	"github.com/moses-platform/backend-template/internal/middleware"
	"github.com/moses-platform/backend-template/internal/model"
)

// DEMO code — the Item CRUD below is example content removed by
// ./clean_out_template.sh. The load-bearing tenant helpers it uses
// (strictTenantCheckEnabled / enforceTenantMatch) live in tenant.go and
// survive the cleanout.

// ItemHandler handles HTTP requests for the Item resource
type ItemHandler struct {
	store *model.ItemStore
}

// NewItemHandler creates a new ItemHandler
func NewItemHandler(store *model.ItemStore) *ItemHandler {
	return &ItemHandler{store: store}
}

// ListItems handles GET /api/v1/items
//
// CHAT-pxeo.12: scoped to the deploy-pinned self tenant. The historical
// "no Moses headers → return all items" branch is gone. The in-memory
// seed rows are tagged with config.SelfTenantID() at store construction
// (model.NewItemStore), so this endpoint returns the samples both on a
// deployed pod (MOSES_TENANT_ID set) and in a local run (the 'local-dev'
// sentinel).
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
