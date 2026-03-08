package handler

import (
	"net/http"
	"os"
	"path/filepath"
)

func OpenAPI(w http.ResponseWriter, r *http.Request) {
	// Try relative path first (development), then absolute (container)
	paths := []string{
		"api/openapi.json",
		filepath.Join(filepath.Dir(os.Args[0]), "api", "openapi.json"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			w.Header().Set("Content-Type", "application/json")
			http.ServeFile(w, r, p)
			return
		}
	}

	http.NotFound(w, r)
}
