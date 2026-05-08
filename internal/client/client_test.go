// Hermetic tests for the Status Harbor API client. Each test spins
// up an httptest.Server that pretends to be the Console and asserts
// the client speaks the right protocol shape: bearer auth, JSON
// bodies, status-code → error mapping, etc.
//
// No real Console required; runs under `go test ./...`.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubConsole returns an httptest.Server whose handler is the supplied
// http.Handler. The Client is wired against it.
func stubConsole(h http.Handler) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	c := New(srv.URL, "test-token", "0.0.0-test")
	return c, srv
}

func TestCreateLighthouse_OK(t *testing.T) {
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request shape.
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/lighthouses" {
			t.Errorf("path = %s, want /api/lighthouses", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		var body CreateLighthouseRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "prod" {
			t.Errorf("name = %q, want prod", body.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(LighthouseCreateResponse{
			Lighthouse: Lighthouse{
				ID:                      "lh-uuid",
				Name:                    "prod",
				Paused:                  false,
				NotifyOnLifecycle:       true,
				FlapProtectionThreshold: 1,
				CreatedAt:               "2026-05-08T00:00:00Z",
				UpdatedAt:               "2026-05-08T00:00:00Z",
			},
			Token: "lh-secret",
		})
	}))
	defer srv.Close()

	got, err := c.CreateLighthouse(context.Background(), CreateLighthouseRequest{Name: "prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "lh-uuid" || got.Token != "lh-secret" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestCreateLighthouse_BadStatus(t *testing.T) {
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error":"name already exists"}`)
	}))
	defer srv.Close()

	_, err := c.CreateLighthouse(context.Background(), CreateLighthouseRequest{Name: "dup"})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err not *APIError: %v", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Message != "name already exists" {
		t.Errorf("message = %q", apiErr.Message)
	}
}

func TestGetLighthouse_NotFound(t *testing.T) {
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := c.GetLighthouse(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(err) = false, want true; err = %v", err)
	}
}

func TestGetLighthouse_OK(t *testing.T) {
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/lighthouses/lh-uuid" {
			t.Errorf("path = %s", r.URL.Path)
		}
		host := "agent-host"
		_ = json.NewEncoder(w).Encode(Lighthouse{
			ID:   "lh-uuid",
			Name: "prod",
			Host: &host,
		})
	}))
	defer srv.Close()

	got, err := c.GetLighthouse(context.Background(), "lh-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Host == nil || *got.Host != "agent-host" {
		t.Errorf("host = %v, want agent-host", got.Host)
	}
}

func TestUpdateLighthouse_OmitsUnsetFields(t *testing.T) {
	var seenBody []byte
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		seenBody, _ = io.ReadAll(r.Body)
		_ = json.NewEncoder(w).Encode(Lighthouse{ID: "lh-uuid", Name: "prod"})
	}))
	defer srv.Close()

	paused := true
	if _, err := c.UpdateLighthouse(context.Background(), "lh-uuid", UpdateLighthouseRequest{
		Paused: &paused,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bodyStr := string(seenBody)
	if !strings.Contains(bodyStr, `"paused":true`) {
		t.Errorf("body should contain paused; got %s", bodyStr)
	}
	// Unset pointer fields should be omitted entirely.
	if strings.Contains(bodyStr, `"host"`) || strings.Contains(bodyStr, `"notify_on_lifecycle"`) {
		t.Errorf("body should not contain unset fields; got %s", bodyStr)
	}
}

func TestDeleteLighthouse_204(t *testing.T) {
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := c.DeleteLighthouse(context.Background(), "lh-uuid"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteLighthouse_404IsNotFound(t *testing.T) {
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	err := c.DeleteLighthouse(context.Background(), "lh-uuid")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(err) = false, want true; err = %v", err)
	}
}

func TestUserAgentStamped(t *testing.T) {
	var ua string
	c, srv := stubConsole(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	_ = c.DeleteLighthouse(context.Background(), "x")
	if !strings.HasPrefix(ua, "terraform-provider-statusharbor/") {
		t.Errorf("user-agent = %q, want prefix terraform-provider-statusharbor/", ua)
	}
}
