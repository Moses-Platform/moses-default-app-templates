// WORKED EXAMPLE — the `things` vertical slice (routes + handler).
//
// WHY THIS IS A _test.go FILE: it is a compiled example that must never ship.
// `go vet ./...` and `go test ./...` (both in CI) compile every _test.go file,
// so a defect here fails the build; `go build` excludes _test.go files, so
// nothing below ever reaches the binary. Deliberately declaration-only — there
// is no Test*/Benchmark*/Example* function here, so `go test` compiles it and
// runs nothing.
//
// HOW TO USE IT: copy the body of exampleRegisterRoutes into registerDemoRoutes
// (demo_routes.go) and the handler declarations into a real file, rename
// Thing/things to your real resource, and wire the rest of the slice — the
// table in internal/database/migrate_demo.go, the "/things" path in
// api/openapi.json (worked spec example: the comment above the //go:embed
// directive in api/api.go), and the access.oidc.protectedPaths entry in
// moses-app.config.json (without it the route stays deny-by-default). Then
// delete this file.
//
// The handler lives in package main here (rather than in internal/handler/)
// only so route + handler + the OIDC scoping rules stay readable side by side
// — which is why the registration says NewThingsHandler(db) and not
// handler.NewThingsHandler(db). Moving it later to internal/handler/things.go
// (next to health.go / me.go / openapi.go) means re-qualifying the constructor
// as handler.NewThingsHandler and importing that package here.
//
// This template has NO enforceTenantMatch helper: the OIDCAuth middleware is
// the gate (a request that reaches a protectedPath already carries a verified
// identity), so tenant safety here reduces to the storage rules:
//  1. Every row is written and read with tenant_id = config.SelfTenantID() —
//     the deploy-pinned MOSES_TENANT_ID, NEVER the X-Moses-Tenant-ID header.
//     That is TENANT space (shared by the whole workspace). For USER space,
//     add `AND owner_sub = $2` with oidcauth.IdentityFrom(r.Context()).Subject.
//  2. Authorization decisions (roles) are made server-side via
//     id.HasRole("your-role") — never gated in the SPA alone.
//  3. Never echo a tenant UUID in a response body (CHAT-w6gt) — note the
//     SELECT below does not project tenant_id.
package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/moses-platform/fullstack-oidc/internal/config"
	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
)

// exampleRegisterRoutes is what registerDemoRoutes looks like once you have
// routes.
//
// MOSES ROUTING contract (CHAT-8qiu0): register every route ONCE under
// MOSES_BASE_PATH, exactly like the kept /api/v1/me route in buildMux, so both
// the browser and the platform's workspace-tool proxy reach it. Mirror each
// path in api/openapi.json RELATIVE to the servers[] "/api/v1" entry (never
// /health) so workspace-tool discovery picks it up.
func exampleRegisterRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	things := NewThingsHandler(db)
	mux.HandleFunc("GET "+basePath+"/api/v1/things", things.List)
	mux.HandleFunc("POST "+basePath+"/api/v1/things", things.Create)
}

// ThingsHandler is the example resource handler.
type ThingsHandler struct{ db *sql.DB }

// NewThingsHandler wires the handler to the shared *sql.DB from main().
func NewThingsHandler(db *sql.DB) *ThingsHandler { return &ThingsHandler{db: db} }

// Thing is the example resource.
type Thing struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// No TenantID / owner_sub field on purpose (CHAT-w6gt).
}

// List answers GET {basePath}/api/v1/things (operationId listThings).
func (h *ThingsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name FROM things WHERE tenant_id = $1 ORDER BY created_at DESC`,
		config.SelfTenantID()) // rule 1
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	things := []Thing{}
	for rows.Next() {
		var t Thing
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			http.Error(w, "scan failed", http.StatusInternalServerError)
			return
		}
		things = append(things, t)
	}
	w.Header().Set("Content-Type", "application/json")
	// The list response is an OBJECT ({"things": [...]}), never a bare array —
	// the OpenAPI schema and the frontend's fetchAPI<{ things: Thing[] }> must
	// agree with this shape.
	_ = json.NewEncoder(w).Encode(map[string]any{"things": things})
}

// Create answers POST {basePath}/api/v1/things (operationId createThing).
func (h *ThingsHandler) Create(w http.ResponseWriter, r *http.Request) {
	id := oidcauth.IdentityFrom(r.Context())
	if !id.Authenticated {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	var t Thing
	if err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO things (tenant_id, owner_sub, name) VALUES ($1, $2, $3)
		 RETURNING id, name`,
		config.SelfTenantID(), id.Subject, in.Name, // rule 1
	).Scan(&t.ID, &t.Name); err != nil {
		http.Error(w, "insert failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(t) // rule 3 — no tenant id in the body
}
