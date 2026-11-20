package main

import (
	"github.com/moses-platform/fullstack-oidc/internal/oidcauth"
	"net/http"
	"testing"
)

func TestStreaming(t *testing.T) {
	auth := oidcauth.New(oidcauth.Config{Issuer: "https://idp.invalid", ClientID: "app", ClientSecret: "secret", CookieSecret: []byte("01234567890123456789012345678901"), ProtectedPaths: []string{"/private"}})
	testStreaming(t, func(next http.Handler) http.Handler { return buildHandler(auth, next) })
}
