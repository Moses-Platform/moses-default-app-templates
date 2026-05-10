// Copyright (c) 2025-2026 Siemer Industries. All rights reserved.
// Licensed under the Business Source License 1.1. See LICENSE file for details.

package main

import (
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
	envMosesAPIBase    = "MOSES_API_BASE"
	envMosesTenantID   = "MOSES_TENANT_ID"
	envMosesChartID    = "MOSES_CHART_ID"
	envMosesAppSlug    = "MOSES_APP_SLUG"
	envWebhookSecret   = "MOSES_CHAT_WEBHOOK_SECRET"
	envMosesDeployed   = "MOSES_DEPLOYED"
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
}

// isProdMode reports whether MOSES_DEPLOYED=1 (the platform sets this in
// the deployed-pod env). Standalone runs (local dev, CI, docker run
// without Moses) get warning-only behavior.
func isProdMode() bool {
	v := strings.TrimSpace(os.Getenv(envMosesDeployed))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// validatePlatformEnv inspects the env and either exits (prod) or warns
// (dev) for each missing required variable. Returns true when all
// required vars were present. The exit hook is parameterised so tests
// can intercept it without crashing the test binary.
func validatePlatformEnv(exit func(int)) bool {
	missing := make([]string, 0, len(requiredPlatformEnv))
	for _, v := range requiredPlatformEnv {
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
// for assertion.
func formatMissingEnvList() string {
	missing := make([]string, 0, len(requiredPlatformEnv))
	for _, v := range requiredPlatformEnv {
		if strings.TrimSpace(os.Getenv(v.name)) == "" {
			missing = append(missing, v.name)
		}
	}
	return fmt.Sprintf("%s", strings.Join(missing, ","))
}
