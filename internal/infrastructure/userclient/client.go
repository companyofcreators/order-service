package userclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Profile struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
}
type FullResponse struct {
	Profile *Profile `json:"profile,omitempty"`
}
type Client struct {
	baseURL    string
	httpClient *http.Client
	hmacKey    []byte
	cache      map[string]*Profile
	mu         sync.RWMutex
}
func New(baseURL, hmacKey string) *Client {
	return &Client{baseURL: baseURL, httpClient: &http.Client{Timeout: 3 * time.Second}, hmacKey: []byte(hmacKey), cache: map[string]*Profile{}}
}
func (c *Client) sign(r *http.Request) {
	if len(c.hmacKey) == 0 { return }
	uid, email, role := r.Header.Get("X-User-Id"), r.Header.Get("X-User-Email"), r.Header.Get("X-User-Role")
	mac := hmac.New(sha256.New, c.hmacKey)
	mac.Write([]byte(uid + "|" + email + "|" + role))
	r.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}
func (c *Client) GetProfile(ctx context.Context, userID string) *Profile {
	c.mu.RLock()
	if p, ok := c.cache[userID]; ok { c.mu.RUnlock(); return p }
	c.mu.RUnlock()
	url := fmt.Sprintf("%s/internal/users/%s", c.baseURL, userID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("X-User-Id", "00000000-0000-0000-0000-000000000000")
	req.Header.Set("X-User-Email", "system@diploma")
	req.Header.Set("X-User-Role", "admin")
	c.sign(req)
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return nil }
	var full FullResponse
	json.NewDecoder(resp.Body).Decode(&full)
	if full.Profile != nil {
		c.mu.Lock()
		c.cache[userID] = full.Profile
		c.mu.Unlock()
	}
	return full.Profile
}
