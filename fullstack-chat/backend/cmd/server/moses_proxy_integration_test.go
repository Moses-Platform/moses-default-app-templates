// Copyright (c) 2025-2026 Siemer Industries. All rights reserved.
// Licensed under the Business Source License 1.1. See LICENSE file for details.

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moses-platform/fullstack-chat/internal/mosesproxy"
)

// CHAT-pswm.3 §C — cross-repo integration test (proxy → simulated
// moses-backend handler).
//
// The unit tests under shared/mosesproxy-go/proxy_test.go already cover
// the 20-case behavioural matrix in isolation. This file additionally
// pins the wiring that fullstack-chat's main() does:
//
//   - mosesproxy.NewHandler is mounted at mosesproxy.InvokePath (the
//     well-known path the in-iframe SDK posts to).
//   - cfg.RequireRequestedWith = true is the canonical fullstack-chat
//     gate (X-Requested-With: moses-iframe required).
//   - The forwarded request to the upstream "moses-backend" carries
//     Authorization: Bearer <jwt>, Content-Type: application/json, and
//     X-Tenant-ID when configured.
//   - Set-Cookie headers from the upstream are stripped (do not leak
//     moses-backend cookies into the iframe's origin).
//   - The forward URL matches the spec shape:
//       ${MOSES_INTERNAL_API_BASE}/api/v1/apps/${MOSES_APP_SLUG}/actions/${actionId}/invoke
//
// Wiring smoke for main()'s actual mount path — base-path subpath —
// is exercised by mountFullChain below.
//
// We deliberately do NOT pull in the platform-prep backend's real
// HandleInvokeUserScoped handler to keep this template self-contained
// (consumed-not-embedded contract). The "moses-backend behaviour"
// the template depends on is exactly: accept Bearer-only POSTs and
// honour X-Tenant-ID. Both are pinned by §A's full-middleware tests
// in the platform repo (iframe_sdk_handler_test.go siblings). The
// §C test here is the templates-side half of the same contract.

// mountFullChain rebuilds the relevant slice of cmd/server/main.go's
// router so the test exercises the SAME wiring as a real pod boot:
// the proxy lives under InvokePath, optionally prefixed by basePath.
func mountFullChain(cfg mosesproxy.Config, basePath string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(mosesproxy.InvokePath, mosesproxy.NewHandler(cfg))
	if basePath != "" {
		mux.HandleFunc(strings.TrimSuffix(basePath, "/")+mosesproxy.InvokePath, mosesproxy.NewHandler(cfg))
	}
	return mux
}

// forwardCapture records the request the proxy emits to the upstream
// (stand-in for moses-backend). Per-test instances are created via
// newForwardCapture(t).
type forwardCapture struct {
	server *httptest.Server
	// Latest forwarded request snapshot. Read after the proxy call returns.
	method        string
	path          string
	authHeader    string
	tenantHeader  string
	contentType   string
	requestedWith string
	bodyBytes     []byte
	// Behaviour knobs.
	respondStatus      int
	respondBody        []byte
	respondContentType string
	emitSetCookie      bool
}

func newForwardCapture(t *testing.T) *forwardCapture {
	t.Helper()
	fc := &forwardCapture{
		respondStatus:      http.StatusOK,
		respondBody:        []byte(`{"status":"succeeded","result":{"conversationId":"conv-int-1"},"invocationId":"00000000-0000-0000-0000-000000000abc"}`),
		respondContentType: "application/json",
	}
	fc.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fc.method = r.Method
		fc.path = r.URL.Path
		fc.authHeader = r.Header.Get("Authorization")
		fc.tenantHeader = r.Header.Get("X-Tenant-ID")
		fc.contentType = r.Header.Get("Content-Type")
		fc.requestedWith = r.Header.Get(mosesproxy.RequestedWithHeader)
		b, _ := io.ReadAll(r.Body)
		fc.bodyBytes = b
		if fc.emitSetCookie {
			// Upstream sets a session cookie scoped to its own origin. The
			// proxy MUST NOT leak this back to the iframe.
			w.Header().Set("Set-Cookie", "moses_session=secret-value; Path=/; HttpOnly")
		}
		w.Header().Set("Content-Type", fc.respondContentType)
		w.WriteHeader(fc.respondStatus)
		_, _ = w.Write(fc.respondBody)
	}))
	t.Cleanup(fc.server.Close)
	return fc
}

