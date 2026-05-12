// Copyright (c) 2025-2026 Siemer Industries. All rights reserved.
// Licensed under the Business Source License 1.1. See LICENSE file for details.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// CHAT-0b6g (DEPS-B1): startup-validation tests.

func setEnvMap(t *testing.T, m map[string]string) {
	t.Helper()
	for k, v := range m {
		t.Setenv(k, v)
	}
}

func TestValidatePlatformEnv_ProdMode_AllPresent_NoExit(t *testing.T) {
	setEnvMap(t, map[string]string{
		envMosesDeployed:        "1",
		envMosesAPIBase:         "https://moses.example.com",
		envMosesTenantID:        "00000000-0000-0000-0000-000000000001",
		envMosesChartID:         "00000000-0000-0000-0000-000000000002",
		envMosesAppSlug:         "fullstack-chat",
		envWebhookSecret:        "abc123",
		envMosesInternalAPIBase: "http://moses-backend.moses.svc.cluster.local:8080",
	})

	exitCalled := false
	exit := func(int) { exitCalled = true }
	ok := validatePlatformEnv(exit)
	if !ok {
		t.Errorf("expected all required env vars to validate as present")
	}
	if exitCalled {
		t.Errorf("exit must NOT be called when all required vars are present")
	}
}

func TestValidatePlatformEnv_ProdMode_MissingRequired_Exits(t *testing.T) {
	// Clear all required vars, set prod mode.
	for _, v := range requiredPlatformEnv {
		t.Setenv(v.name, "")
	}
	t.Setenv(envMosesDeployed, "1")

	exitCalled := false
	exitCode := -1
	exit := func(c int) { exitCalled = true; exitCode = c }
	ok := validatePlatformEnv(exit)
	if ok {
		t.Errorf("expected validate to return false when prod-mode required vars are missing")
	}
	if !exitCalled {
		t.Errorf("expected exit to be called in prod mode with missing required vars")
	}
	if exitCode == 0 {
		t.Errorf("expected non-zero exit code, got %d", exitCode)
	}
}

func TestValidatePlatformEnv_DevMode_MissingRequired_DoesNotExit(t *testing.T) {
	// Clear all required vars, leave MOSES_DEPLOYED unset (dev mode).
	for _, v := range requiredPlatformEnv {
		t.Setenv(v.name, "")
	}
	t.Setenv(envMosesDeployed, "")

	exitCalled := false
	exit := func(int) { exitCalled = true }
	ok := validatePlatformEnv(exit)
	if ok {
		t.Errorf("expected validate to return false (vars are absent)")
	}
	if exitCalled {
		t.Errorf("dev mode must NOT call exit even when required vars are missing — only warn")
	}
}

// writeConfigJSON writes a moses-app.config.json fixture to a tmp file and
// returns its path. It also sets MOSES_APP_CONFIG_PATH for the test.
func writeConfigJSON(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "moses-app.config.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	t.Setenv("MOSES_APP_CONFIG_PATH", p)
	return p
}

// CHAT-ct5q: a fork without chat_prompt actions deploys in prod mode
// WITHOUT setting MOSES_CHAT_WEBHOOK_SECRET.
func TestValidatePlatformEnv_NoChatPromptAction_WebhookSecretNotRequired(t *testing.T) {
	writeConfigJSON(t, `{
        "name": "fork-without-chat-prompt",
        "platformActions": [
            {"id": "do", "type": "launch_agent"}
        ]
    }`)
	setEnvMap(t, map[string]string{
		envMosesDeployed:        "1",
		envMosesAPIBase:         "https://moses.example.com",
		envMosesTenantID:        "00000000-0000-0000-0000-000000000001",
		envMosesChartID:         "00000000-0000-0000-0000-000000000002",
		envMosesAppSlug:         "fork-without-chat-prompt",
		envWebhookSecret:        "", // deliberately missing (no chat_prompt → not required)
		envMosesInternalAPIBase: "", // deliberately missing (no chat_prompt → not required)
	})

	exitCalled := false
	exit := func(int) { exitCalled = true }
	ok := validatePlatformEnv(exit)
	if !ok {
		t.Errorf("expected validation to pass when config declares no chat_prompt action and webhook secret is absent")
	}
	if exitCalled {
		t.Errorf("exit must NOT be called when chat_prompt is not declared and webhook secret is absent")
	}
}

