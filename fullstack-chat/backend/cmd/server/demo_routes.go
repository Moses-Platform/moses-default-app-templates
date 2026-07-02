// DEMO WIRING — everything in this file backs the entries-feed demo and is
// replaced by clean_out_template.sh with a stub twin. The plumbing routes
// (health, openapi, webhook receiver, /__moses/invoke proxy) are registered
// in buildMux (main.go) and survive the cleanout.
package main

import (
	"database/sql"
	"net/http"

	"github.com/moses-platform/fullstack-chat/internal/handler"
)

// registerDemoRoutes mounts the demo API endpoints under basePath. Called
// exactly once from buildMux. Add your own app routes here (the clean twin
// keeps the function so buildMux never changes); remember the MOSES ROUTING
// contract — every API route registers ONCE, under basePath.
func registerDemoRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	entries := handler.NewEntriesHandler(db)
	mux.HandleFunc(basePath+"/api/v1/entries", entries.Entries)
}

// newCompletionSink chooses where verified chat-completion webhooks are
// persisted. The demo writes rows into the chat_completions table so the
// end-to-end recipe can inspect signature_valid in SQL.
func newCompletionSink(db *sql.DB) handler.CompletionSink {
	return handler.NewDBCompletionSink(db)
}
