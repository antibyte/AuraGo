package tools

import "testing"

func TestGo2RTCTelemetrySeparatesMediaAndInternalConsumers(t *testing.T) {
	var out Go2RTCStreamStatus
	readGo2RTCTelemetry(map[string]interface{}{"producers": []interface{}{map[string]interface{}{
		"medias":    []interface{}{"video, recvonly, H264", "audio, sendonly, PCMA/8000"},
		"receivers": []interface{}{map[string]interface{}{"codec": map[string]interface{}{"codec_name": "H264"}, "bytes": float64(100), "packets": float64(7)}},
		"senders":   []interface{}{map[string]interface{}{"codec": map[string]interface{}{"codec_name": "PCMA"}, "bytes": float64(40), "packets": float64(2)}},
	}}}, &out)
	if out.Video.ReceivedPackets != 7 || out.Audio.SentBytes != 40 || len(out.VideoCodecs) == 0 || len(out.AudioCodecs) == 0 {
		t.Fatalf("telemetry: %+v", out)
	}
	consumers := []interface{}{map[string]interface{}{"format_name": "preload"}, map[string]interface{}{"format_name": "probe"}, map[string]interface{}{"user_agent": "go2rtc/ffmpeg"}, map[string]interface{}{"user_agent": "AuraGo Camera Snapshot"}, map[string]interface{}{"user_agent": "AuraGo Camera Viewer"}}
	if got := go2RTCViewerCount(consumers); got != 1 {
		t.Fatalf("viewer count: %d", got)
	}
}