// CHAT-ct5q: a fork WITH chat_prompt declared continues to fail-fast on
// missing webhook secret — no regression for canonical fullstack-chat.
func TestValidatePlatformEnv_ChatPromptDeclared_WebhookSecretRequired(t *testing.T) {
	writeConfigJSON(t, `{
        "name": "fork-with-chat-prompt",
        "platformActions": [
            {"id": "do", "type": "chat_prompt"}
        ]
    }`)
	setEnvMap(t, map[string]string{
		envMosesDeployed:        "1",
		envMosesAPIBase:         "https://moses.example.com",
		envMosesTenantID:        "00000000-0000-0000-0000-000000000001",
		envMosesChartID:         "00000000-0000-0000-0000-000000000002",
		envMosesAppSlug:         "fork-with-chat-prompt",
		envWebhookSecret:        "",                                                   // missing → must fail-fast
		envMosesInternalAPIBase: "http://moses-backend.moses.svc.cluster.local:8080", // present; the webhook secret is what's missing
	})

	exitCalled := false
	exit := func(int) { exitCalled = true }
	ok := validatePlatformEnv(exit)
	if ok {
		t.Errorf("expected validation to fail when chat_prompt is declared and webhook secret is missing")
	}
	if !exitCalled {
		t.Errorf("expected exit to be called in prod mode when webhook secret is missing for a chat_prompt-declared fork")
	}
}

// CHAT-pswm.9: chat_prompt-declared fork ALSO fails-fast when
// MOSES_INTERNAL_API_BASE is missing — mosesproxy-go would otherwise
// return 503 moses_unconfigured on every iframe click. Gated on
// chat_prompt presence because forks that drop chat_prompt do not mount
// the proxy.
func TestValidatePlatformEnv_ChatPromptDeclared_InternalAPIBaseRequired(t *testing.T) {
	writeConfigJSON(t, `{
        "name": "fork-with-chat-prompt",
        "platformActions": [
            {"id": "do", "type": "chat_prompt"}
        ]
    }`)
	setEnvMap(t, map[string]string{
		envMosesDeployed:        "1",
		envMosesAPIBase:         "https://moses.example.com",
		envMosesTenantID:        "00000000-0000-0000-0000-000000000001",
		envMosesChartID:         "00000000-0000-0000-0000-000000000002",
		envMosesAppSlug:         "fork-with-chat-prompt",
		envWebhookSecret:        "abc123",
		envMosesInternalAPIBase: "", // missing → must fail-fast
	})

	exitCalled := false
	exit := func(int) { exitCalled = true }
	ok := validatePlatformEnv(exit)
	if ok {
		t.Errorf("expected validation to fail when chat_prompt is declared and MOSES_INTERNAL_API_BASE is missing")
	}
	if !exitCalled {
		t.Errorf("expected exit to be called when MOSES_INTERNAL_API_BASE is missing for a chat_prompt-declared fork")
	}
}

// CHAT-ct5q: missing/unreadable config falls back to strict mode
// (preserves historical behaviour — webhook secret stays required).
func TestValidatePlatformEnv_NoConfigFile_FallsBackToStrict(t *testing.T) {
	// Point MOSES_APP_CONFIG_PATH at a non-existent file and ensure no
	// stray ./moses-app.config.json is in the test cwd.
	t.Setenv("MOSES_APP_CONFIG_PATH", filepath.Join(t.TempDir(), "does-not-exist.json"))
	setEnvMap(t, map[string]string{
		envMosesDeployed: "1",
		envMosesAPIBase:  "https://moses.example.com",
		envMosesTenantID: "00000000-0000-0000-0000-000000000001",
		envMosesChartID:  "00000000-0000-0000-0000-000000000002",
		envMosesAppSlug:  "fullstack-chat",
		envWebhookSecret: "",
	})

	// chdir to tmpdir so the ./moses-app.config.json fallback path also misses.
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	exitCalled := false
	exit := func(int) { exitCalled = true }
	ok := validatePlatformEnv(exit)
	if ok {
		t.Errorf("expected validation to fail when webhook secret is missing and config cannot be located (strict fallback)")
	}
	if !exitCalled {
		t.Errorf("expected exit to be called under strict fallback when webhook secret is missing")
	}
}

func TestIsProdMode_HonorsMultipleTruthyValues(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"false": false,
		"1":     true,
		"true":  true,
		"True":  true,
		"YES":   true,
		"yes":   true,
	}
	for v, want := range cases {
		t.Setenv(envMosesDeployed, v)
		got := isProdMode()
		if got != want {
			t.Errorf("isProdMode(%q) = %v, want %v", v, got, want)
		}
	}
}
