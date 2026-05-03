package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aurago/internal/launchpad"
)

func testLaunchpadServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	db, err := launchpad.InitDB(filepath.Join(t.TempDir(), "launchpad_test.db"))
	if err != nil {
		t.Fatalf("launchpad.InitDB() error = %v", err)
	}
	return &Server{LaunchpadDB: db, Logger: slog.Default()}, db
}

func TestHandleListLaunchpadLinks(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()
	launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "App1", URL: "https://app1.com", Category: "Cat1"})
	launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "App2", URL: "https://app2.com", Category: "Cat2"})

	req := httptest.NewRequest(http.MethodGet, "/api/launchpad/links", nil)
	rr := httptest.NewRecorder()
	handleListLaunchpadLinks(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	var links []launchpad.LaunchpadLink
	if err := json.NewDecoder(rr.Body).Decode(&links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 2 {
		t.Errorf("Expected 2 links, got %d", len(links))
	}
}

func TestHandleCreateLaunchpadLink(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()

	payload := map[string]string{
		"title":    "New App",
		"url":      "https://newapp.com",
		"category": "Tools",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/launchpad/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleCreateLaunchpadLink(s)(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var link launchpad.LaunchpadLink
	if err := json.NewDecoder(rr.Body).Decode(&link); err != nil {
		t.Fatal(err)
	}
	if link.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if link.Title != "New App" {
		t.Errorf("Expected title 'New App', got '%s'", link.Title)
	}
}

func TestHandleCreateLaunchpadLinkValidation(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()

	payload := map[string]string{
		"title": "Bad",
		"url":   "ftp://bad.com",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/launchpad/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleCreateLaunchpadLink(s)(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
}

func TestHandleGetLaunchpadLink(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()
	id, _ := launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "GetMe", URL: "https://getme.com"})

	req := httptest.NewRequest(http.MethodGet, "/api/launchpad/links/"+id, nil)
	rr := httptest.NewRecorder()
	handleGetLaunchpadLink(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	var link launchpad.LaunchpadLink
	if err := json.NewDecoder(rr.Body).Decode(&link); err != nil {
		t.Fatal(err)
	}
	if link.Title != "GetMe" {
		t.Errorf("Expected title 'GetMe', got '%s'", link.Title)
	}
}

func TestHandleUpdateLaunchpadLink(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()
	id, _ := launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "Old", URL: "https://old.com"})

	payload := map[string]string{
		"title": "Updated",
		"url":   "https://updated.com",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/launchpad/links/"+id, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handleUpdateLaunchpadLink(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	link, _ := launchpad.GetByID(s.LaunchpadDB, id)
	if link.Title != "Updated" {
		t.Errorf("Expected title 'Updated', got '%s'", link.Title)
	}
}

func TestHandleDeleteLaunchpadLink(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()
	id, _ := launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "DeleteMe", URL: "https://delete.com"})

	req := httptest.NewRequest(http.MethodDelete, "/api/launchpad/links/"+id, nil)
	rr := httptest.NewRecorder()
	handleDeleteLaunchpadLink(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}

	_, err := launchpad.GetByID(s.LaunchpadDB, id)
	if err == nil {
		t.Error("Expected link to be deleted")
	}
}

func TestHandleListLaunchpadCategories(t *testing.T) {
	s, db := testLaunchpadServer(t)
	defer db.Close()
	launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "A", URL: "https://a.com", Category: "Alpha"})
	launchpad.Create(s.LaunchpadDB, launchpad.LaunchpadLink{Title: "B", URL: "https://b.com", Category: "Beta"})

	req := httptest.NewRequest(http.MethodGet, "/api/launchpad/categories", nil)
	rr := httptest.NewRecorder()
	handleListLaunchpadCategories(s)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rr.Code)
	}
	var cats []string
	if err := json.NewDecoder(rr.Body).Decode(&cats); err != nil {
		t.Fatal(err)
	}
	if len(cats) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(cats))
	}
}
