// Package client is a thin HTTP client for the Status Harbor Console
// API. The provider holds one instance per configured provider block;
// resources call methods like CreateLighthouse / GetLighthouse on it.
//
// Authentication is a team:admin Bearer token; the server side enforces
// that tokens authenticate as the creator user on dashboard endpoints.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps an http.Client with the api base URL + bearer token.
type Client struct {
	baseURL   string
	token     string
	userAgent string
	httpc     *http.Client
}

// New constructs a Client. baseURL is the Console root (production:
// https://terraform.statusharbor.io). The provider hardcodes prod;
// staging/dev overrides happen via build-time ldflags.
func New(baseURL, token, providerVersion string) *Client {
	return &Client{
		baseURL:   baseURL,
		token:     token,
		userAgent: fmt.Sprintf("terraform-provider-statusharbor/%s", providerVersion),
		httpc:     &http.Client{Timeout: 30 * time.Second},
	}
}

// APIError carries the parsed { "error": "..." } body from a non-2xx
// response. Callers compare via errors.As to surface a useful message
// in Terraform diagnostics.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("status harbor api: %d %s", e.StatusCode, e.Message)
}

// IsNotFound returns true when err wraps a 404. Used by resource Read
// implementations to distinguish "deleted out-of-band" from real
// failures so Terraform can drop the resource from state.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// Lighthouse mirrors the server's LighthouseAPIResponse. Only the
// fields we need to reconcile state are listed; extra fields the
// server may add are ignored on decode (json.Decoder default).
type Lighthouse struct {
	ID                      string  `json:"id"`
	Name                    string  `json:"name"`
	Host                    *string `json:"host"`
	AgentHostname           *string `json:"agent_hostname"`
	AgentVersion            *string `json:"agent_version"`
	Paused                  bool    `json:"paused"`
	NotifyOnLifecycle       bool    `json:"notify_on_lifecycle"`
	FlapProtectionThreshold int     `json:"flap_protection_threshold"`
	LastHeartbeatAt         *string `json:"last_heartbeat_at"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

// LighthouseCreateResponse extends Lighthouse with the plaintext token
// that's returned only on creation.
type LighthouseCreateResponse struct {
	Lighthouse
	Token string `json:"token"`
}

// CreateLighthouseRequest is the body for POST /api/lighthouses.
type CreateLighthouseRequest struct {
	Name string  `json:"name"`
	Host *string `json:"host,omitempty"`
}

// UpdateLighthouseRequest is the body for PATCH /api/lighthouses/{id}.
// Pointers so we only send fields the user touched.
type UpdateLighthouseRequest struct {
	Host                    *string `json:"host,omitempty"`
	NotifyOnLifecycle       *bool   `json:"notify_on_lifecycle,omitempty"`
	FlapProtectionThreshold *int    `json:"flap_protection_threshold,omitempty"`
	Paused                  *bool   `json:"paused,omitempty"`
}

// CreateLighthouse mints a new Lighthouse and returns its metadata
// plus the one-time bearer token the agent will use.
func (c *Client) CreateLighthouse(ctx context.Context, req CreateLighthouseRequest) (*LighthouseCreateResponse, error) {
	var out LighthouseCreateResponse
	if err := c.do(ctx, http.MethodPost, "/api/lighthouses", req, &out, http.StatusCreated); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLighthouse fetches a Lighthouse by id. Returns IsNotFound on 404
// so Read can drop the resource from state cleanly.
func (c *Client) GetLighthouse(ctx context.Context, id string) (*Lighthouse, error) {
	var out Lighthouse
	if err := c.do(ctx, http.MethodGet, "/api/lighthouses/"+id, nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateLighthouse PATCHes the editable fields. Unset fields are
// preserved server-side.
func (c *Client) UpdateLighthouse(ctx context.Context, id string, req UpdateLighthouseRequest) (*Lighthouse, error) {
	var out Lighthouse
	if err := c.do(ctx, http.MethodPatch, "/api/lighthouses/"+id, req, &out, http.StatusOK); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteLighthouse permanently removes a Lighthouse. The agent
// authenticated against the bound token will exit on its next
// heartbeat (cascade-deletes the api_token row, agent gets 401 →
// ErrLighthouseGone).
func (c *Client) DeleteLighthouse(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/lighthouses/"+id, nil, nil, http.StatusNoContent)
}

// do is the shared transport core: marshal, set headers, send,
// branch on status. Returns *APIError on non-2xx so callers can
// inspect StatusCode.
func (c *Client) do(ctx context.Context, method, path string, body, out any, expectedStatus int) error {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != expectedStatus {
		raw, _ := io.ReadAll(resp.Body)
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		msg := errBody.Error
		if msg == "" {
			msg = string(raw)
		}
		return &APIError{StatusCode: resp.StatusCode, Message: msg}
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
