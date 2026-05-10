package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moses-platform/fullstack-simple/internal/config"
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

// CHAT-pxeo.12: in-memory store has no persistent state across restarts,
// but the storage key contract is the same as the persistent templates:
// reads/writes use config.SelfTenantID(); the X-Moses-Tenant-ID header
// is caller-context only.

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

// enforceTenantMatch returns true when it has written a 403 response. Caller
// MUST stop processing on a true return. Body intentionally omits UUIDs.
func enforceTenantMatch(w http.ResponseWriter, r *http.Request) bool {
	if !strictTenantCheckEnabled() {
		return false
	}
	caller := strings.TrimSpace(r.Header.Get("X-Moses-Tenant-ID"))
	if caller == "" || caller == config.SelfTenantID() {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`))
	return true
}

func (h *ItemsHandler) list(w http.ResponseWriter, r *http.Request) {
	if enforceTenantMatch(w, r) {
		return
	}
	tenantID := config.SelfTenantID()

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
	if enforceTenantMatch(w, r) {
		return
	}
	tenantID := config.SelfTenantID()

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
	if enforceTenantMatch(w, r) {
		return
	}
	tenantID := config.SelfTenantID()

	// Extract ID from path: /api/v1/items/{id}.
	// CHAT-pbup.17: derive ID via the literal segment (not TrimPrefix
	// against r.URL.Path) so both the root mount and the MOSES_BASE_PATH
	// mount produce the same ID. TrimPrefix-against-literal silently
	// returned the sub-path-prefixed string at the sub-path mount.
	const segment = "/api/v1/items/"
	idx := strings.Index(r.URL.Path, segment)
	id := ""
	if idx >= 0 {
		id = strings.TrimSuffix(r.URL.Path[idx+len(segment):], "/")
	}
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
