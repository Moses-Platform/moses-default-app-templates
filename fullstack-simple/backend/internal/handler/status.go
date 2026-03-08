package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"
)

var startTime = time.Now()

func Status(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"app":     "fullstack-simple",
		"version": "1.0.0",
		"uptime":  time.Since(startTime).String(),
		"moses": map[string]string{
			"tenant_id":  r.Header.Get("X-Moses-Tenant-ID"),
			"user_id":    r.Header.Get("X-Moses-User-ID"),
			"chart_id":   r.Header.Get("X-Moses-Chart-ID"),
			"request_id": r.Header.Get("X-Moses-Request-ID"),
		},
		"env": map[string]string{
			"port":     os.Getenv("PORT"),
			"base_url": os.Getenv("BASE_URL"),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
