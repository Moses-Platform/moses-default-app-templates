// Copyright (c) 2025-2026 Siemer Industries. All rights reserved.
// Licensed under the Business Source License 1.1. See LICENSE file for details.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

// CHAT-0b6g (DEPS-B1): startup-validate the platform-injected env
// contract that fullstack-chat depends on. The contract:
//
//   Required (DEPS-A1):
//     MOSES_API_BASE      — backend → platform invokes
//     MOSES_TENANT_ID     — backend self-identification
//     MOSES_CHART_ID      — scope queries to own chart
//     MOSES_APP_SLUG      — webhook-claim verification
//
//   Required when chat_prompt declared (DEPS-A2):
//     MOSES_CHAT_WEBHOOK_SECRET — HMAC verification of completion webhook
//
//   Optional:
//     MOSES_DEPLOYMENT_ID, MOSES_USER_ID, MOSES_BASE_PATH,
//     MOSES_EMBEDDING_*, MOSES_CHAT_WEBHOOK_SECRET_PREVIOUS
//
// Behavior is environment-gated:
//   - MOSES_DEPLOYED=1 (set by the chart at platform deploy time): missing
//     required vars → log.Fatal with structured remediation.
//   - Otherwise (local dev / CI / docker run): log.Warn per missing var
//     listing what features will fail. The app continues and falls back
//     to local-dev defaults where possible.
//
// validatePlatformEnv returns a boolean indicating whether all required
// vars are present. Tests can call this directly without invoking
// log.Fatal by mocking out the os.Exit path via the validateExit helper.

const (
	envMosesAPIBase         = "MOSES_API_BASE"
	envMosesTenantID        = "MOSES_TENANT_ID"
	envMosesChartID         = "MOSES_CHART_ID"
	envMosesAppSlug         = "MOSES_APP_SLUG"
	envWebhookSecret        = "MOSES_CHAT_WEBHOOK_SECRET"
	envMosesInternalAPIBase = "MOSES_INTERNAL_API_BASE"
	envMosesDeployed        = "MOSES_DEPLOYED"
)

// requiredPlatformEnv is the canonical list of platform-injected vars the
// fullstack-chat backend depends on. Kept here so the test fixture and the
// runtime check stay in lockstep.
var requiredPlatformEnv = []struct {
	name        string
	description string
	chatPrompt  bool // only required when the app declares a chat_prompt action
}{
	{envMosesAPIBase, "MOSES_API_BASE — origin of the Moses platform API", false},
	// CHAT-pxeo.12: MOSES_TENANT_ID is now authoritative for storage keys
	// (read via internal/config.SelfTenantID()). Already-deployed apps
	// without it will fail-fast in main() via config.Validate().
	{envMosesTenantID, "MOSES_TENANT_ID — backend self-identification (REQUIRED — storage key)", false},
	{envMosesChartID, "MOSES_CHART_ID — scope queries to own chart", false},
	{envMosesAppSlug, "MOSES_APP_SLUG — webhook-claim verification", false},
	{envWebhookSecret, "MOSES_CHAT_WEBHOOK_SECRET — HMAC verification (chat_prompt apps)", true},
	// CHAT-pswm.1/.9: MOSES_INTERNAL_API_BASE is the cluster-DNS base
	// the mosesproxy-go handler forwards to. Without it, /__moses/invoke
	// returns 503 moses_unconfigured and the in-iframe SDK fires "Moses
	// SDK error" on every click — same gating as the webhook secret
	// because both are required only when chat_prompt is declared.
	{envMosesInternalAPIBase, "MOSES_INTERNAL_API_BASE — cluster-internal moses-backend FQDN (mosesproxy-go forward target)", true},
}

