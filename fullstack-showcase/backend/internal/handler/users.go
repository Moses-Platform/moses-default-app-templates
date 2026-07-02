package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	apiKey string
	apiURL string
	client *http.Client
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
		// Detail stays server-side (it can embed the configured URL);
		// the browser gets a generic envelope.
		log.Printf("platform users: failed to create request: %v", err)
		writeUsersError(w, http.StatusInternalServerError)
		return
	}
	req.Header.Set("X-API-Key", h.apiKey)

	resp, err := h.client.Do(req)
	if err != nil {
		log.Printf("platform users: platform API call failed: %v", err)
		writeUsersError(w, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Never forward the upstream body or status verbatim to the
		// browser — platform error payloads are an internal detail (and a
		// leak channel). Log the truncated detail server-side; the client
		// sees a stable generic envelope.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		log.Printf("platform users: platform API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		writeUsersError(w, http.StatusBadGateway)
		return
	}

	// Forward the response directly
	w.WriteHeader(http.StatusOK)
	io.Copy(w, resp.Body)
}

// writeUsersError emits the sanitized JSON error envelope for the users
// endpoint. Content-Type is already set by the caller.
func writeUsersError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "platform API error"})
}
