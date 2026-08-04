package main

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/moses-platform/fullstack-unified/internal/config"
)

//go:embed static/*
var staticFiles embed.FS

// Adding your first endpoint — worked example (comments only; nothing here ships).
//
// openapi.json is embedded into the binary below and served at /api/openapi.json,
// so anything written into it reaches production. Keep it minimal and real: put
// examples here, in comments, never in the spec itself.
//
// Add a path to the "paths" map in openapi.json, e.g.:
//
//	"/things": {
//	  "get": {
//	    "operationId": "listThings",
//	    "summary": "List things",
//	    "responses": {"200": {"description": "OK",
//	      "content": {"application/json": {"schema": {
//	        "type": "object",
//	        "required": ["things"],
//	        "properties": {"things": {"type": "array", "items": {
//	          "type": "object",
//	          "properties": {"id": {"type": "string"},
//	                         "name": {"type": "string"}}}}}}}}}}
//	  },
//	  "post": {
//	    "operationId": "createThing",
//	    "requestBody": {"required": true, "content": {"application/json":
//	      {"schema": {"type": "object", "required": ["name"],
//	        "properties": {"name": {"type": "string"}}}}}},
//	    "responses": {"201": {"description": "Created",
//	      "content": {"application/json": {"schema": {"type": "object",
//	        "properties": {"id": {"type": "string"},
//	                       "name": {"type": "string"}}}}}}}
//	  }
//	}
//
// Five rules this demonstrates:
//  1. The path key is RELATIVE to servers[0].url ("/api/v1"), so "/things" is
//     served at /api/v1/things. Writing "/api/v1/things" here double-prefixes and
//     every agent tool call 404s while the browser UI keeps working.
//  2. Never list /health — it would register a phantom workspace tool.
//  3. Each operationId becomes an MCP tool named workspace_<toolKey>_<operationId>
//     (toolKey derives from "name" in moses-app.config.json), so listThings is
//     callable by agents as workspace_<toolKey>_listThings.
//  4. Never put a tenant UUID in a response schema (CHAT-w6gt).
//  5. The list response is an OBJECT ({"things": [...]}), never a bare array.
//     That is what the handler encodes (map[string]any{"things": things}) and
//     what the frontend types (fetchAPI<{ things: Thing[] }>); all three layers
//     must agree or the generated MCP tool returns something the caller cannot
//     read. Schemas are INLINED above so nothing dangles — if you prefer
//     {"$ref": "#/components/schemas/Thing"} you must also define that schema
//     under components.schemas in openapi.json, or the served spec is invalid.
//
// Then register the matching route in demo_routes.go — the worked route +
// handler example is real, CI-compiled code in example_test.go. The spec and
// the router must agree, and main_test.go locks that.
//
//go:embed api/openapi.json
var openapiSpec []byte

// indexTemplate is parsed once at startup from the embedded static/index.html.
// It renders the Moses browser-logger meta tag using MOSES_CHART_ID,
// MOSES_DEPLOYMENT_ID, and MOSES_API_BASE env vars (BLF-J / CHAT-ry35). When
// the env vars are absent the tag still renders with empty values and the
// snippet falls back to a location-derived `loc` param so the platform can
// resolve identity server-side (it does NOT disable itself).
//
// NOTE: because index.html goes through html/template, any literal `{{ }}`
// in it is executed as a template action against indexContext — a stray
// `{{title}}` silently renders as an error/empty. Escape braces or keep
// them out of static/index.html (see skills/usage.md).
var indexTemplate *template.Template

// indexContext is the data struct rendered into static/index.html. The
// fields match the data-* attributes on the <meta name="moses-config"> tag.
type indexContext struct {
	ChartID      string
	DeploymentID string
	APIBase      string
}

// loadIndexTemplate parses the embedded static/index.html into indexTemplate.
// Idempotent — safe to call from tests.
func loadIndexTemplate() {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Printf("NOTE: failed to create static sub-filesystem: %v", err)
		return
	}
	data, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		log.Printf("NOTE: failed to read embedded static/index.html: %v", err)
		return
	}
	tmpl, err := template.New("index.html").Parse(string(data))
	if err != nil {
		log.Printf("NOTE: failed to parse static/index.html as template; serving raw: %v", err)
		return
	}
	indexTemplate = tmpl
}

