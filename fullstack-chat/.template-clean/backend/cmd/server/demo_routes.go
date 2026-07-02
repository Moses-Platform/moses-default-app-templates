// App route wiring. The demo entries feed that used to live here was
// removed by clean_out_template.sh; the plumbing routes (health, openapi,
// completion webhook, /__moses/invoke proxy) are registered in buildMux
// (main.go) and are already live.
package main

import (
	"database/sql"
	"net/http"

	"github.com/moses-platform/fullstack-chat/internal/handler"
)

// registerDemoRoutes mounts this app's API endpoints under basePath. Called
// exactly once from buildMux (main.go) — keep the signature stable.
//
// TODO: register your routes here. MOSES ROUTING contract: every API route
// registers ONCE, under basePath (it is "" for standalone deploys and
// "/apps/<tenant>/<slug>" for Moses sub-path deploys), e.g.
//
//	widgets := handler.NewWidgetsHandler(db)
//	mux.HandleFunc(basePath+"/api/v1/widgets", widgets.Widgets)
//
// Then add each path to backend/api/openapi.json so the platform's
// workspace-tool discovery picks it up after deploy.
func registerDemoRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	_, _, _ = mux, basePath, db // parameters kept for your wiring above
}

// newCompletionSink chooses where verified chat-completion webhooks are
// persisted (see handler.CompletionSink in webhook_chat.go).
//
// TODO: replace the log-only sink with your own storage (e.g. a struct
// writing to a table created in internal/database/migrate_demo.go's
// successor). The demo used a Postgres sink writing to chat_completions.
func newCompletionSink(db *sql.DB) handler.CompletionSink {
	_ = db
	return handler.LogCompletionSink{}
}
