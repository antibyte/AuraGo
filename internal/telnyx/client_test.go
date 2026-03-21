package telnyx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-key", nil)
	if c.apiKey != "test-key" {
		t.Errorf("expected apiKey 'test-key', got %q", c.apiKey)
	}
	if c.baseURL != defaultBaseURL {
		t.Errorf("expected baseURL %q, got %q", defaultBaseURL, c.baseURL)
	}
}

func TestClient_AuthHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-api-key" {
			t.Errorf("expected Authorization 'Bearer test-api-key', got %q", auth)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %q", ct)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]string{}})
	}))
	defer ts.Close()

	c := NewClient("test-api-key", nil)
	c.baseURL = ts.URL

	_, _, err := c.get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_RetryOn429(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"errors":[{"code":"rate_limit","title":"Rate limited"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"id":"ok"}}`))
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	data, status, err := c.get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}
	if data == nil {
		t.Error("expected non-nil data")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_ErrorParsing(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{
			Errors: []APIError{{Code: "40001", Title: "Validation Error", Detail: "phone number invalid"}},
		})
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	_, _, err := c.get(context.Background(), "/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestClient_GetBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/balance" {
			t.Errorf("expected path /balance, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(BalanceResponse{
			Data: struct {
				Balance         string `json:"balance"`
				Currency        string `json:"currency"`
				CreditLimit     string `json:"credit_limit"`
				AvailableCredit string `json:"available_credit"`
			}{
				Balance:  "25.50",
				Currency: "USD",
			},
		})
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	resp, err := c.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.Balance != "25.50" {
		t.Errorf("expected balance '25.50', got %q", resp.Data.Balance)
	}
	if resp.Data.Currency != "USD" {
		t.Errorf("expected currency 'USD', got %q", resp.Data.Currency)
	}
}
