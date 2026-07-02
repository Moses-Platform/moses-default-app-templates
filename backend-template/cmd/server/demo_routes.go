package main

// DEMO route wiring. Everything in this file backs the example Item CRUD +
// platform-info endpoints; ./clean_out_template.sh replaces it with an empty
// stub. Register your real API routes here — buildMux (main.go) calls
// registerDemoRoutes exactly once, and the MOSES ROUTING contract
// (CHAT-8qiu0) is already honored by prefixing every route with basePath.

import (
	"log"
	"net/http"

	"github.com/moses-platform/backend-template/internal/handler"
	"github.com/moses-platform/backend-template/internal/model"
)

// registerDemoRoutes registers the demo API endpoints ONCE under basePath
// (its browser-facing home; the workspace-tool proxy calls the same paths).
func registerDemoRoutes(mux *http.ServeMux, basePath string) {
	// In-memory item store — demo only; restarts wipe state.
	itemStore := model.NewItemStore()
	itemHandler := handler.NewItemHandler(itemStore)
	platformHandler := handler.NewPlatformHandler()
	mux.HandleFunc("GET "+basePath+"/api/v1/items", itemHandler.ListItems)
	mux.HandleFunc("GET "+basePath+"/api/v1/items/{id}", itemHandler.GetItem)
	mux.HandleFunc("POST "+basePath+"/api/v1/items", itemHandler.CreateItem)
	mux.HandleFunc("GET "+basePath+"/api/v1/platform/info", platformHandler.PlatformInfo)
}

// logDemoEndpoints prints the demo API endpoints at startup. Called from
// main()'s startup goroutine so buildMux stays log-free for tests.
func logDemoEndpoints(displayURL, basePath string) {
	log.Printf("API endpoints: %s%s/api/v1/items", displayURL, basePath)
}
