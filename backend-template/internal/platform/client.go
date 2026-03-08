package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// PlatformUser represents a tenant user from the Moses Platform API.
type PlatformUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

// PlatformTenant represents tenant info from the Moses Platform API.
type PlatformTenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MosesPlatformClient is a simple HTTP client for the Moses Platform API.
type MosesPlatformClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewFromEnv creates a client using MOSES_PLATFORM_API_KEY and MOSES_PLATFORM_API_URL env vars.
// Returns nil if the environment variables are not set.
func NewFromEnv() *MosesPlatformClient {
	apiKey := os.Getenv("MOSES_PLATFORM_API_KEY")
	apiURL := os.Getenv("MOSES_PLATFORM_API_URL")
	if apiKey == "" || apiURL == "" {
		return nil
	}
	return &MosesPlatformClient{
		baseURL: apiURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetTenant returns the tenant info for the calling app's tenant.
func (c *MosesPlatformClient) GetTenant(ctx context.Context) (*PlatformTenant, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/platform/tenant", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("platform API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platform API returned %d", resp.StatusCode)
	}

	var tenant PlatformTenant
	if err := json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &tenant, nil
}

// ListUsers returns all users in the calling app's tenant.
func (c *MosesPlatformClient) ListUsers(ctx context.Context) ([]PlatformUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/platform/users", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("platform API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("platform API returned %d", resp.StatusCode)
	}

	var result struct {
		Users []PlatformUser `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Users, nil
}