func newInvokeRequest(t *testing.T, host, actionID, topic string) *http.Request {
	t.Helper()
	payload, _ := json.Marshal(map[string]interface{}{
		"actionId":  actionID,
		"variables": map[string]string{"topic": topic},
	})
	req := httptest.NewRequest(http.MethodPost, host+mosesproxy.InvokePath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestIntegration_BearerForward_ReachesUpstream_WithExpectedShape
// — pins the canonical SDK contract: Bearer-only POST through the
// proxy reaches the upstream with Authorization preserved, JSON
// Content-Type, and the chartId folded into the forward body.
func TestIntegration_BearerForward_ReachesUpstream_WithExpectedShape(t *testing.T) {
	fc := newForwardCapture(t)
	cfg := mosesproxy.Config{
		InternalAPIBase:      fc.server.URL,
		AppSlug:              "fullstack-chat",
		ChartID:              "11111111-1111-1111-1111-111111111111",
		TenantID:             "22222222-2222-2222-2222-222222222222",
		RequireRequestedWith: true,
	}
	h := mountFullChain(cfg, "")

	req := newInvokeRequest(t, "http://app.local", "generate-entry", "octopus")
	req.Header.Set("Authorization", "Bearer user.jwt.value")
	req.Header.Set(mosesproxy.RequestedWithHeader, mosesproxy.RequestedWithValue)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	// Forwarded path must match the spec.
	wantPath := "/api/v1/apps/fullstack-chat/actions/generate-entry/invoke"
	if fc.path != wantPath {
		t.Fatalf("upstream path = %q, want %q", fc.path, wantPath)
	}
	if fc.method != http.MethodPost {
		t.Fatalf("upstream method = %q, want POST", fc.method)
	}

	// Bearer preserved verbatim.
	if fc.authHeader != "Bearer user.jwt.value" {
		t.Fatalf("upstream Authorization = %q, want Bearer user.jwt.value", fc.authHeader)
	}
	if fc.contentType != "application/json" {
		t.Fatalf("upstream Content-Type = %q, want application/json", fc.contentType)
	}

	// X-Tenant-ID forwarded when configured.
	if fc.tenantHeader != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("upstream X-Tenant-ID = %q, want the configured TenantID", fc.tenantHeader)
	}

	// X-Requested-With is the SDK-side gate; the proxy does NOT forward
	// it upstream (moses-backend doesn't gate on it). Explicitly assert
	// to lock the boundary.
	if fc.requestedWith != "" {
		t.Fatalf("upstream X-Requested-With = %q, want empty (gate is proxy-local only)", fc.requestedWith)
	}

	// Forward body shape: { variables: { topic }, chartId }.
	var forwarded struct {
		Variables map[string]interface{} `json:"variables"`
		ChartID   string                 `json:"chartId"`
	}
	if err := json.Unmarshal(fc.bodyBytes, &forwarded); err != nil {
		t.Fatalf("failed to parse forwarded body %q: %v", string(fc.bodyBytes), err)
	}
	if forwarded.Variables["topic"] != "octopus" {
		t.Fatalf("forwarded variables.topic = %v, want octopus", forwarded.Variables["topic"])
	}
	if forwarded.ChartID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("forwarded chartId = %q, want the configured ChartID", forwarded.ChartID)
	}
}

// TestIntegration_CookieForward_ReachesUpstream_AsBearer
// — pins the Chrome-on-Minikube path: the iframe carries only an
// access_token cookie, the proxy must rewrite it into a Bearer header
// for the upstream. This is the lift that makes the round-trip work
// without the Tauri AUTH_HEADER_INJECTION_SHIM.
func TestIntegration_CookieForward_ReachesUpstream_AsBearer(t *testing.T) {
	fc := newForwardCapture(t)
	cfg := mosesproxy.Config{
		InternalAPIBase:      fc.server.URL,
		AppSlug:              "fullstack-chat",
		RequireRequestedWith: true,
	}
	h := mountFullChain(cfg, "")

	req := newInvokeRequest(t, "http://app.local", "generate-entry", "walrus")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie.jwt.value"})
	req.Header.Set(mosesproxy.RequestedWithHeader, mosesproxy.RequestedWithValue)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fc.authHeader != "Bearer cookie.jwt.value" {
		t.Fatalf("upstream Authorization = %q, want Bearer cookie.jwt.value (cookie lifted into Bearer)", fc.authHeader)
	}
}

