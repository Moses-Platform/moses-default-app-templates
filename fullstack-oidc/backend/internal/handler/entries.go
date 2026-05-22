package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/moses-platform/fullstack-oidc/internal/config"
	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

// EntriesHandler serves the per-user `entries` demo resource. Data is
// scoped by BOTH the deploy-pinned tenant id (config.SelfTenantID) and
// the authenticated OIDC subject — showing how the validated identity
// from OIDCAuth feeds row-level ownership.
type EntriesHandler struct {
	db *sql.DB
}

// NewEntriesHandler builds an EntriesHandler.
func NewEntriesHandler(db *sql.DB) *EntriesHandler {
	return &EntriesHandler{db: db}
}

type entry struct {
	ID        string `json:"id"`
	OwnerSub  string `json:"owner_sub"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// Entries dispatches GET (list) and POST (create). The route is
// protected by OIDCAuth, so id.Authenticated is always true here.
func (h *EntriesHandler) Entries(w http.ResponseWriter, r *http.Request) {
	id := oidcauth.IdentityFrom(r.Context())
	if !id.Authenticated || id.Subject == "" {
		// Defensive: the middleware should have gated this already.
		http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
		return
	}
	tenant := config.SelfTenantID()

	switch r.Method {
	case http.MethodGet:
		h.list(w, r, tenant, id.Subject)
	case http.MethodPost:
		h.create(w, r, tenant, id.Subject)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *EntriesHandler) list(w http.ResponseWriter, r *http.Request, tenant, sub string) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, owner_sub, body, created_at
		   FROM entries
		  WHERE tenant_id = $1 AND owner_sub = $2
		  ORDER BY created_at DESC`,
		tenant, sub)
	if err != nil {
		http.Error(w, `{"error":"query_failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := []entry{}
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.OwnerSub, &e.Body, &e.CreatedAt); err != nil {
			http.Error(w, `{"error":"scan_failed"}`, http.StatusInternalServerError)
			return
		}
		out = append(out, e)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *EntriesHandler) create(w http.ResponseWriter, r *http.Request, tenant, sub string) {
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad_request"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Body) == "" {
		http.Error(w, `{"error":"body_required"}`, http.StatusBadRequest)
		return
	}

	var e entry
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO entries (tenant_id, owner_sub, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, owner_sub, body, created_at`,
		tenant, sub, req.Body).
		Scan(&e.ID, &e.OwnerSub, &e.Body, &e.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"insert_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(e)
}
