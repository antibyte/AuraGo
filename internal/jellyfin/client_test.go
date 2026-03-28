package jellyfin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"aurago/internal/config"
)

func testConfig(serverURL string) config.JellyfinConfig {
	u, _ := url.Parse(serverURL)
	port, _ := strconv.Atoi(u.Port())
	return config.JellyfinConfig{
		Enabled: true,
		Host:    u.Hostname(),
		Port:    port,
		APIKey:  "test-api-key",
	}
}

func TestNewClient(t *testing.T) {
	cfg := config.JellyfinConfig{
		Enabled:        true,
		Host:           "localhost",
		Port:           8096,
		APIKey:         "test-key",
		ConnectTimeout: 5,
		RequestTimeout: 10,
	}

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	expected := "http://localhost:8096"
	if client.BaseURL() != expected {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL(), expected)
	}
}

func TestNewClientHTTPS(t *testing.T) {
	cfg := config.JellyfinConfig{
		Enabled:  true,
		Host:     "media.example.com",
		Port:     8920,
		UseHTTPS: true,
		APIKey:   "test-key",
	}

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	expected := "https://media.example.com:8920"
	if client.BaseURL() != expected {
		t.Errorf("BaseURL = %q, want %q", client.BaseURL(), expected)
	}
}

func TestNewClientMissingHost(t *testing.T) {
	cfg := config.JellyfinConfig{
		Enabled: true,
		APIKey:  "test-key",
	}

	_, err := NewClient(cfg, nil)
	if err == nil {
		t.Fatal("Expected error for missing host, got nil")
	}
}

func TestNewClientMissingAPIKey(t *testing.T) {
	cfg := config.JellyfinConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    8096,
	}

	_, err := NewClient(cfg, nil)
	if err == nil {
		t.Fatal("Expected error for missing API key, got nil")
	}
}

func TestPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Ping" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Emby-Token") != "test-api-key" {
			t.Error("missing or wrong auth header")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`"Jellyfin Server"`))
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.APIKey = "test-api-key"
	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestGetSystemInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(SystemInfo{
			ServerName:      "TestServer",
			Version:         "10.9.0",
			OperatingSystem: "Linux",
			ID:              "abc123",
		})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.APIKey = "test-key"
	client, _ := NewClient(cfg, nil)
	defer client.Close()

	info, err := client.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo: %v", err)
	}
	if info.ServerName != "TestServer" {
		t.Errorf("ServerName = %q, want TestServer", info.ServerName)
	}
	if info.Version != "10.9.0" {
		t.Errorf("Version = %q, want 10.9.0", info.Version)
	}
}

func TestGetLibraries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Library/VirtualFolders" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode([]Library{
			{Name: "Movies", ItemID: "1", CollectionType: "movies", Locations: []string{"/media/movies"}},
			{Name: "Music", ItemID: "2", CollectionType: "music", Locations: []string{"/media/music"}},
		})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.APIKey = "test-key"
	client, _ := NewClient(cfg, nil)
	defer client.Close()

	libs, err := client.GetLibraries(context.Background())
	if err != nil {
		t.Fatalf("GetLibraries: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("expected 2 libraries, got %d", len(libs))
	}
	if libs[0].Name != "Movies" {
		t.Errorf("first library name = %q, want Movies", libs[0].Name)
	}
}

func TestSearchItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Items" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("SearchTerm") != "matrix" {
			t.Errorf("SearchTerm = %q, want matrix", q.Get("SearchTerm"))
		}
		json.NewEncoder(w).Encode(ItemsResponse{
			Items: []MediaItem{
				{Name: "The Matrix", ID: "m1", Type: "Movie", ProductionYear: 1999},
			},
			TotalRecordCount: 1,
		})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.APIKey = "test-key"
	client, _ := NewClient(cfg, nil)
	defer client.Close()

	resp, err := client.SearchItems(context.Background(), "matrix", "Movie", 20)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if resp.TotalRecordCount != 1 {
		t.Errorf("TotalRecordCount = %d, want 1", resp.TotalRecordCount)
	}
	if resp.Items[0].Name != "The Matrix" {
		t.Errorf("first item name = %q, want The Matrix", resp.Items[0].Name)
	}
}

func TestGetSessions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Sessions" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode([]Session{
			{ID: "s1", UserName: "admin", Client: "Jellyfin Web", DeviceName: "Chrome"},
		})
	}))
	defer srv.Close()

	cfg := testConfig(srv.URL)
	cfg.APIKey = "test-key"
	client, _ := NewClient(cfg, nil)
	defer client.Close()

	sessions, err := client.GetSessions(context.Background())
	if err != nil {
		t.Fatalf("GetSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].UserName != "admin" {
		t.Errorf("session user = %q, want admin", sessions[0].UserName)
	}
}

func TestDefaultPort(t *testing.T) {
	cfg := config.JellyfinConfig{
		Enabled: true,
		Host:    "localhost",
		Port:    0,
		APIKey:  "test-key",
	}

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	if client.BaseURL() != "http://localhost:8096" {
		t.Errorf("BaseURL = %q, want http://localhost:8096", client.BaseURL())
	}
}