// renderIndex renders indexTemplate into w with the given context. Returns
// whether the template was rendered (false → caller should fall back to the
// file server). Extracted for test coverage of the BLF-J substitution.
func renderIndex(w http.ResponseWriter, ctx indexContext) bool {
	if indexTemplate == nil {
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := indexTemplate.Execute(w, ctx); err != nil {
		log.Printf("NOTE: failed to render index.html template: %v", err)
	}
	return true
}

func main() {
	// CHAT-pxeo.12: hard fail-fast when MOSES_TENANT_ID is unset on a
	// deployed pod. fullstack-unified has no persistent storage in this
	// template, but the contract is symmetrical with the other templates.
	config.Validate()

	// CHAT-0lu74: external runtime secrets (e.g. EXAMPLE_API_KEY) belong in
	// `moses-app.config.json` → `secrets.external[]`. See
	// `skills/secrets-tutorial.md` for the read+validate pattern.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// CHAT-pbup: prefer MOSES_BASE_PATH (canonical) over BASE_URL (deprecated
	// alias). Both carry the same value during the N-2 deprecation window.
	baseURL := strings.TrimSuffix(os.Getenv("MOSES_BASE_PATH"), "/")
	if baseURL == "" {
		baseURL = strings.TrimSuffix(os.Getenv("BASE_URL"), "/")
		if baseURL != "" {
			log.Printf("NOTE: BASE_URL is deprecated; please set MOSES_BASE_PATH instead. See DEPRECATIONS.md")
		}
	}

	mux := http.NewServeMux()

	// Register health / OpenAPI / API routes (see registerAPIRoutes).
	registerAPIRoutes(mux, baseURL)

	// Static files — served at BASE_URL prefix via Go embed
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub-filesystem: %v", err)
	}

	// Parse index.html as a Go template once at startup so the BLF-J browser
	// logger meta tag can be filled with chart/deployment IDs from the env
	// without re-reading the file per request. If parsing fails the index
	// handler falls back to serving the raw file — the snippet's readConfig()
	// treats the literal {{.…}} placeholders as absent and the logger then
	// uses its location-derived `loc` fallback.
	loadIndexTemplate()

	fileServer := http.FileServer(http.FS(staticFS))

	// indexHandler renders index.html through the parsed template (BLF-J),
	// falling back to the file server for every other asset (CSS, JS, fav-
	// icon, etc.). It is mounted AFTER http.StripPrefix(baseURL) so the path
	// here is always relative to the embedded static FS root.
	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if renderIndex(w, indexContext{
				ChartID:      os.Getenv("MOSES_CHART_ID"),
				DeploymentID: os.Getenv("MOSES_DEPLOYMENT_ID"),
				APIBase:      os.Getenv("MOSES_API_BASE"),
			}) {
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}

	if baseURL != "" {
		mux.Handle(baseURL+"/", http.StripPrefix(baseURL, http.HandlerFunc(indexHandler)))
	} else {
		mux.HandleFunc("/", indexHandler)
	}

	srv := &http.Server{
		Addr: ":" + port,
		// CHAT-pbup: WithEmbeddingHeaders emits Content-Security-Policy:
		// frame-ancestors per the MOSES_EMBEDDING_FRAMING env var on every
		// HTML response. JSON / OpenAPI responses skip the header.
		// RejectCrossSiteCSRF blocks cross-site state-changing requests (the app
		// must not rely on the platform edge alone — the access_token cookie is
		// SameSite=None). Wrapped innermost so it guards the API + static routes
		// while the outer middleware still runs on blocked (403) requests; only
		// unsafe methods are blocked, so static-asset / SPA GETs pass through.
		// corsMiddleware (cors.go) is outermost: OFF by default (same-origin
		// deployment model), opt-in via CORS_ALLOWED_ORIGINS.
		Handler:      corsMiddleware(withEmbeddingHeaders(RejectCrossSiteCSRF(mux))),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Server starting on :%s (BASE_URL=%s)", port, baseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("Server stopped")
}

// registerAPIRoutes wires the health / OpenAPI / API routes onto mux.
// Single source of truth shared by main() and the tests.
//
// MOSES ROUTING (CHAT-8qiu0). Moses deploys apps at a sub-path; the pod
// receives that path in MOSES_BASE_PATH and the ingress does NOT strip it.
// The platform's workspace-tool proxy calls the app's API *under*
// MOSES_BASE_PATH, so the API is registered ONCE there — baseURL is "" for
// standalone/dev deploys and "/apps/<tenant>/<slug>" for Moses sub-path
// deploys.
//
//   - /health: registered at BOTH /health and {baseURL}/health — the kubelet
//     probe hits the canonical /health (it bypasses the ingress), while a
//     sub-path caller reaching the pod still gets a 200.
//   - /api/openapi.json: registered at BOTH the canonical path and
//     {baseURL}/api/openapi.json — discovery uses the canonical path; the
//     base-path alias keeps it reachable when the app is mounted under
//     MOSES_BASE_PATH (CHAT-yfmwv).
//   - /api/v1/* : registered ONCE under baseURL (its browser-facing home).
func registerAPIRoutes(mux *http.ServeMux, baseURL string) {
	// Health check — canonical for the K8s probe, plus the base-path alias.
	mux.HandleFunc("/health", handleHealth)
	if baseURL != "" {
		mux.HandleFunc(baseURL+"/health", handleHealth)
	}

	// OpenAPI spec — canonical for the WorkspaceToolProxy discovery hook, plus
	// the base-path alias so it stays reachable when the app is mounted under
	// MOSES_BASE_PATH (CHAT-yfmwv).
	mux.HandleFunc("/api/openapi.json", handleOpenAPI)
	if baseURL != "" {
		mux.HandleFunc(baseURL+"/api/openapi.json", handleOpenAPI)
	}

	// API endpoints — registered ONCE under MOSES_BASE_PATH, concentrated in
	// demo_routes.go: one call site, so route registration can never drift.
	registerDemoRoutes(mux, baseURL)
}

// mosesContext holds Moses platform headers extracted from the request.
//
// CHAT-pxeo.12: CallerTenantID is the value of X-Moses-Tenant-ID from
// the inbound request — it represents the *caller's* tenant context,
// NOT this app's self-identification. Self tenant comes from the
// MOSES_TENANT_ID env (config.SelfTenantID()). They MAY agree
// (workspace-tool calls) or disagree (cross-tenant access attempt) —
// the latter triggers the strict-tenant-check 403 path.
//
// CHAT-w6gt: tenant UUIDs are kept for internal logic (the 403
// cross-check, audit trails) but omitted from JSON responses so the
// /status debug endpoint doesn't broaden the tenant-UUID attack surface
// via screenshots / log channels / cross-pod aggregation. The
// caller-tenant value is still observable via the X-Moses-Tenant-ID
// request header they sent in.
type mosesContext struct {
	SelfTenantID   string `json:"-"`
	CallerTenantID string `json:"-"`
	UserID         string `json:"user_id"`
	ChartID        string `json:"chart_id"`
	RequestID      string `json:"request_id"`
}

func getMosesContext(r *http.Request) mosesContext {
	return mosesContext{
		SelfTenantID:   config.SelfTenantID(),
		CallerTenantID: r.Header.Get("X-Moses-Tenant-ID"),
		UserID:         r.Header.Get("X-Moses-User-ID"),
		ChartID:        r.Header.Get("X-Moses-Chart-ID"),
		RequestID:      r.Header.Get("X-Moses-Request-ID"),
	}
}

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

// enforceTenantMatch returns true and writes a 403 when the caller-supplied
// header tenant disagrees with the deploy-pinned self tenant. Body
// intentionally omits UUIDs.
func enforceTenantMatch(w http.ResponseWriter, r *http.Request) bool {
	if !strictTenantCheckEnabled() {
		return false
	}
	caller := strings.TrimSpace(r.Header.Get("X-Moses-Tenant-ID"))
	if caller == "" || caller == config.SelfTenantID() {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"tenant_mismatch","code":"E_TENANT_MISMATCH"}`))
	return true
}

// embeddingPolicy is the resolved CSP frame-ancestors policy this server
// emits on HTML responses. Set once at startup from env vars (CHAT-pbup):
//
//	MOSES_EMBEDDING_FRAMING            "moses-only" | "public" | "denied" (default: appType-driven)
//	MOSES_EMBEDDING_ALLOWED_ANCESTORS  space-separated CSP source list
//	MOSES_EMBEDDING_REPORT_URI         optional report-uri
//
// The platform's deploy pipeline resolves "moses-only" to a concrete origin
// list (chart domain + Tauri origins, mirrored from chart/templates/ingressroute.yaml)
// before the env vars reach this binary. Standalone deploys leave the env
// vars empty and the middleware emits the appType default — for hybrid
// (this template), that's "moses-only" with self-only frame-ancestors.
type embeddingPolicy struct {
	cspFrameAncestors string
	xFrameOptions     string // "" -> omit header
	reportURI         string
}

var resolvedEmbeddingPolicy = func() embeddingPolicy {
	framing := os.Getenv("MOSES_EMBEDDING_FRAMING")
	if framing == "" {
		framing = "moses-only" // appType default for hybrid
	}
	allowed := os.Getenv("MOSES_EMBEDDING_ALLOWED_ANCESTORS")
	report := os.Getenv("MOSES_EMBEDDING_REPORT_URI")

	switch framing {
	case "public":
		return embeddingPolicy{cspFrameAncestors: "*", reportURI: report}
	case "denied":
		return embeddingPolicy{cspFrameAncestors: "'none'", xFrameOptions: "DENY", reportURI: report}
	default: // moses-only
		if allowed == "" {
			allowed = "'self'"
		}
		return embeddingPolicy{cspFrameAncestors: allowed, reportURI: report}
	}
}()

// withEmbeddingHeaders attaches Content-Security-Policy: frame-ancestors
// (and optionally X-Frame-Options for the strict "denied" case) to HTML
// responses. JSON / OpenAPI responses skip the header — they are never
// framed by browsers and adding the directive would be noise.
func withEmbeddingHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We have to set the header BEFORE the underlying handler writes
		// the response body. The handler chain hasn't yet decided
		// Content-Type, but we attach unconditionally and let the
		// caller-side strip it for non-HTML responses if they care. In
		// practice this template's mux serves index.html / static at
		// the prefix and JSON at /api/* — for JSON the spec is silent
		// (no harm).
		policy := "frame-ancestors " + resolvedEmbeddingPolicy.cspFrameAncestors
		if resolvedEmbeddingPolicy.reportURI != "" {
			policy += "; report-uri " + resolvedEmbeddingPolicy.reportURI
		}
		w.Header().Set("Content-Security-Policy", policy)
		if resolvedEmbeddingPolicy.xFrameOptions != "" {
			w.Header().Set("X-Frame-Options", resolvedEmbeddingPolicy.xFrameOptions)
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "fullstack-unified",
	})
}

func handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(openapiSpec)
}
