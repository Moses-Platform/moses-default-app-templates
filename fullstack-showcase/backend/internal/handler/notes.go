package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/moses-platform/fullstack-showcase/internal/middleware"
)

// Note represents a stored note
type Note struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteInput is the input for creating/updating a note
type NoteInput struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// NotesHandler holds the database dependency
type NotesHandler struct {
	db *sql.DB
}

// NewNotesHandler creates a new NotesHandler
func NewNotesHandler(db *sql.DB) *NotesHandler {
	return &NotesHandler{db: db}
}

// CHAT-pxeo.12: storage tenant_id is deploy-pinned (env). Header is
// caller-context only and drives the 403 cross-check on writes.

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

// enforceTenantMatch returns true and writes 403 when the caller-supplied
// header tenant disagrees with the deploy-pinned self tenant. Body
// intentionally omits the actual UUIDs so we never leak tenant ids.
func enforceTenantMatch(w http.ResponseWriter, mc middleware.MosesContext) bool {
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

// ListNotes returns all notes for the deploy-pinned self tenant.
func (h *NotesHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mc := middleware.GetMosesContext(r.Context())
	if enforceTenantMatch(w, mc) {
		return
	}
	tenantID := mc.SelfTenantID

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, tenant_id, title, content, created_at, updated_at
		 FROM notes WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		http.Error(w, "Failed to query notes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	notes := []Note{}
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.TenantID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			http.Error(w, "Failed to scan note", http.StatusInternalServerError)
			return
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Failed to iterate notes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

// CreateNote creates a new note for the deploy-pinned self tenant.
func (h *NotesHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mc := middleware.GetMosesContext(r.Context())
	if enforceTenantMatch(w, mc) {
		return
	}

	var input NoteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if input.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	tenantID := mc.SelfTenantID

	var note Note
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO notes (tenant_id, title, content) VALUES ($1, $2, $3)
		 RETURNING id, tenant_id, title, content, created_at, updated_at`,
		tenantID, input.Title, input.Content,
	).Scan(&note.ID, &note.TenantID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt)
	if err != nil {
		http.Error(w, "Failed to create note", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(note)
}

// GetNote returns a single note by ID, scoped to the deploy-pinned self
// tenant.
// CHAT-pbup.17: derive ID via the literal segment (not TrimPrefix against
// r.URL.Path) so both root and MOSES_BASE_PATH-prefixed mounts work.
func (h *NotesHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractNoteID(r.URL.Path)
	if id == "" {
		http.Error(w, "Note ID required", http.StatusBadRequest)
		return
	}

	mc := middleware.GetMosesContext(r.Context())
	if enforceTenantMatch(w, mc) {
		return
	}
	tenantID := mc.SelfTenantID

	var note Note
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, tenant_id, title, content, created_at, updated_at
		 FROM notes WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&note.ID, &note.TenantID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Failed to get note", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// DeleteNote deletes a note by ID, scoped to the deploy-pinned self tenant.
// CHAT-pbup.17: derive ID via the literal segment (not TrimPrefix against
// r.URL.Path) so both root and MOSES_BASE_PATH-prefixed mounts work.
func (h *NotesHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractNoteID(r.URL.Path)
	if id == "" {
		http.Error(w, "Note ID required", http.StatusBadRequest)
		return
	}

	mc := middleware.GetMosesContext(r.Context())
	if enforceTenantMatch(w, mc) {
		return
	}
	tenantID := mc.SelfTenantID

	result, err := h.db.ExecContext(r.Context(),
		`DELETE FROM notes WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		http.Error(w, "Failed to delete note", http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Notes dispatches to the correct handler based on method and path.
// CHAT-pbup.17: detect the trailing-ID case via the same literal-segment
// helper used by GetNote/DeleteNote so the dispatcher works whether the
// route was registered at "/api/v1/notes" or
// "/apps/<tenant>/<slug>/api/v1/notes".
func (h *NotesHandler) Notes(w http.ResponseWriter, r *http.Request) {
	id := extractNoteID(r.URL.Path)

	if id == "" {
		switch r.Method {
		case http.MethodGet:
			h.ListNotes(w, r)
		case http.MethodPost:
			h.CreateNote(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Path has an ID
	switch r.Method {
	case http.MethodGet:
		h.GetNote(w, r)
	case http.MethodDelete:
		h.DeleteNote(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// extractNoteID returns the trailing ID after the "/api/v1/notes" segment,
// or "" when the path ends at the collection (i.e. ListNotes / CreateNote
// dispatch). It tolerates an arbitrary leading sub-path
// (e.g. "/apps/<tenant>/<slug>") so the same handler logic works at both
// the root mount and the MOSES_BASE_PATH mount.
func extractNoteID(urlPath string) string {
	const segment = "/api/v1/notes"
	idx := strings.Index(urlPath, segment)
	if idx < 0 {
		return ""
	}
	rest := urlPath[idx+len(segment):]
	rest = strings.TrimPrefix(rest, "/")
	return strings.TrimSuffix(rest, "/")
}
