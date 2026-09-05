package cyd

import (
	"math"
	"time"
)

const (
	TitleMax = 32
	BodyMax  = 96
	TaskMax  = 40
	ModelMax = 23
)

// Snapshot is the compact dashboard payload for Cheap Yellow Displays.
type Snapshot struct {
	TS      int64       `json:"ts"`
	Agent   AgentInfo   `json:"agent"`
	Host    HostMetrics `json:"host"`
	Work    WorkInfo    `json:"work"`
	Display DisplayInfo `json:"display"`
	Notify  *Notify     `json:"notify"`
}

type AgentInfo struct {
	Busy        bool   `json:"busy"`
	Model       string `json:"model"`
	Personality string `json:"personality"`
	Task        string `json:"task"`
}

type HostMetrics struct {
	CPUPct      float64 `json:"cpu_pct"`
	MemPct      float64 `json:"mem_pct"`
	DiskPct     float64 `json:"disk_pct"`
	UptimeS     uint64  `json:"uptime_s"`
	HostUptimeS uint64  `json:"host_uptime_s"`
}

type WorkInfo struct {
	MissionsRunning int     `json:"missions_running"`
	MissionsQueued  int     `json:"missions_queued"`
	NotesOpen       int     `json:"notes_open"`
	LastUserH       float64 `json:"last_user_h"`
}

type DisplayInfo struct {
	Page       string `json:"page"`
	Brightness int    `json:"brightness"`
	LED        string `json:"led"`
}

type Notify struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	Priority string `json:"priority"`
	TTLS     int    `json:"ttl_s"`
}

type Device struct {
	TokenID  string    `json:"token_id"`
	Name     string    `json:"name"`
	Firmware string    `json:"firmware"`
	Variant  string    `json:"variant"`
	RSSI     int       `json:"rssi"`
	LastSeen time.Time `json:"last_seen"`
	WS       bool      `json:"ws"`
	Width    int       `json:"width,omitempty"`
	Height   int       `json:"height,omitempty"`
}

type Heartbeat struct {
	Firmware string `json:"firmware"`
	Variant  string `json:"variant"`
	RSSI     int    `json:"rssi"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

type Inputs struct {
	Busy            bool
	Model           string
	Personality     string
	Task            string
	CPUPct          float64
	MemPct          float64
	DiskPct         float64
	UptimeS         uint64
	HostUptimeS     uint64
	MissionsRunning int
	MissionsQueued  int
	NotesOpen       int
	LastUserH       float64
	Page            string
	Brightness      int
	LED             string
}

func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func NotifyRank(priority string) int {
	switch priority {
	case "critical":
		return 3
	case "high":
		return 2
	case "low":
		return 0
	default:
		return 1
	}
}

func NotifyTTL(priority string, ttl int) int {
	if ttl > 300 {
		ttl = 300
	}
	if ttl > 0 {
		return ttl
	}
	if NotifyRank(priority) >= 3 {
		return 60
	}
	return 30
}

func BuildSnapshot(in Inputs, overlay *Notify) Snapshot {
	led := in.LED
	if led == "" {
		if in.Busy {
			led = "yellow"
		} else {
			led = "green"
		}
	}
	page := in.Page
	if page == "" {
		page = "status"
	}
	brightness := in.Brightness
	if brightness <= 0 {
		brightness = 180
	}
	snap := Snapshot{
		TS: time.Now().Unix(),
		Agent: AgentInfo{
			Busy:        in.Busy,
			Model:       Truncate(in.Model, ModelMax),
			Personality: Truncate(in.Personality, ModelMax),
			Task:        Truncate(in.Task, TaskMax),
		},
		Host: HostMetrics{
			CPUPct:      finite(in.CPUPct),
			MemPct:      finite(in.MemPct),
			DiskPct:     finite(in.DiskPct),
			UptimeS:     in.UptimeS,
			HostUptimeS: in.HostUptimeS,
		},
		Work: WorkInfo{
			MissionsRunning: in.MissionsRunning,
			MissionsQueued:  in.MissionsQueued,
			NotesOpen:       in.NotesOpen,
			LastUserH:       finite(in.LastUserH),
		},
		Display: DisplayInfo{
			Page:       page,
			Brightness: brightness,
			LED:        led,
		},
	}
	if overlay != nil && overlay.ID != "" {
		copyNotify := *overlay
		snap.Notify = &copyNotify
	}
	return snap
}
