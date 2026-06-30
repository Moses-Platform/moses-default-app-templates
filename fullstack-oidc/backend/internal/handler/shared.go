package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/moses-platform/fullstack-oidc/internal/config"
	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

// SharedNotesHandler serves the tenant-shared `shared_notes` demo resource —
// the TENANT space, the counterpart to the per-user EntriesHandler.
//
// Reads are scoped by the deploy-pinned tenant id ALONE (config.SelfTenantID),
// so every member of the workspace sees the same list. `author_sub` is recorded
// for attribution only; it is NEVER used to filter, which is the whole point:
// when an agent posts here through a Moses workspace-tool call, `oidcauth`
// resolves its identity from the trusted `X-Moses-User-ID` header (the agent's
// id, not the human's). Tenant-scoping keeps that delivered content visible to
// everyone — including the human who owns the app. Scope collaborative or
// agent-fed content this way; use the per-user `entries` shape only for data
// that is meant to stay private.
type SharedNotesHandler struct {
	db *sql.DB
}

// NewSharedNotesHandler builds a SharedNotesHandler.
func NewSharedNotesHandler(db *sql.DB) *SharedNotesHandler {
	return &SharedNotesHandler{db: db}
}

type sharedNote struct {
	ID        string `json:"id"`
	AuthorSub string `json:"author_sub"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// listSharedNotesByTenantQuery filters by tenant ONLY — never by author — so the
// whole workspace shares one list (and agent-posted notes stay visible to the
// human). Do NOT add an `author_sub` predicate here; that would turn this back
// into a private per-user table and reintroduce the exact bug it exists to fix.
// Guarded by TestSharedNotesListQueryIsTenantScopedOnly.
const listSharedNotesByTenantQuery = `SELECT id, author_sub, body, created_at
		   FROM shared_notes
		  WHERE tenant_id = $1
		  ORDER BY created_at DESC`

const insertSharedNoteQuery = `INSERT INTO shared_notes (tenant_id, author_sub, body)
		 VALUES ($1, $2, $3)
		 RETURNING id, author_sub, body, created_at`

// SharedNotes dispatches GET (list) and POST (create). The route is protected
// by OIDCAuth, so id.Authenticated is always true here — but unlike entries the
// caller's identity drives only authorship, not visibility.
func (h *SharedNotesHandler) SharedNotes(w http.ResponseWriter, r *http.Request) {
	id := oidcauth.IdentityFrom(r.Context())
	if !id.Authenticated || id.Subject == "" {
		// Defensive: the middleware should have gated this already.
		http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
		return
	}
	tenant := config.SelfTenantID()

	switch r.Method {
	case http.MethodGet:
		h.list(w, r, tenant)
	case http.MethodPost:
		h.create(w, r, tenant, id.Subject)
	default:
		http.Error(w, `{"error":"method_not_allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *SharedNotesHandler) list(w http.ResponseWriter, r *http.Request, tenant string) {
	// Tenant-only filter: no `author_sub` predicate, so the list is shared by
	// the whole workspace and surfaces content posted by agents too.
	rows, err := h.db.QueryContext(r.Context(), listSharedNotesByTenantQuery, tenant)
	if err != nil {
		http.Error(w, `{"error":"query_failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	out := []sharedNote{}
	for rows.Next() {
		var n sharedNote
		if err := rows.Scan(&n.ID, &n.AuthorSub, &n.Body, &n.CreatedAt); err != nil {
			http.Error(w, `{"error":"scan_failed"}`, http.StatusInternalServerError)
			return
		}
		out = append(out, n)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *SharedNotesHandler) create(w http.ResponseWriter, r *http.Request, tenant, author string) {
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

	var n sharedNote
	err := h.db.QueryRowContext(r.Context(), insertSharedNoteQuery, tenant, author, req.Body).
		Scan(&n.ID, &n.AuthorSub, &n.Body, &n.CreatedAt)
	if err != nil {
		http.Error(w, `{"error":"insert_failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(n)
}
