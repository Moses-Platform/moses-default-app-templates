// Copyright (c) 2025-2026 Siemer Industries. All rights reserved.
// Licensed under the Business Source License 1.1. See LICENSE file for details.

package main

import (
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
		envMosesDeployed: "1",
		envMosesAPIBase:  "https://moses.example.com",
		envMosesTenantID: "00000000-0000-0000-0000-000000000001",
		envMosesChartID:  "00000000-0000-0000-0000-000000000002",
		envMosesAppSlug:  "fullstack-chat",
		envWebhookSecret: "abc123",
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
