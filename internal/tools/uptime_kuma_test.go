package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const sampleUptimeKumaMetrics = `# HELP monitor_status Current monitor status
# TYPE monitor_status gauge
monitor_status{monitor_name="API",monitor_type="http",monitor_url="https://api.example.com"} 1
monitor_response_time{monitor_name="API",monitor_type="http",monitor_url="https://api.example.com"} 123
monitor_status{monitor_name="Database",monitor_type="tcp",monitor_hostname="db.internal",monitor_port="5432"} 0
`

func TestParseUptimeKumaMetricsMapsSnapshots(t *testing.T) {
	snapshot, err := parseUptimeKumaMetrics([]byte(sampleUptimeKumaMetrics), nil)
	if err != nil {
		t.Fatalf("parseUptimeKumaMetrics() error = %v", err)
	}

	if snapshot.Summary.Total != 2 || snapshot.Summary.Up != 1 || snapshot.Summary.Down != 1 || snapshot.Summary.Unknown != 0 {
		t.Fatalf("unexpected summary: %+v", snapshot.Summary)
	}
	if len(snapshot.Monitors) != 2 {
		t.Fatalf("len(snapshot.Monitors) = %d, want 2", len(snapshot.Monitors))
	}

	api := snapshot.Monitors[0]
	if api.MonitorName != "API" || api.Status != "up" || api.ResponseTimeMS != 123 {
		t.Fatalf("unexpected API monitor: %+v", api)
	}

	db := snapshot.Monitors[1]
	if db.MonitorName != "Database" || db.Status != "down" || db.MonitorHostname != "db.internal" || db.MonitorPort != "5432" {
		t.Fatalf("unexpected Database monitor: %+v", db)
	}
}

func TestFetchUptimeKumaSnapshotUsesAPIKeyBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Fatal("expected basic auth header")
		}
		if user != "" {
			t.Fatalf("basic auth username = %q, want empty string", user)
		}
		if pass != "uk2_demo_secret" {
			t.Fatalf("basic auth password = %q, want API key", pass)
		}
		fmt.Fprint(w, sampleUptimeKumaMetrics)
	}))
	defer srv.Close()

	snapshot, err := FetchUptimeKumaSnapshot(context.Background(), UptimeKumaConfig{
		BaseURL:        srv.URL,
		APIKey:         "uk2_demo_secret",
		RequestTimeout: 5,
	}, nil)
	if err != nil {
		t.Fatalf("FetchUptimeKumaSnapshot() error = %v", err)
	}

	if snapshot.Summary.Down != 1 {
		t.Fatalf("snapshot.Summary.Down = %d, want 1", snapshot.Summary.Down)
	}
}

func TestFetchUptimeKumaSnapshotInsecureTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sampleUptimeKumaMetrics)
	}))
	defer srv.Close()

	_, err := FetchUptimeKumaSnapshot(context.Background(), UptimeKumaConfig{
		BaseURL:        srv.URL,
		APIKey:         "uk2_demo_secret",
		RequestTimeout: 5,
	}, nil)
	if err == nil {
		t.Fatal("expected TLS verification error without insecure_ssl")
	}

	snapshot, err := FetchUptimeKumaSnapshot(context.Background(), UptimeKumaConfig{
		BaseURL:        srv.URL,
		APIKey:         "uk2_demo_secret",
		InsecureSSL:    true,
		RequestTimeout: 5,
	}, nil)
	if err != nil {
		t.Fatalf("FetchUptimeKumaSnapshot() with insecure_ssl error = %v", err)
	}
	if snapshot.Summary.Total != 2 {
		t.Fatalf("snapshot.Summary.Total = %d, want 2", snapshot.Summary.Total)
	}
}

func TestUptimeKumaPollerReportsTransitionsOnlyAfterBaseline(t *testing.T) {
	var calls atomic.Int32
	events := make(chan UptimeKumaTransition, 4)

	makeSnapshot := func(status string) UptimeKumaSnapshot {
		return UptimeKumaSnapshot{
			Monitors: []UptimeKumaMonitorSnapshot{
				{
					MonitorName: "API",
					Status:      status,
					Labels: map[string]string{
						"monitor_name": "API",
						"monitor_type": "http",
						"monitor_url":  "https://api.example.com",
					},
				},
			},
		}
	}

	poller := NewUptimeKumaPoller(UptimeKumaPollerConfig{
		Interval: 10 * time.Millisecond,
		Fetch: func(ctx context.Context) (UptimeKumaSnapshot, error) {
			switch calls.Add(1) {
			case 1:
				return makeSnapshot("up"), nil
			case 2:
				return makeSnapshot("down"), nil
			case 3:
				return makeSnapshot("down"), nil
			default:
				return makeSnapshot("up"), nil
			}
		},
		OnTransition: func(event UptimeKumaTransition) {
			events <- event
		},
	})
	poller.Start()
	defer poller.Stop()

	var got []UptimeKumaTransition
	timeout := time.After(500 * time.Millisecond)
	for len(got) < 2 {
		select {
		case event := <-events:
			got = append(got, event)
		case <-timeout:
			t.Fatalf("timed out waiting for poller transitions, got %d", len(got))
		}
	}

	if got[0].Event != "DOWN" || got[0].PreviousStatus != "up" || got[0].CurrentStatus != "down" {
		t.Fatalf("unexpected first transition: %+v", got[0])
	}
	if got[1].Event != "UP" || got[1].PreviousStatus != "down" || got[1].CurrentStatus != "up" {
		t.Fatalf("unexpected second transition: %+v", got[1])
	}
}
