package main

import (
	"database/sql"
	"net/http"

	"github.com/moses-platform/fullstack-showcase/internal/handler"
)

// registerDemoRoutes concentrates ALL demo route registration in one file
// so clean_out_template.sh can replace it with an empty stub. buildMux
// calls it exactly once; everything registered here is showcase content —
// the platform plumbing (health + openapi dual-mount) stays in buildMux.
//
// Routes are registered ONCE under MOSES_BASE_PATH (see the MOSES ROUTING
// comment on buildMux in main.go).
func registerDemoRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	notesHandler := handler.NewNotesHandler(db)
	usersHandler := handler.NewUsersHandler()

	mux.HandleFunc(basePath+"/api/v1/moses-info", handler.MosesInfo)
	mux.HandleFunc(basePath+"/api/v1/capabilities", handler.ListCapabilities)
	mux.HandleFunc(basePath+"/api/v1/capabilities/", handler.GetCapability)
	mux.HandleFunc(basePath+"/api/v1/notes", notesHandler.Notes)
	mux.HandleFunc(basePath+"/api/v1/notes/", notesHandler.Notes)
	mux.HandleFunc(basePath+"/api/v1/users", usersHandler.Users)
}
