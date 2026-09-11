package lidarr_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marcatos/cuearr/internal/adapters/lidarr"
	"github.com/marcatos/cuearr/internal/domain"
)

func TestClientPing(t *testing.T) {
	const apiKey = "test-api-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/v1/system/status" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/v1/system/status")
		}
		if got := r.Header.Get("X-Api-Key"); got != apiKey {
			t.Errorf("X-Api-Key = %q, want configured API key", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := lidarr.Client{
		BaseURL:    server.URL + "/",
		APIKey:     apiKey,
		HTTPClient: server.Client(),
	}

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestClientRequestImportMapsPath(t *testing.T) {
	const apiKey = "test-api-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/api/v1/command" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/api/v1/command")
		}
		if got := r.Header.Get("X-Api-Key"); got != apiKey {
			t.Errorf("X-Api-Key = %q, want configured API key", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var body struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Name != "DownloadedAlbumsScan" {
			t.Errorf("name = %q, want DownloadedAlbumsScan", body.Name)
		}
		if body.Path != "/lidarr/music/album" {
			t.Errorf("path = %q, want %q", body.Path, "/lidarr/music/album")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	client := lidarr.Client{
		BaseURL:    server.URL + "/",
		APIKey:     apiKey,
		HTTPClient: server.Client(),
		PathMap: []domain.PathMapRule{
			{From: "/downloads", To: "/lidarr"},
		},
	}

	if err := client.RequestImport(context.Background(), "/downloads/music/album"); err != nil {
		t.Fatalf("RequestImport() error = %v", err)
	}
}

func TestClientReturnsSanitizedErrorForUnauthorizedResponse(t *testing.T) {
	const apiKey = "secret-api-key"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "authentication failed", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := lidarr.Client{
		BaseURL:    server.URL,
		APIKey:     apiKey,
		HTTPClient: server.Client(),
	}

	err := client.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() error = nil, want unauthorized response error")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("Ping() error = %q, want status and body snippet", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatalf("Ping() error exposed API key: %q", err)
	}
}
