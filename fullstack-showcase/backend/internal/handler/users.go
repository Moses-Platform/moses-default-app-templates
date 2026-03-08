package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// PlatformUser matches the Moses Platform API response shape.
type PlatformUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

// UsersHandler serves tenant user data via the Moses Platform API.
type UsersHandler struct {
	apiKey  string
	apiURL  string
	client  *http.Client
}

// NewUsersHandler creates a handler that reads platform credentials from env.
func NewUsersHandler() *UsersHandler {
	return &UsersHandler{
		apiKey: os.Getenv("MOSES_PLATFORM_API_KEY"),
		apiURL: os.Getenv("MOSES_PLATFORM_API_URL"),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Users handles GET /api/v1/users — returns tenant users from Moses Platform API.
func (h *UsersHandler) Users(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Graceful degradation: if platform credentials not available, return empty list
	if h.apiKey == "" || h.apiURL == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users":   []PlatformUser{},
			"message": "Platform API not configured — approve the moses-platform integration grant to enable",
		})
		return
	}

	// Call Moses Platform API
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.apiURL+"/api/v1/platform/users", nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("X-API-Key", h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("platform API call failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("platform API returned %d: %s", resp.StatusCode, string(body)), resp.StatusCode)
		return
	}

	// Forward the response directly
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}
