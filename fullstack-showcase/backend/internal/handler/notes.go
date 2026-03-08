package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
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

// ListNotes returns all notes for the current tenant
func (h *NotesHandler) ListNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenantID := middleware.GetMosesContext(r.Context()).TenantID

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

// CreateNote creates a new note for the current tenant
func (h *NotesHandler) CreateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	tenantID := middleware.GetMosesContext(r.Context()).TenantID

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

// GetNote returns a single note by ID, scoped to tenant
func (h *NotesHandler) GetNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/notes/")
	if id == "" {
		http.Error(w, "Note ID required", http.StatusBadRequest)
		return
	}

	tenantID := middleware.GetMosesContext(r.Context()).TenantID

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

// DeleteNote deletes a note by ID, scoped to tenant
func (h *NotesHandler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/notes/")
	if id == "" {
		http.Error(w, "Note ID required", http.StatusBadRequest)
		return
	}

	tenantID := middleware.GetMosesContext(r.Context()).TenantID

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

// Notes dispatches to the correct handler based on method and path
func (h *NotesHandler) Notes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/notes")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
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