// isProdMode reports whether MOSES_DEPLOYED=1 (the platform sets this in
// the deployed-pod env). Standalone runs (local dev, CI, docker run
// without Moses) get warning-only behavior.
func isProdMode() bool {
	v := strings.TrimSpace(os.Getenv(envMosesDeployed))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// chatPromptActionDeclared reports whether the deployed
// `moses-app.config.json` (if discoverable next to the binary) declares
// at least one `platformActions[].type == "chat_prompt"` entry.
//
// CHAT-ct5q: gates the chatPrompt-flagged env vars so a fork that drops
// chat_prompt actions doesn't fail-fast on missing
// MOSES_CHAT_WEBHOOK_SECRET. Resolution rules:
//
//   - MOSES_APP_CONFIG_PATH (env override) wins if set.
//   - Else search a few standard locations next to the binary.
//   - If no config is found OR parsing fails, RETURN TRUE — preserve
//     the historical "always required" behaviour so the strict default
//     still protects the canonical fullstack-chat shape. Forks that
//     drop chat_prompt opt into the gate by ensuring the config travels
//     with the image (Dockerfile COPY) at a discoverable path.
func chatPromptActionDeclared() bool {
	candidates := []string{}
	if p := strings.TrimSpace(os.Getenv("MOSES_APP_CONFIG_PATH")); p != "" {
		candidates = append(candidates, p)
	}
	candidates = append(candidates,
		"./moses-app.config.json",
		"/app/moses-app.config.json",
		"../moses-app.config.json",
	)

	var data []byte
	var readErr error
	found := ""
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			data = b
			found = p
			break
		}
		readErr = err
	}
	if data == nil {
		log.Printf("CHAT-ct5q: moses-app.config.json not found (last err: %v); defaulting to strict chat_prompt-required mode", readErr)
		return true
	}

	var cfg struct {
		PlatformActions []struct {
			Type string `json:"type"`
		} `json:"platformActions"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("CHAT-ct5q: failed to parse %s: %v; defaulting to strict chat_prompt-required mode", found, err)
		return true
	}
	for _, a := range cfg.PlatformActions {
		if a.Type == "chat_prompt" {
			return true
		}
	}
	log.Printf("CHAT-ct5q: %s declares no chat_prompt actions; chatPrompt-flagged env vars are NOT required", found)
	return false
}

// validatePlatformEnv inspects the env and either exits (prod) or warns
// (dev) for each missing required variable. Returns true when all
// required vars were present. The exit hook is parameterised so tests
// can intercept it without crashing the test binary.
func validatePlatformEnv(exit func(int)) bool {
	chatPromptRequired := chatPromptActionDeclared()
	missing := make([]string, 0, len(requiredPlatformEnv))
	for _, v := range requiredPlatformEnv {
		// CHAT-ct5q: skip chatPrompt-flagged entries when the deployed
		// config declares no chat_prompt action.
		if v.chatPrompt && !chatPromptRequired {
			continue
		}
		if strings.TrimSpace(os.Getenv(v.name)) == "" {
			missing = append(missing, v.description)
		}
	}
	if len(missing) == 0 {
		log.Println("CHAT-0b6g: platform env contract validated — all required vars present")
		return true
	}

	if isProdMode() {
		log.Printf("CHAT-0b6g: missing %d required platform env vars in production mode (MOSES_DEPLOYED=1):", len(missing))
		for _, m := range missing {
			log.Printf("  - %s", m)
		}
		log.Printf("Remediation: ensure the chart's deployment.yaml iterates .Values.env / .Values.globalEnv (CHAT-zber, DEPS-A1b) so platform-injected env vars land in the pod. See moses-deployment-guide § Platform-injected runtime env.")
		if exit != nil {
			exit(1)
		}
		return false
	}

	// Dev mode: warn-and-continue.
	log.Printf("CHAT-0b6g: %d platform env vars are unset (running in standalone mode — set MOSES_DEPLOYED=1 to fail-fast):", len(missing))
	for _, m := range missing {
		log.Printf("  - WARN: %s", m)
	}
	return false
}

// formatMissingEnvList is a tiny helper exported for the test harness.
// It produces a comma-separated list of missing env names (no descriptions)
// for assertion. Honors the chatPrompt gate (CHAT-ct5q) so it stays in
// lockstep with validatePlatformEnv.
func formatMissingEnvList() string {
	chatPromptRequired := chatPromptActionDeclared()
	missing := make([]string, 0, len(requiredPlatformEnv))
	for _, v := range requiredPlatformEnv {
		if v.chatPrompt && !chatPromptRequired {
			continue
		}
		if strings.TrimSpace(os.Getenv(v.name)) == "" {
			missing = append(missing, v.name)
		}
	}
	return fmt.Sprintf("%s", strings.Join(missing, ","))
}
