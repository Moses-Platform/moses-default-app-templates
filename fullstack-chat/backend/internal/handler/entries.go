// Package handler implements the chat-roundtrip REST surface for the
// fullstack-chat template.
package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Entry is one row in the user-visible feed.
type Entry struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Source    string    `json:"source"` // "user" | "moses_manager" | "agent" — free text
	CreatedAt time.Time `json:"created_at"`
}

// EntriesHandler owns the entries CRUD against Postgres.
type EntriesHandler struct {
	DB *sql.DB
}

// NewEntriesHandler wires the handler with the platform-injected DB.
func NewEntriesHandler(db *sql.DB) *EntriesHandler {
	return &EntriesHandler{DB: db}
}

// Entries dispatches GET (list) and POST (create) on /api/v1/entries.
// Workspace-Tools registers POST /api/v1/entries as an MCP tool that Moses
// Manager calls when fulfilling a chat_prompt action.
func (h *EntriesHandler) Entries(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.list(w, r)
	case http.MethodPost:
		h.create(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET to list, POST to create.")
	}
}

func (h *EntriesHandler) list(w http.ResponseWriter, r *http.Request) {
	tenantID := mosesTenant(r)

	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, text, source, created_at
		   FROM entries
		  WHERE tenant_id = $1
		  ORDER BY created_at DESC
		  LIMIT 200`,
		tenantID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_failed", err.Error())
		return
	}
	defer rows.Close()

	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.Text, &e.Source, &e.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "scan_failed", err.Error())
			return
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "rows_err", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "count": len(out)})
}

type createBody struct {
	Text   string `json:"text"`
	Source string `json:"source"`
}

func (h *EntriesHandler) create(w http.ResponseWriter, r *http.Request) {
	tenantID := mosesTenant(r)

	var body createBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	body.Text = strings.TrimSpace(body.Text)
	if body.Text == "" {
		writeError(w, http.StatusBadRequest, "text_required", "field 'text' is required and must be non-empty")
		return
	}
	if n := len([]rune(body.Text)); n > 280 {
		writeError(w, http.StatusBadRequest, "text_too_long", fmt.Sprintf("text length %d > 280 chars", n))
		return
	}
	if body.Source == "" {
		body.Source = "user"
	}
	if !validSource(body.Source) {
		writeError(w, http.StatusBadRequest, "invalid_source",
			"source must be one of: user, moses_manager, agent, system")
		return
	}

	var e Entry
	err := h.DB.QueryRowContext(r.Context(),
		`INSERT INTO entries (tenant_id, text, source)
		      VALUES ($1, $2, $3)
		   RETURNING id, text, source, created_at`,
		tenantID, body.Text, body.Source,
	).Scan(&e.ID, &e.Text, &e.Source, &e.CreatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, e)
}

// mosesTenant extracts the tenant identifier from the platform-injected
// header. Falls back to a per-deploy default for local dev (where Moses
// middleware isn't sitting in front).
func mosesTenant(r *http.Request) string {
	if v := r.Header.Get("X-Moses-Tenant-ID"); v != "" {
		return v
	}
	return "local-dev"
}

func validSource(s string) bool {
	switch s {
	case "user", "moses_manager", "agent", "system":
		return true
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
		"code":  code,
	})
}

// errMissingDB is exported for tests that exercise nil-DB defenses.
var errMissingDB = errors.New("database not configured")
