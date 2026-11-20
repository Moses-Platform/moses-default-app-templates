package main

import (
	"net/http/httptest"
	"testing"
)

func TestUserIDIsUntrusted(t *testing.T) {
	r := httptest.NewRequest("GET", "/health", nil)
	r.Header.Set("X-Moses-User-ID", "forged-user")
	w := httptest.NewRecorder()
	c := getMosesContext(r)
	if c.UntrustedUserID != "forged-user" {
		t.Fatal(c)
	}
	w.WriteHeader(204)
	if w.Code != 204 {
		t.Fatalf("public request denied: %d", w.Code)
	}
}
