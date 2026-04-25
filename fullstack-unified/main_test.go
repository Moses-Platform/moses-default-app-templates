package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

// TestRenderIndex_SubstitutesMosesConfig verifies the BLF-J server-side
// substitution: chart/deployment/api-base values are interpolated into the
// <meta name="moses-config"> tag the browser-logger snippet reads.
func TestRenderIndex_SubstitutesMosesConfig(t *testing.T) {
	loadIndexTemplate()
	if indexTemplate == nil {
		t.Fatal("indexTemplate is nil after loadIndexTemplate; embedded index.html missing or unparseable")
	}

	w := httptest.NewRecorder()
	if !renderIndex(w, indexContext{
		ChartID:      "chart-123",
		DeploymentID: "dep-abc",
		APIBase:      "http://moses.local",
	}) {
		t.Fatal("renderIndex returned false despite indexTemplate being set")
	}

	body := w.Body.String()
	for _, want := range []string{
		`name="moses-config"`,
		`data-chart-id="chart-123"`,
		`data-deployment-id="dep-abc"`,
		`data-api-base="http://moses.local"`,
		`moses-browser-logger.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("rendered index missing %q\n--- body ---\n%s", want, body)
		}
	}
}

// TestRenderIndex_EmptyContextStillRendersTag verifies that when the env
// vars are absent the meta tag is still emitted (with empty data-* values),
// because the snippet's readConfig() short-circuits on missing IDs and that
// branch is the one we rely on for "built outside Moses" no-op behaviour.
func TestRenderIndex_EmptyContextStillRendersTag(t *testing.T) {
	loadIndexTemplate()
	if indexTemplate == nil {
		t.Fatal("indexTemplate is nil after loadIndexTemplate")
	}

	w := httptest.NewRecorder()
	if !renderIndex(w, indexContext{}) {
		t.Fatal("renderIndex returned false despite indexTemplate being set")
	}

	body := w.Body.String()
	if !strings.Contains(body, `name="moses-config"`) {
		t.Errorf("expected <meta name=\"moses-config\"> tag even with empty context\n--- body ---\n%s", body)
	}
	if !strings.Contains(body, `data-chart-id=""`) {
		t.Errorf("expected empty data-chart-id when env var missing\n--- body ---\n%s", body)
	}
}
