package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Contract tests for the Moses plumbing surface (health + openapi + static
// index + browser logger + embedding CSP + tenant contract + the spec↔mux lock). They
// pass with ZERO application routes; as you add real endpoints, extend them
// (see the deleted demo's tests in git history for route-probing examples).

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "healthy") {
		t.Errorf("GET /health: expected body with 'healthy', got %q", w.Body.String())
	}
}

// CHAT-8qiu0: /health answers at BOTH the canonical path (kubelet probe,
// bypasses ingress) AND the base-path-prefixed path (sub-path callers).
func TestRoutes_SubPath_HealthAtBothPaths(t *testing.T) {
	mux := http.NewServeMux()
	registerAPIRoutes(mux, "/apps/t/x")
	for _, path := range []string{"/health", "/apps/t/x/health"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d", path, rec.Code)
		}
	}
}

// CHAT-8qiu0: /api/openapi.json stays canonical (platform discovery hook).
func TestRoutes_OpenAPICanonical(t *testing.T) {
	mux := http.NewServeMux()
	registerAPIRoutes(mux, "/apps/t/x")
	req := httptest.NewRequest("GET", "/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/openapi.json: expected 200, got %d", rec.Code)
	}
}

// CHAT-yfmwv: /api/openapi.json ALSO answers at the base-path-prefixed path.
func TestRoutes_OpenAPIAtBasePath(t *testing.T) {
	mux := http.NewServeMux()
	registerAPIRoutes(mux, "/apps/t/x")
	req := httptest.NewRequest("GET", "/apps/t/x/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /apps/t/x/api/openapi.json: expected 200, got %d", rec.Code)
	}
}

// CHAT-8qiu0 acceptance check: the OpenAPI spec's servers[] (if present)
// MUST stay base-path-free — the platform's openapi_parser folds
// servers[0].url into endpoint.Path and the proxy prepends MOSES_BASE_PATH;
// a base-path-aware servers[] would double-prefix.
func TestOpenAPISpec_ServersBasePathFree(t *testing.T) {
	spec := loadOpenAPISpec(t, openAPISpecPath())
	for _, s := range spec.Servers {
		if strings.Contains(s.URL, "/apps/") {
			t.Errorf("servers[].url %q must not contain a /apps/ base-path prefix", s.URL)
		}
	}
}

