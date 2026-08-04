// App route wiring — this is where your own API endpoints go. The
// plumbing routes (health, openapi, completion webhook, /__moses/invoke
// proxy) are registered in buildMux (main.go) and are already live.
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
// "/apps/<tenant>/<slug>" for Moses sub-path deploys).
//
// WORKED EXAMPLE: example_test.go (this package) — real, CI-compiled code, not
// a comment. Its handler half is internal/handler/example_test.go, its storage
// half the `things` table sketched in internal/database/migrate_demo.go, and
// its spec half the comment above the //go:embed directive in api/api.go.
func registerDemoRoutes(mux *http.ServeMux, basePath string, db *sql.DB) {
	_, _, _ = mux, basePath, db // parameters kept for your wiring above
}

// newCompletionSink chooses where verified chat-completion webhooks are
// persisted (see handler.CompletionSink in webhook_chat.go).
//
// TODO: replace the log-only sink with your own storage — e.g. a struct
// writing to a table you create in internal/database/migrate_demo.go.
func newCompletionSink(db *sql.DB) handler.CompletionSink {
	_ = db
	return handler.LogCompletionSink{}
}
