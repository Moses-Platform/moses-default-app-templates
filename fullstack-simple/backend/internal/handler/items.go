package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Item represents a simple data entry scoped to a tenant.
type Item struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

// ItemsHandler provides in-memory CRUD for items, scoped per tenant.
// For database-backed CRUD patterns, see the fullstack-showcase template
type ItemsHandler struct {
	mu    sync.RWMutex
	store map[string][]Item // tenant_id -> items
}

// NewItemsHandler creates a new items handler with an empty in-memory store.
func NewItemsHandler() *ItemsHandler {
	return &ItemsHandler{
		store: make(map[string][]Item),
	}
}

// Handle routes requests to /api/v1/items (no trailing ID).
func (h *ItemsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.create(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleWithID routes requests to /api/v1/items/{id}.
func (h *ItemsHandler) HandleWithID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodDelete:
		h.delete(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ItemsHandler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Moses-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	h.mu.RLock()
	items := h.store[tenantID]
	h.mu.RUnlock()

	if items == nil {
		items = []Item{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *ItemsHandler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Moses-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
		return
	}

	item := Item{
		ID:        generateUUID(),
		Title:     strings.TrimSpace(body.Title),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	h.mu.Lock()
	h.store[tenantID] = append(h.store[tenantID], item)
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (h *ItemsHandler) delete(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Header.Get("X-Moses-Tenant-ID")
	if tenantID == "" {
		tenantID = "default"
	}

	// Extract ID from path: /api/v1/items/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/items/")
	if id == "" {
		http.Error(w, `{"error":"item ID is required"}`, http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	items := h.store[tenantID]
	found := false
	for i, item := range items {
		if item.ID == id {
			h.store[tenantID] = append(items[:i], items[i+1:]...)
			found = true
			break
		}
	}
	h.mu.Unlock()

	if !found {
		http.Error(w, `{"error":"item not found"}`, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// generateUUID creates a random UUID v4 without external dependencies.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