// The index page is served as a plain static file: nothing server-renders
// the browser-logger config any more (the logger finds its app from the page
// URL), so no Moses origin or id is baked into the page.
func TestStaticHandler_ServesIndexWithLoggerAndNoServerConfig(t *testing.T) {
	h := newStaticHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<script src="moses-browser-logger.js"></script>`) {
		t.Errorf("index must load the browser logger by a relative path\n--- body ---\n%s", body)
	}
	for _, banned := range []string{"moses-config", "{{"} {
		if strings.Contains(body, banned) {
			t.Errorf("index must not carry server-rendered logger config (%q)\n--- body ---\n%s", banned, body)
		}
	}
}

// browserLoggerHarness runs static/moses-browser-logger.js in a node vm with a
// fake window/fetch/clock and prints every fetch the snippet makes as JSON.
// Scenarios:
//   - expire:   the token has expired by the time the first batch is sent
//     (ingest 401 more than a minute after bootstrap);
//   - fresh401: the first batch is refused straight after bootstrap;
//   - race:     two batches in flight with the expiring token are both refused.
const browserLoggerHarness = `
const vm = require("vm"), fs = require("fs");
const [src, scenario] = [fs.readFileSync(process.argv[2], "utf8"), process.argv[3]];
let now = 1000000;
const statuses = { expire: [401, 200], fresh401: [401], race: [401, 401] }[scenario];
let releaseIngest;
const ingestGate = scenario === "race" ? new Promise((r) => (releaseIngest = r)) : Promise.resolve();
const calls = [], listeners = {};
const location = { origin: "http://apps.example", pathname: "/apps/t/s/", href: "http://apps.example/apps/t/s/" };
const window = { location, addEventListener: (t, f) => (listeners[t] = listeners[t] || []).push(f) };
const document = { querySelector: () => null, getElementsByTagName: () => [], addEventListener() {}, visibilityState: "visible" };
class FakeDate extends Date { static now() { return now; } }
function fetch(url, opts) {
  calls.push({ url, opts: { credentials: opts && opts.credentials, body: opts && opts.body } });
  if (url.indexOf("/api/v1/browser-logs/bootstrap") !== -1) {
    return Promise.resolve({ ok: true, json: () => Promise.resolve({ enabled: true, token: "tok", endpoint: "/api/v1/browser-logs" }) });
  }
  const status = statuses.length ? statuses.shift() : 200;
  return ingestGate.then(() => ({ ok: status < 300, status }));
}
const fire = (type, ev) => (listeners[type] || []).forEach((f) => f(ev));
const later = (ms, f) => setTimeout(f, ms);
vm.runInNewContext(src, { window, document, location, fetch, Date: FakeDate, console: { error() {}, warn() {} }, setTimeout, JSON, Math, Object, Array, String, Number, Promise, RegExp, Error, encodeURIComponent });
fire("error", { message: "boom", filename: "app.js", lineno: 1, colno: 1 });
if (scenario !== "fresh401") now += 61000;
later(20, () => {
  if (scenario === "fresh401") fire("error", { message: "later" });
  if (scenario === "race") {
    fire("error", { message: "second batch" });
    fire("pagehide", {});
    releaseIngest();
    later(20, () => { fire("error", { message: "after recovery" }); fire("pagehide", {}); });
  }
  later(60, () => process.stdout.write(JSON.stringify(calls), () => process.exit(0)));
});
`

type loggerFetch struct {
	URL  string `json:"url"`
	Opts struct {
		Credentials string `json:"credentials"`
		Body        string `json:"body"`
	} `json:"opts"`
}

func runBrowserLogger(t *testing.T, scenario string) []loggerFetch {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed; the browser-logger behaviour test needs it")
	}
	dir := t.TempDir()
	harness := filepath.Join(dir, "harness.js")
	if err := os.WriteFile(harness, []byte(browserLoggerHarness), 0o600); err != nil {
		t.Fatal(err)
	}
	snippet, err := filepath.Abs(filepath.Join("static", "moses-browser-logger.js"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, harness, snippet, scenario).Output()
	if err != nil {
		t.Fatalf("node harness failed: %v", err)
	}
	var calls []loggerFetch
	if err := json.Unmarshal(out, &calls); err != nil {
		t.Fatalf("harness output %q: %v", out, err)
	}
	return calls
}

func splitLoggerCalls(calls []loggerFetch) (bootstraps, ingests []loggerFetch) {
	for _, c := range calls {
		if strings.Contains(c.URL, "/api/v1/browser-logs/bootstrap") {
			bootstraps = append(bootstraps, c)
		} else {
			ingests = append(ingests, c)
		}
	}
	return bootstraps, ingests
}

// The platform's bootstrap finds the app from the page location and applies
// the app's access decision to the caller: the logger must ask same-origin
// with the viewer's credentials, by loc only.
func TestBrowserLogger_BootstrapsSameOriginByLocation(t *testing.T) {
	bootstraps, _ := splitLoggerCalls(runBrowserLogger(t, "expire"))
	if len(bootstraps) == 0 {
		t.Fatal("logger made no bootstrap request")
	}
	b := bootstraps[0]
	want := "/api/v1/browser-logs/bootstrap?loc=" + url.QueryEscape("http://apps.example/apps/t/s/")
	if b.URL != strings.ReplaceAll(want, "+", "%20") {
		t.Errorf("bootstrap URL = %q, want %q (relative, by loc, no ids)", b.URL, want)
	}
	if b.Opts.Credentials != "same-origin" {
		t.Errorf("bootstrap credentials = %q, want same-origin", b.Opts.Credentials)
	}
}

// Tokens are short-lived: an ingest 401 bootstraps again and resends the batch.
func TestBrowserLogger_ExpiredTokenRebootstrapsAndResends(t *testing.T) {
	bootstraps, ingests := splitLoggerCalls(runBrowserLogger(t, "expire"))
	if len(bootstraps) != 2 || len(ingests) != 2 {
		t.Fatalf("want 2 bootstraps and 2 ingests, got %d and %d", len(bootstraps), len(ingests))
	}
	if ingests[1].Opts.Body != ingests[0].Opts.Body {
		t.Errorf("resent batch differs:\n first %s\n again %s", ingests[0].Opts.Body, ingests[1].Opts.Body)
	}
	if ingests[1].Opts.Credentials != "omit" {
		t.Errorf("ingest credentials = %q, want omit (bearer token only)", ingests[1].Opts.Credentials)
	}
}

// Two batches refused at once share ONE re-bootstrap; both are resent and the
// logger keeps running (before: the second 401 silenced it until reload).
func TestBrowserLogger_ConcurrentRefusalsShareOneRebootstrap(t *testing.T) {
	bootstraps, ingests := splitLoggerCalls(runBrowserLogger(t, "race"))
	if len(bootstraps) != 2 {
		t.Fatalf("want 2 bootstraps (install + one re-bootstrap), got %d", len(bootstraps))
	}
	var resent strings.Builder
	for _, c := range ingests[min(2, len(ingests)):] {
		resent.WriteString(c.Opts.Body)
	}
	for _, want := range []string{"boom", "second batch", "after recovery"} {
		if !strings.Contains(resent.String(), want) {
			t.Errorf("%q not sent after the re-bootstrap; sends after the two refused: %s", want, resent.String())
		}
	}
}

// At most one bootstrap a minute: a 401 straight after a bootstrap means the
// token is refused for good, so the logger goes silent instead of looping.
func TestBrowserLogger_FreshTokenRefusedGoesSilent(t *testing.T) {
	bootstraps, ingests := splitLoggerCalls(runBrowserLogger(t, "fresh401"))
	if len(bootstraps) != 1 || len(ingests) != 1 {
		t.Errorf("want 1 bootstrap and 1 ingest (then silence), got %d and %d", len(bootstraps), len(ingests))
	}
}

// CHAT-pxeo.12 acceptance (cross-check, tested directly on the helper — it
// guards every write handler you add): a caller-supplied X-Moses-Tenant-ID
// that disagrees with the deploy-pinned MOSES_TENANT_ID env is rejected
// with 403 + the canonical error envelope. The body must NOT echo any UUID.
func TestEnforceTenantMatch_Mismatch403(t *testing.T) {
	t.Setenv("MOSES_TENANT_ID", "self-tenant-uuid-deploy-pinned")
	t.Setenv("MOSES_DEPLOYED", "")
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "true")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	req.Header.Set("X-Moses-Tenant-ID", "caller-tenant-uuid-different")
	rec := httptest.NewRecorder()

	if !enforceTenantMatch(rec, req) {
		t.Fatal("expected enforceTenantMatch to write the 403 and return true")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if !strings.Contains(got, `"error":"tenant_mismatch"`) {
		t.Errorf("expected tenant_mismatch error, got %q", got)
	}
	if !strings.Contains(got, `"code":"E_TENANT_MISMATCH"`) {
		t.Errorf("expected E_TENANT_MISMATCH code, got %q", got)
	}
	if strings.Contains(got, "caller-tenant-uuid-different") || strings.Contains(got, "self-tenant-uuid-deploy-pinned") {
		t.Errorf("body must NOT echo any tenant UUID; got %q", got)
	}
}

// CHAT-pxeo.12: with the strict check disabled the cross-check is skipped.
func TestEnforceTenantMatch_StrictCheckDisabled(t *testing.T) {
	t.Setenv("MOSES_STRICT_TENANT_CHECK", "false")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	req.Header.Set("X-Moses-Tenant-ID", "caller-different")
	rec := httptest.NewRecorder()
	if enforceTenantMatch(rec, req) {
		t.Errorf("cross-check should be skipped, got 403")
	}
}

// CHAT-pbup: TestEmbeddingHeadersMiddleware verifies that withEmbeddingHeaders
// emits Content-Security-Policy: frame-ancestors per the resolved policy
// AND that X-Frame-Options is only emitted for "denied". The package-level
// resolvedEmbeddingPolicy is set once at init from env vars; we override it
// for each subtest so the assertions are deterministic.
func TestEmbeddingHeadersMiddleware(t *testing.T) {
	saved := resolvedEmbeddingPolicy
	t.Cleanup(func() { resolvedEmbeddingPolicy = saved })

	cases := []struct {
		name           string
		policy         embeddingPolicy
		wantCSP        string
		wantXFOpresent bool
	}{
		{
			name:           "public",
			policy:         embeddingPolicy{cspFrameAncestors: "*"},
			wantCSP:        "frame-ancestors *",
			wantXFOpresent: false,
		},
		{
			name:           "moses-only with multi-origin",
			policy:         embeddingPolicy{cspFrameAncestors: "https://moses.example.com tauri://localhost"},
			wantCSP:        "frame-ancestors https://moses.example.com tauri://localhost",
			wantXFOpresent: false,
		},
		{
			name:           "denied",
			policy:         embeddingPolicy{cspFrameAncestors: "'none'", xFrameOptions: "DENY"},
			wantCSP:        "frame-ancestors 'none'",
			wantXFOpresent: true,
		},
		{
			name:           "with reportUri appended",
			policy:         embeddingPolicy{cspFrameAncestors: "*", reportURI: "/csp-report"},
			wantCSP:        "frame-ancestors *; report-uri /csp-report",
			wantXFOpresent: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolvedEmbeddingPolicy = tc.policy
			h := withEmbeddingHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				_, _ = io.WriteString(w, "<html></html>")
			}))
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			gotCSP := w.Header().Get("Content-Security-Policy")
			if gotCSP != tc.wantCSP {
				t.Errorf("CSP got=%q want=%q", gotCSP, tc.wantCSP)
			}
			gotXFO := w.Header().Get("X-Frame-Options")
			if tc.wantXFOpresent && gotXFO == "" {
				t.Errorf("X-Frame-Options expected non-empty for %s", tc.name)
			}
			if !tc.wantXFOpresent && gotXFO != "" {
				t.Errorf("X-Frame-Options expected empty for %s, got %q", tc.name, gotXFO)
			}
		})
	}
}

// CHAT-il3y6: TestBrowserLoggerSyncCrashCapture is a STRUCTURAL guard on the
// embedded vanilla-JS browser-logger. The bug it locks down: the window
// 'error' / 'unhandledrejection' listeners used to be attached only INSIDE the
// bootstrap `.then(boot => …)` callback, so a render-time crash that fired
// window.onerror before the async bootstrap fetch resolved was lost. The fix
// moves the listener registration ABOVE the bootstrap `.then(` (so it runs
// synchronously when install() is called) and buffers events into a bounded
// pre-bootstrap queue that is drained (enabled) or dropped (disabled/error)
// once bootstrap resolves.
//
// We cannot execute the JS from Go, so this asserts the source ordering and
// the presence of the bounded-buffer / drop machinery — a cheap regression
// gate that fails loudly if someone re-introduces the late-attach bug.
func TestBrowserLoggerSyncCrashCapture(t *testing.T) {
	data, err := staticFiles.ReadFile("static/moses-browser-logger.js")
	if err != nil {
		t.Fatalf("read embedded static/moses-browser-logger.js: %v", err)
	}
	src := string(data)

	// 1. The window 'error' listener must be registered BEFORE the bootstrap
	//    promise .then() — that ordering is the whole fix. We locate the first
	//    addEventListener("error" and the bootstrap .then( and assert the
	//    listener comes first.
	errListenerIdx := strings.Index(src, `addEventListener("error"`)
	if errListenerIdx < 0 {
		t.Fatal(`expected window.addEventListener("error", …) in the snippet`)
	}
	rejListenerIdx := strings.Index(src, `addEventListener("unhandledrejection"`)
	if rejListenerIdx < 0 {
		t.Fatal(`expected window.addEventListener("unhandledrejection", …) in the snippet`)
	}
	thenIdx := strings.Index(src, `bootstrapPromise.then(`)
	if thenIdx < 0 {
		t.Fatal("expected bootstrapPromise.then( in the snippet")
	}
	if errListenerIdx > thenIdx {
		t.Errorf(`window 'error' listener (index %d) is attached AFTER bootstrapPromise.then( (index %d) — `+
			`it must be attached SYNCHRONOUSLY before the bootstrap await or pre-bootstrap crashes are lost`,
			errListenerIdx, thenIdx)
	}
	if rejListenerIdx > thenIdx {
		t.Errorf(`window 'unhandledrejection' listener (index %d) is attached AFTER bootstrapPromise.then( (index %d) — `+
			`it must be attached SYNCHRONOUSLY before the bootstrap await`,
			rejListenerIdx, thenIdx)
	}

	// 2. The console.error/.warn patch must stay POST-bootstrap (it depends on
	//    the fetched patchConsole flag). Its CALL site (not the function
	//    definition near the top of the file) must appear AFTER the .then(.
	consolePatchIdx := strings.Index(src, "= readPatchConsoleFlag()")
	if consolePatchIdx < 0 {
		t.Fatal("expected a `= readPatchConsoleFlag()` call site in the snippet")
	}
	if consolePatchIdx < thenIdx {
		t.Errorf("console patch call (readPatchConsoleFlag, index %d) must stay inside/after the bootstrap .then( (index %d)",
			consolePatchIdx, thenIdx)
	}

	// 3. Bounded pre-bootstrap buffer + drop-on-disabled machinery must exist.
	for _, want := range []string{
		"PREBOOTSTRAP_BUFFER_CAP",
		"flushReady",
		"dropped",
		"queue.length = 0",                       // buffer cleared on disabled/error
		"queue.length > PREBOOTSTRAP_BUFFER_CAP", // drop-oldest bound
	} {
		if !strings.Contains(src, want) {
			t.Errorf("browser-logger snippet missing expected crash-capture token %q", want)
		}
	}

	// 4. There must be exactly ONE window 'error' listener registration — a
	//    second one (re-introduced after bootstrap) would double-count events.
	if n := strings.Count(src, `addEventListener("error"`); n != 1 {
		t.Errorf(`expected exactly one window 'error' listener registration, found %d (a second set double-counts)`, n)
	}
	if n := strings.Count(src, `addEventListener("unhandledrejection"`); n != 1 {
		t.Errorf(`expected exactly one 'unhandledrejection' listener registration, found %d`, n)
	}
}

// WS-F F1: the served spec is normalized to the canonical pattern —
// servers exactly [{url:"/api/v1"}] with all paths RELATIVE to that base.
// The platform's openapi_parser folds servers[0].url + path into the
// registered endpoint path, so given servers[0].url == "/api/v1" a paths
// key that itself starts with /api/ would double-prefix, and /health must
// not be listed (it would register a phantom tool at /api/v1/health).
func TestOpenAPISpec_CanonicalServersAndRelativePaths(t *testing.T) {
	spec := loadOpenAPISpec(t, openAPISpecPath())
	if len(spec.Servers) != 1 || spec.Servers[0].URL != "/api/v1" {
		t.Fatalf("servers must be exactly [{url:\"/api/v1\"}], got %+v", spec.Servers)
	}
	for p := range spec.Paths {
		if !strings.HasPrefix(p, "/") {
			t.Errorf("paths key %q must start with /", p)
		}
		if strings.HasPrefix(p, "/api/") {
			t.Errorf("paths key %q must be relative to servers[0].url (/api/v1) — an /api/-rooted key double-prefixes", p)
		}
		if p == "/health" {
			t.Errorf("/health must not be listed in the served spec — it would register a phantom workspace tool")
		}
	}
}

// WS-F F1 spec↔mux consistency lock: every spec path, folded with
// servers[0].url ("/api/v1") and mounted under MOSES_BASE_PATH, must
// resolve to a registered mux route. This is exactly the address the
// platform's workspace-tool proxy requests (openapi_parser.go folds
// servers[0].url + path; apppath prepends MOSES_BASE_PATH). We resolve
// via mux.Handler (pattern non-empty == route exists) instead of
// ServeHTTP status codes because a registered {id} route legitimately
// answers 404 for a missing resource.
func TestOpenAPISpec_MuxConsistency(t *testing.T) {
	spec := loadOpenAPISpec(t, openAPISpecPath())
	mux := http.NewServeMux()
	registerAPIRoutes(mux, "/apps/t/x")
	param := regexp.MustCompile(`\{[^}]+\}`)
	for p := range spec.Paths {
		urlPath := "/apps/t/x/api/v1" + param.ReplaceAllString(p, "1")
		req := httptest.NewRequest("GET", urlPath, nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Errorf("spec path %q → GET %s does not resolve to any registered route (spec↔mux drift)", p, urlPath)
		}
	}
}

func openAPISpecPath() string {
	return filepath.Join("api", "openapi.json")
}

type openAPISpec struct {
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]json.RawMessage `json:"paths"`
}

func loadOpenAPISpec(t *testing.T, specPath string) openAPISpec {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read OpenAPI spec %s: %v", specPath, err)
	}
	var spec openAPISpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("parse OpenAPI spec %s: %v", specPath, err)
	}
	return spec
}
