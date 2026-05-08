// fakeConsole stands up an httptest.Server that pretends to be the
// Status Harbor API for the duration of an acceptance test. It owns
// an in-memory map of lighthouses keyed by id and serves the four
// endpoints the provider drives: POST/GET/PATCH/DELETE
// /api/lighthouses[/{id}].
//
// Tests wire it in via setProviderBaseURL(srv.URL) which mutates the
// package-level apiBaseURL var that the provider's client reads on
// configure. Restore on test cleanup.

package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeLighthouse struct {
	ID                      string
	Name                    string
	Host                    *string
	Paused                  bool
	NotifyOnLifecycle       bool
	FlapProtectionThreshold int
	AgentHostname           *string
	AgentVersion            *string
	LastHeartbeatAt         *string
	CreatedAt               string
	UpdatedAt               string
}

type fakeConsole struct {
	mu         sync.Mutex
	lighthouse map[string]*fakeLighthouse
	server     *httptest.Server
}

func newFakeConsole(t *testing.T) *fakeConsole {
	t.Helper()
	fc := &fakeConsole{lighthouse: map[string]*fakeLighthouse{}}
	fc.server = httptest.NewServer(http.HandlerFunc(fc.route))
	t.Cleanup(fc.server.Close)
	return fc
}

func (fc *fakeConsole) URL() string { return fc.server.URL }

// route dispatches by method + path. Keep narrow — only the
// endpoints the provider actually calls are implemented.
func (fc *fakeConsole) route(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-token" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/lighthouses":
		fc.create(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/lighthouses/"):
		fc.get(w, r)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/lighthouses/"):
		fc.update(w, r)
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/lighthouses/"):
		fc.delete(w, r)
	default:
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}
}

func (fc *fakeConsole) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string  `json:"name"`
		Host *string `json:"host"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad body"}`, http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}
	fc.mu.Lock()
	for _, lh := range fc.lighthouse {
		if lh.Name == body.Name {
			fc.mu.Unlock()
			http.Error(w, `{"error":"name already exists"}`, http.StatusConflict)
			return
		}
	}
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	lh := &fakeLighthouse{
		ID:                      id,
		Name:                    body.Name,
		Host:                    body.Host,
		Paused:                  false,
		NotifyOnLifecycle:       true,
		FlapProtectionThreshold: 1,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	fc.lighthouse[id] = lh
	fc.mu.Unlock()

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                        id,
		"name":                      lh.Name,
		"host":                      lh.Host,
		"paused":                    lh.Paused,
		"notify_on_lifecycle":       lh.NotifyOnLifecycle,
		"flap_protection_threshold": lh.FlapProtectionThreshold,
		"agent_hostname":            lh.AgentHostname,
		"agent_version":             lh.AgentVersion,
		"last_heartbeat_at":         lh.LastHeartbeatAt,
		"created_at":                lh.CreatedAt,
		"updated_at":                lh.UpdatedAt,
		"token":                     "fake-bearer-" + id[:8],
	})
}

func (fc *fakeConsole) get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lighthouses/")
	fc.mu.Lock()
	lh, ok := fc.lighthouse[id]
	fc.mu.Unlock()
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                        lh.ID,
		"name":                      lh.Name,
		"host":                      lh.Host,
		"paused":                    lh.Paused,
		"notify_on_lifecycle":       lh.NotifyOnLifecycle,
		"flap_protection_threshold": lh.FlapProtectionThreshold,
		"agent_hostname":            lh.AgentHostname,
		"agent_version":             lh.AgentVersion,
		"last_heartbeat_at":         lh.LastHeartbeatAt,
		"created_at":                lh.CreatedAt,
		"updated_at":                lh.UpdatedAt,
	})
}

func (fc *fakeConsole) update(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lighthouses/")
	var body struct {
		Host                    *string `json:"host"`
		NotifyOnLifecycle       *bool   `json:"notify_on_lifecycle"`
		FlapProtectionThreshold *int    `json:"flap_protection_threshold"`
		Paused                  *bool   `json:"paused"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	fc.mu.Lock()
	defer fc.mu.Unlock()
	lh, ok := fc.lighthouse[id]
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if body.Host != nil {
		lh.Host = body.Host
	}
	if body.NotifyOnLifecycle != nil {
		lh.NotifyOnLifecycle = *body.NotifyOnLifecycle
	}
	if body.FlapProtectionThreshold != nil {
		lh.FlapProtectionThreshold = *body.FlapProtectionThreshold
	}
	if body.Paused != nil {
		lh.Paused = *body.Paused
	}
	lh.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                        lh.ID,
		"name":                      lh.Name,
		"host":                      lh.Host,
		"paused":                    lh.Paused,
		"notify_on_lifecycle":       lh.NotifyOnLifecycle,
		"flap_protection_threshold": lh.FlapProtectionThreshold,
		"created_at":                lh.CreatedAt,
		"updated_at":                lh.UpdatedAt,
	})
}

func (fc *fakeConsole) delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/lighthouses/")
	fc.mu.Lock()
	_, ok := fc.lighthouse[id]
	if ok {
		delete(fc.lighthouse, id)
	}
	fc.mu.Unlock()
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setProviderBaseURL swaps the package-level apiBaseURL and returns a
// cleanup func that restores the original. Tests use this to point
// the provider at a fakeConsole instead of production.
func setProviderBaseURL(t *testing.T, url string) {
	t.Helper()
	prev := apiBaseURL
	apiBaseURL = url
	t.Cleanup(func() { apiBaseURL = prev })
}

// providerConfigHCL is the HCL provider block that wires the fake
// API token through. The fakeConsole accepts only "test-token";
// any other value gets 401.
const providerConfigHCL = `
provider "statusharbor" {
  api_token = "test-token"
}
`
