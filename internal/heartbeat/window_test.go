package heartbeat

import (
	"testing"
	"time"

	"aurago/internal/config"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"00:00", 0, 0, false},
		{"08:30", 8, 30, false},
		{"23:59", 23, 59, false},
		{"24:00", 0, 0, true},
		{"12:60", 0, 0, true},
		{"abc", 0, 0, true},
		{"8:30", 8, 30, false},
		{"08-30", 0, 0, true},
		{"", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			h, m, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr {
				if h != tt.wantH || m != tt.wantM {
					t.Fatalf("parseTime(%q) = %d:%d, want %d:%d", tt.input, h, m, tt.wantH, tt.wantM)
				}
			}
		})
	}
}

func TestTimeInMinutes(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"00:00", 0},
		{"01:00", 60},
		{"08:30", 510},
		{"23:59", 1439},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := timeInMinutes(tt.input)
			if got != tt.want {
				t.Errorf("timeInMinutes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsInWindow(t *testing.T) {
	tests := []struct {
		name  string
		now   time.Time
		start string
		end   string
		want  bool
	}{
		{"midday in day window", time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local), "08:00", "22:00", true},
		{"before day window", time.Date(2024, 1, 1, 7, 0, 0, 0, time.Local), "08:00", "22:00", false},
		{"after day window", time.Date(2024, 1, 1, 23, 0, 0, 0, time.Local), "08:00", "22:00", false},
		{"exactly at start", time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local), "08:00", "22:00", true},
		{"exactly at end", time.Date(2024, 1, 1, 22, 0, 0, 0, time.Local), "08:00", "22:00", false},
		{"overnight window - night", time.Date(2024, 1, 1, 23, 0, 0, 0, time.Local), "22:00", "08:00", true},
		{"overnight window - early morning", time.Date(2024, 1, 1, 3, 0, 0, 0, time.Local), "22:00", "08:00", true},
		{"overnight window - day", time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local), "22:00", "08:00", false},
		{"empty window", time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local), "08:00", "08:00", false},
		{"full day window", time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local), "00:00", "23:59", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInWindow(tt.now, tt.start, tt.end)
			if got != tt.want {
				t.Errorf("isInWindow(%v, %q, %q) = %v, want %v", tt.now, tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestGetActiveWindow(t *testing.T) {
	hb := config.HeartbeatConfig{
		DayTimeWindow: config.HeartbeatTimeWindow{
			Start: "08:00", End: "20:00", Interval: "1h",
		},
		NightTimeWindow: config.HeartbeatTimeWindow{
			Start: "22:00", End: "06:00", Interval: "4h",
		},
	}

	tests := []struct {
		name         string
		now          time.Time
		wantStart    string
		wantEnd      string
		wantInterval string
	}{
		{"day", time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local), "08:00", "20:00", "1h"},
		{"night", time.Date(2024, 1, 1, 23, 0, 0, 0, time.Local), "22:00", "06:00", "4h"},
		{"early morning", time.Date(2024, 1, 1, 3, 0, 0, 0, time.Local), "22:00", "06:00", "4h"},
		{"boundary day", time.Date(2024, 1, 1, 8, 0, 0, 0, time.Local), "08:00", "20:00", "1h"},
		{"boundary night", time.Date(2024, 1, 1, 22, 0, 0, 0, time.Local), "22:00", "06:00", "4h"},
		{"gap between windows", time.Date(2024, 1, 1, 21, 0, 0, 0, time.Local), "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			window, interval := getActiveWindow(tt.now, hb)
			var gotStart, gotEnd string
			if window != nil {
				gotStart = window.Start
				gotEnd = window.End
			}
			if gotStart != tt.wantStart || gotEnd != tt.wantEnd {
				t.Errorf("getActiveWindow window = {%s %s}, want {%s %s}", gotStart, gotEnd, tt.wantStart, tt.wantEnd)
			}
			if interval != tt.wantInterval {
				t.Errorf("getActiveWindow interval = %q, want %q", interval, tt.wantInterval)
			}
		})
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"15m", 15 * time.Minute},
		{"30m", 30 * time.Minute},
		{"1h", 1 * time.Hour},
		{"2h", 2 * time.Hour},
		{"4h", 4 * time.Hour},
		{"6h", 6 * time.Hour},
		{"12h", 12 * time.Hour},
		{"", 1 * time.Hour},    // default
		{"xyz", 1 * time.Hour}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInterval(tt.input)
			if got != tt.want {
				t.Errorf("parseInterval(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