// TestIntegration_OmitsXTenantWhenNotConfigured
// — moses-backend's multi-tenant resolution falls back to JWT claims
// when no X-Tenant-ID header is present; templates that do not set
// MOSES_TENANT_ID should NOT have a stray empty header forwarded.
func TestIntegration_OmitsXTenantWhenNotConfigured(t *testing.T) {
	fc := newForwardCapture(t)
	cfg := mosesproxy.Config{
		InternalAPIBase:      fc.server.URL,
		AppSlug:              "fullstack-chat",
		RequireRequestedWith: true,
		// TenantID intentionally empty.
	}
	h := mountFullChain(cfg, "")

	req := newInvokeRequest(t, "http://app.local", "generate-entry", "kraken")
	req.Header.Set("Authorization", "Bearer user.jwt.value")
	req.Header.Set(mosesproxy.RequestedWithHeader, mosesproxy.RequestedWithValue)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fc.tenantHeader != "" {
		t.Fatalf("upstream X-Tenant-ID = %q, want empty when TenantID is not configured", fc.tenantHeader)
	}
}

// TestIntegration_StripsUpstreamSetCookie
// — moses-backend may emit Set-Cookie scoped to its own service DNS
// (e.g. session refresh). The proxy MUST NOT pass that through to the
// iframe's origin where it would land as an opaque, unusable cookie
// on the app's nginx subpath. CHAT-pswm.8 security invariant.
func TestIntegration_StripsUpstreamSetCookie(t *testing.T) {
	fc := newForwardCapture(t)
	fc.emitSetCookie = true
	cfg := mosesproxy.Config{
		InternalAPIBase:      fc.server.URL,
		AppSlug:              "fullstack-chat",
		RequireRequestedWith: true,
	}
	h := mountFullChain(cfg, "")

	req := newInvokeRequest(t, "http://app.local", "generate-entry", "narwhal")
	req.Header.Set("Authorization", "Bearer user.jwt.value")
	req.Header.Set(mosesproxy.RequestedWithHeader, mosesproxy.RequestedWithValue)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("proxy passed upstream Set-Cookie back to caller: %q — must be stripped", got)
	}
}

// TestIntegration_BasePathMount_RoutesProxyThroughSubpath
// — fullstack-chat mounts both the root path AND the basePath alias
// for sub-path deploys (main.go:registerAPI). Verify the proxy reaches
// the upstream when the iframe lives at /apps/{tenant}/{slug}/ and the
// SDK posts to a base-path-relative /__moses/invoke.
func TestIntegration_BasePathMount_RoutesProxyThroughSubpath(t *testing.T) {
	fc := newForwardCapture(t)
	cfg := mosesproxy.Config{
		InternalAPIBase:      fc.server.URL,
		AppSlug:              "fullstack-chat",
		RequireRequestedWith: true,
	}
	const basePath = "/apps/system-s-workspace/app-deadbeef-stable"
	h := mountFullChain(cfg, basePath)

	payload, _ := json.Marshal(map[string]interface{}{
		"actionId":  "generate-entry",
		"variables": map[string]string{"topic": "dolphin"},
	})
	req := httptest.NewRequest(http.MethodPost, "http://app.local"+basePath+mosesproxy.InvokePath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer user.jwt.value")
	req.Header.Set(mosesproxy.RequestedWithHeader, mosesproxy.RequestedWithValue)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("base-path mount: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if fc.method != http.MethodPost {
		t.Fatalf("base-path mount: upstream method = %q, want POST", fc.method)
	}
}

// TestIntegration_RequireRequestedWith_RejectsMissingHeader
// — the canonical fullstack-chat wiring sets cfg.RequireRequestedWith = true
// (CHAT-pswm.9). Pin the 403 path so a future edit that flips the gate off
// goes red here before reaching the test cluster.
func TestIntegration_RequireRequestedWith_RejectsMissingHeader(t *testing.T) {
	fc := newForwardCapture(t)
	cfg := mosesproxy.Config{
		InternalAPIBase:      fc.server.URL,
		AppSlug:              "fullstack-chat",
		RequireRequestedWith: true,
	}
	h := mountFullChain(cfg, "")

	req := newInvokeRequest(t, "http://app.local", "generate-entry", "manatee")
	req.Header.Set("Authorization", "Bearer user.jwt.value")
	// Deliberately OMIT the X-Requested-With header.

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403 (missing X-Requested-With), got %d body=%s", rec.Code, rec.Body.String())
	}
	// And the upstream MUST NOT have been hit.
	if fc.method != "" {
		t.Fatalf("upstream was reached despite missing X-Requested-With: method=%q path=%q", fc.method, fc.path)
	}
}
