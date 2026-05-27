package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/companyofcreators/order-service/pkg/header_auth"
)

// UserClient communicates with the User Service to check roles.
type UserClient struct {
	baseURL    string
	httpClient *http.Client
	signer     *header_auth.HeaderSigner
	log        *slog.Logger
}

// NewUserClient creates a new UserClient.
func NewUserClient(baseURL string, signer *header_auth.HeaderSigner, log *slog.Logger) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		signer: signer,
		log:    log,
	}
}

type userRolesResponse struct {
	Roles []string `json:"roles"`
}

// HasRole checks if the given user has the specified role by calling user-service.
func (c *UserClient) HasRole(ctx context.Context, userID, role string) (bool, error) {
	url := fmt.Sprintf("%s/internal/users/%s/roles", c.baseURL, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-Id", userID)
	c.signer.SignHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.ErrorContext(ctx, "failed to call user service for roles",
			"url", url,
			"error", err.Error(),
		)
		return false, fmt.Errorf("failed to check user role: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("user service returned status %d", resp.StatusCode)
	}

	var rolesResp userRolesResponse
	if err := json.NewDecoder(resp.Body).Decode(&rolesResp); err != nil {
		return false, fmt.Errorf("failed to decode user service response: %w", err)
	}

	for _, r := range rolesResp.Roles {
		if r == role {
			return true, nil
		}
	}

	return false, nil
}
