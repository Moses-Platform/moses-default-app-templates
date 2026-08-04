// WORKED EXAMPLE — your first tenant-scoped handler pair.
//
// WHY THIS IS A _test.go FILE: it is a compiled example that must never ship.
// `go vet ./...` and `go test ./...` (both in CI) compile every _test.go file,
// so a defect here fails the build; `go build` excludes _test.go files, so
// nothing below ever reaches the binary. Deliberately declaration-only — there
// is no Test*/Benchmark*/Example* function here, so `go test` compiles it and
// runs nothing.
//
// HOW TO USE IT: copy the declarations into a real file (e.g.
// internal/handler/things.go), rename Thing/things to your real resource, and
// wire the rest of the slice — the table in internal/database/migrate_demo.go,
// the routes in cmd/server/demo_routes.go (see cmd/server/example_test.go) and
// the "/things" path in api/openapi.json (worked spec example: the comment
// above the //go:embed directive in api/api.go). Then delete this file.
//
// The template ships no application API routes of its own — the chat-completion
// webhook is platform plumbing, not an app route.
//
// The three rules this example encodes, in order:
//  1. enforceTenantMatch FIRST in every write handler; return immediately on
//     a true result — the 403 body is already written.
//  2. Scope every query by config.SelfTenantID() (the deploy-pinned
//     MOSES_TENANT_ID), NEVER by callerTenantHeader(r).
//  3. Never echo a tenant UUID in a response body (CHAT-w6gt) — note the
//     SELECT below does not project tenant_id.
//
// Responses go through writeJSON / writeError (respond.go) so every handler
// shares the same {"error": ..., "code": ...} shape.
package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/moses-platform/fullstack-chat/internal/config"
)

// ThingsHandler is the example resource handler.
type ThingsHandler struct{ db *sql.DB }

// NewThingsHandler wires the handler to the shared *sql.DB from main().
func NewThingsHandler(db *sql.DB) *ThingsHandler { return &ThingsHandler{db: db} }

// Thing is the example resource.
type Thing struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// No TenantID field on purpose (CHAT-w6gt).
}

// List answers GET {basePath}/api/v1/things (operationId listThings).
func (h *ThingsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name FROM things WHERE tenant_id = $1 ORDER BY created_at DESC`,
		config.SelfTenantID()) // rule 2
	if err != nil {
		writeError(w, http.StatusInternalServerError, "E_QUERY", "query failed")
		return
	}
	defer rows.Close()
	things := []Thing{}
	for rows.Next() {
		var t Thing
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			writeError(w, http.StatusInternalServerError, "E_SCAN", "scan failed")
			return
		}
		things = append(things, t)
	}
	// The list response is an OBJECT ({"things": [...]}), never a bare array —
	// the OpenAPI schema and the frontend's fetchAPI<{ things: Thing[] }> must
	// agree with this shape.
	writeJSON(w, http.StatusOK, map[string]any{"things": things})
}

// Create answers POST {basePath}/api/v1/things (operationId createThing).
func (h *ThingsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if enforceTenantMatch(w, r) { // rule 1 — before any mutation
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		writeError(w, http.StatusBadRequest, "E_INVALID_BODY", "name is required")
		return
	}
	var t Thing
	if err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO things (tenant_id, name) VALUES ($1, $2) RETURNING id, name`,
		config.SelfTenantID(), in.Name, // rule 2
	).Scan(&t.ID, &t.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "E_INSERT", "insert failed")
		return
	}
	writeJSON(w, http.StatusCreated, t) // rule 3 — no tenant id in the body
}
