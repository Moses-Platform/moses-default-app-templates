package handler

import (
	"encoding/json"
	"net/http"

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

// ListItems handles GET /api/v1/items
// Returns all items, filtered by tenant when X-Moses-Tenant-ID header is present.
func (h *ItemHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	mosesCtx := middleware.GetMosesContext(r)

	var items []model.Item
	if mosesCtx.TenantID != "" {
		// Filter by tenant when Moses headers are present
		items = h.store.GetByTenant(mosesCtx.TenantID)
	} else {
		// Return all items for local development (no tenant filtering)
		items = h.store.GetAll()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// GetItem handles GET /api/v1/items/{id}
// Returns a single item by ID, with tenant validation when Moses headers are present.
func (h *ItemHandler) GetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Item ID is required", http.StatusBadRequest)
		return
	}

	mosesCtx := middleware.GetMosesContext(r)

	item, found := h.store.GetByID(id)
	if !found {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	// Enforce tenant isolation when Moses headers are present
	if mosesCtx.TenantID != "" && item.TenantID != mosesCtx.TenantID {
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
// Creates a new item and associates it with the tenant from Moses headers.
func (h *ItemHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
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

	mosesCtx := middleware.GetMosesContext(r)

	// Create item with tenant ID from Moses headers
	item := h.store.Add(req.Name, req.Description, mosesCtx.TenantID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(item); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
