package tools

import (
	"bytes"
	"context"
	"image/jpeg"
	"time"
)

type Go2RTCDiagnostics struct {
	Status           string               `json:"status"`
	Stage            string               `json:"stage"`
	ErrorCode        string               `json:"error_code,omitempty"`
	DurationMS       int64                `json:"duration_ms"`
	Runtime          Go2RTCStatus         `json:"runtime"`
	Stream           *Go2RTCStreamStatus  `json:"stream,omitempty"`
	EffectiveBackend string               `json:"effective_transcoding_backend"`
	Hardware         Go2RTCHardwareResult `json:"hardware"`
	Width            int                  `json:"width"`
	Height           int                  `json:"height"`
	Bytes            int                  `json:"bytes"`
	Tuya             *Go2RTCTuyaResult    `json:"tuya,omitempty"`
}

func (m *Go2RTCManager) Diagnose(ctx context.Context, id string) (out Go2RTCDiagnostics) {
	start := time.Now()
	budget := 15 * time.Second
	var tuyaSource string
	var tuyaAccount string
	for _, camera := range m.Config().Streams {
		if camera.ID == id && camera.Tuya != nil {
			budget = 45 * time.Second
			tuyaSource = camera.Source
			tuyaAccount = camera.Tuya.AccountID
		}
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	defer func() { out.DurationMS = time.Since(start).Milliseconds() }()
	out.Status = "error"
	out.Stage = "api"
	out.Hardware = m.HardwareStatus()
	out.Runtime = m.Status(ctx)
	if !out.Runtime.APIUsable {
		out.ErrorCode = "api_unavailable"
		return
	}
	out.Stage = "stream"
	stream, err := m.StreamStatus(ctx, id)
	if err != nil {
		out.ErrorCode = "stream_unavailable"
		return
	}
	out.Stream = &stream
	out.EffectiveBackend = "original"
	if stream.Media.CodecProfile != "" && stream.Media.CodecProfile != "original" {
		out.EffectiveBackend = out.Hardware.Backend
	}
	if tuyaSource != "" {
		probe := m.TuyaTest(ctx, tuyaSource, tuyaAccount)
		out.Tuya = &probe
		if !m.MediaRevisionCurrent(id, stream.Revision) {
			out.Tuya = nil
			out.ErrorCode = "camera_changed"
			out.Stage = "stream"
			return
		}
		if probe.Status != "ok" {
			out.ErrorCode = probe.ErrorCode
			out.Stage = "tuya_source"
			return
		}
	}
	out.Stage = "snapshot"
	_, data, err := m.SnapshotBytes(ctx, id, Go2RTCSnapshotOptions{Width: 1280})
	if err != nil {
		out.ErrorCode = "snapshot_unavailable"
		return
	}
	info, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if !m.MediaRevisionCurrent(id, stream.Revision) {
		out.ErrorCode = "camera_changed"
		out.Stage = "stream"
		return
	}
	if err != nil {
		out.ErrorCode = "invalid_camera_image"
		return
	}
	out.Width = info.Width
	out.Height = info.Height
	out.Bytes = len(data)
	out.Status = "ok"
	out.Stage = "complete"
	if out.EffectiveBackend != "original" && out.Hardware.Backend != "software" && !out.Hardware.Verified {
		out.Status = "error"
		out.Stage = "hardware"
		out.ErrorCode = "hardware_not_verified"
	}
	return
}
