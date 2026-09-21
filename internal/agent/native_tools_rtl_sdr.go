package agent

import openai "github.com/sashabaranov/go-openai"

func rtlSDRSchema() openai.Tool {
	tuning := schema(map[string]interface{}{
		"frequency_hz": prop("integer", "RF frequency in Hz; bounded by the detected tuner."),
		"mode":         map[string]interface{}{"type": "string", "enum": []string{"wfm", "nfm", "am", "usb", "lsb", "dab"}},
		"bandwidth_hz": prop("integer", "Demodulator bandwidth; omit for mode default."), "agc": prop("boolean", "Automatic gain; usually true."), "gain_db": prop("number", "One supported gain from status."), "ppm": prop("integer", "Frequency correction, -200 to 200."), "squelch_db": prop("number", "Squelch threshold in dB; -100 for broadcast radio."), "stereo": prop("boolean", "Enable stereo for WFM."), "dab_block": prop("string", "DAB Band III block returned by scan."), "service_id": prop("string", "Exact DAB service ID returned by stations; required for DAB recording."), "label": prop("string", "Optional station label."),
	}, "mode")
	return tool("rtl_sdr", "Receive and record radio on the server's configured RTL-SDR dongle. Start durable recordings or timezone-aware schedules and transcribe using AuraGo's configured ASR. Long work returns an ID; poll result. No LLM is needed when a schedule fires. Never invent a frequency or DAB service ID.", schema(map[string]interface{}{
		"operation": map[string]interface{}{"type": "string", "enum": []string{"status", "stations", "scan", "record", "schedule", "recordings", "schedules", "result", "stop_recording", "transcribe", "delete_schedule"}},
		"id":        prop("string", "Existing recording/schedule ID for item operations."), "name": prop("string", "Short user-visible title."), "tuning": tuning, "start_at": prop("string", "Schedule first start, RFC3339 with UTC offset."), "timezone": prop("string", "IANA timezone, e.g. Europe/Berlin; repeat follows local wall time."), "repeat": map[string]interface{}{"type": "string", "enum": []string{"once", "daily", "weekly"}}, "duration_seconds": prop("integer", "5..7200 seconds; default 600."), "transcribe": prop("boolean", "Transcribe after recording."), "offset": prop("integer", "Pagination offset from the previous response; default 0."),
	}, "operation"))
}
