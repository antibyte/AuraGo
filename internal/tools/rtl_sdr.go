package tools

import (
	"aurago/internal/config"
	"aurago/internal/rtlsdr"
	"aurago/internal/security"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

var rtlSDRMu sync.RWMutex
var rtlSDRService *rtlsdr.Service

func SetRTLSDRService(s *rtlsdr.Service) { rtlSDRMu.Lock(); defer rtlSDRMu.Unlock(); rtlSDRService = s }

type RTLSDRRequest struct {
	Operation  string        `json:"operation"`
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Tuning     rtlsdr.Tuning `json:"tuning"`
	Start      time.Time     `json:"start_at"`
	Timezone   string        `json:"timezone"`
	Repeat     string        `json:"repeat"`
	Duration   int           `json:"duration_seconds"`
	Transcribe bool          `json:"transcribe"`
	Offset     int           `json:"offset"`
}

func (r *RTLSDRRequest) UnmarshalJSON(data []byte) error {
	type request RTLSDRRequest
	value := request{Tuning: rtlsdr.DefaultTuning()}
	// Each demodulator supplies its bandwidth when the caller omits it.
	value.Tuning.Bandwidth = 0
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*r = RTLSDRRequest(value)
	return nil
}
func ExecuteRTLSDR(ctx context.Context, cfg *config.Config, r RTLSDRRequest) string {
	fail := func(code string) string {
		b, _ := json.Marshal(map[string]string{"status": "error", "error": code})
		return "Tool Output: " + string(b)
	}
	if cfg == nil || !cfg.RTLSDR.Enabled || !cfg.RTLSDR.AllowAgent || !cfg.VirtualDesktop.Enabled {
		return fail("sdr_disabled")
	}
	rtlSDRMu.RLock()
	svc := rtlSDRService
	rtlSDRMu.RUnlock()
	if svc == nil {
		return fail("sdr_needs_setup")
	}
	operation := strings.ToLower(strings.TrimSpace(r.Operation))
	var data any
	var err error
	read := operation == "status" || operation == "stations" || operation == "recordings" || operation == "schedules" || operation == "result"
	if !read && (cfg.RTLSDR.ReadOnly || cfg.VirtualDesktop.ReadOnly || cfg.Docker.ReadOnly || !cfg.Docker.Enabled) {
		return fail("sdr_read_only")
	}
	switch operation {
	case "status":
		st := svc.Snapshot()
		receiver, e := svc.Receiver(ctx)
		receiver.Spectrum = nil // FFT bins belong to the interactive desktop only.
		code := ""
		if e != nil {
			code = "sdr_needs_setup"
		}
		data = map[string]any{"active_job": st.Active, "listening": st.Listening, "scanning": st.Scanning, "used_bytes": st.UsedBytes, "quota_bytes": st.QuotaBytes, "read_only": st.ReadOnly, "receiver": receiver, "readiness": code}
	case "stations":
		st := svc.Snapshot()
		rows := append(st.Favorites, st.Stations...)
		if r.Offset < 0 || r.Offset > len(rows) {
			return fail("sdr_invalid_request")
		}
		end := min(r.Offset+20, len(rows))
		data = map[string]any{"stations": rows[r.Offset:end], "total": len(rows), "next_offset": end}
	case "recordings":
		rows := svc.Snapshot().Recordings
		if r.Offset < 0 || r.Offset > len(rows) {
			return fail("sdr_invalid_request")
		}
		end := min(r.Offset+20, len(rows))
		page := rows[r.Offset:end]
		for i := range page {
			page[i].Segments = nil
		}
		data = map[string]any{"recordings": page, "total": len(rows), "next_offset": end}
	case "schedules":
		data = svc.Snapshot().Schedules
	case "result":
		var row rtlsdr.Recording
		row, err = svc.Recording(r.ID)
		if err == nil {
			if r.Offset < 0 || r.Offset > len(row.Segments) {
				return fail("sdr_invalid_request")
			}
			total := len(row.Segments)
			end := min(r.Offset+5, total)
			row.Segments = row.Segments[r.Offset:end]
			data = map[string]any{"recording": row, "segment_count": total, "next_offset": end, "audio_url": "/api/desktop/rtl-sdr/recordings/" + r.ID + "/audio"}
		}
	case "record":
		data, err = svc.Record(rtlsdr.Recording{Name: r.Name, Tuning: r.Tuning, Duration: r.Duration, Transcribe: r.Transcribe, Agent: true})
	case "schedule":
		data, err = svc.SaveSchedule(rtlsdr.Schedule{Name: r.Name, Tuning: r.Tuning, Start: r.Start, Timezone: r.Timezone, Repeat: r.Repeat, Duration: r.Duration, Transcribe: r.Transcribe, Enabled: true, Agent: true})
	case "stop_recording":
		err = svc.StopRecording(r.ID)
	case "transcribe":
		err = svc.RetryTranscriptionAs(ctx, r.ID, true)
	case "delete_schedule":
		err = svc.DeleteSchedule(r.ID)
	case "scan":
		err = svc.ScanAs(ctx, true)
		data = map[string]string{"job_id": "scan"}
	default:
		return fail("sdr_invalid_operation")
	}
	if err != nil {
		code := "sdr_operation_failed"
		if strings.HasPrefix(err.Error(), "sdr_") && !strings.ContainsAny(err.Error(), " :/\\\n") {
			code = err.Error()
		}
		return fail(code)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return fail("sdr_result_failed")
	}
	// Status is a trusted local envelope; station metadata and transcripts are not.
	result, _ := json.Marshal(map[string]any{"status": "success", "data": security.IsolateExternalData(string(raw))})
	return "Tool Output: " + string(result)
}
