package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserIDIsUntrusted(t *testing.T) {
	r := httptest.NewRequest("GET", "/health", nil)
	r.Header.Set("X-Moses-User-ID", "forged-user")
	w := httptest.NewRecorder()
	MosesHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetMosesContext(r).UntrustedUserID != "forged-user" {
			t.Fatal("lost caller context")
		}
		w.WriteHeader(204)
	})).ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("public request denied: %d", w.Code)
	}
}
