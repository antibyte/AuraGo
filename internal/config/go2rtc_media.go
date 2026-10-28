package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const Go2RTCHardwareImage = "alexxit/go2rtc:1.9.14-hardware@sha256:7fce3ffefe663753510ff4671d8b7496dead49b46c87392898ddef152c25687d"

// Go2RTCSourceKey preserves the original main-source key; optional roles get
// separate keys and never share secrets with another camera or role.
func Go2RTCSourceKey(id, role string) string {
	key := Go2RTCStreamSourceVaultKey(id)
	if key == "" {
		return ""
	}
	switch role {
	case "main", "":
		return key
	case "sub":
		return key + "_sub"
	case "snapshot":
		return key + "_snapshot"
	}
	return ""
}

func Go2RTCRoleSource(s Go2RTCStreamConfig, role string) string {
	switch role {
	case "sub":
		return s.SubSource
	case "snapshot":
		return s.SnapshotSource
	default:
		return s.Source
	}
}

func Go2RTCSetRoleSource(s *Go2RTCStreamConfig, role, value string) {
	switch role {
	case "main":
		s.Source = value
		s.SourceConfigured = strings.TrimSpace(value) != ""
	case "sub":
		s.SubSource = value
		s.SubSourceConfigured = strings.TrimSpace(value) != ""
	case "snapshot":
		s.SnapshotSource = value
		s.SnapshotSourceConfigured = strings.TrimSpace(value) != ""
	}
}

func ValidateGo2RTCMedia(c Go2RTCConfig) error {
	if err := ValidateGo2RTCTuya(c); err != nil {
		return err
	}
	h := c.Hardware
	if !slices.Contains([]string{"", "software", "vaapi", "cuda"}, h.Backend) {
		return fmt.Errorf("go2rtc.hardware.backend must be software, vaapi or cuda")
	}
	if h.Backend == "vaapi" && (!regexp.MustCompile(`^/dev/dri/renderD[0-9]+$`).MatchString(h.RenderNode) || !regexp.MustCompile(`^[1-9][0-9]{0,9}$`).MatchString(h.RenderGroup)) {
		return fmt.Errorf("VAAPI needs a render node and its numeric non-root group")
	}
	if h.Backend == "cuda" && !regexp.MustCompile(`^(?:[0-9]+|GPU-[a-fA-F0-9-]+)$`).MatchString(h.DeviceID) {
		return fmt.Errorf("CUDA needs one GPU index or UUID")
	}
	for _, s := range c.Streams {
		for _, field := range []struct {
			name, value string
			allowed     []string
		}{
			{"preview_source", s.PreviewSource, []string{"", "auto", "main", "sub"}},
			{"detection_source", s.DetectionSource, []string{"", "auto", "main", "sub"}},
			{"snapshot_mode", s.SnapshotMode, []string{"", "auto", "camera", "stream"}},
			{"codec_profile", s.CodecProfile, []string{"", "original", "h264_opus", "h264_aac"}},
			{"transport", s.Transport, []string{"", "inherit", "tcp", "udp"}},
			{"playback_mode", s.PlaybackMode, []string{"", "auto", "webrtc_tcp", "mse", "hls", "mp4", "mjpeg"}},
		} {
			if !slices.Contains(field.allowed, field.value) {
				return fmt.Errorf("go2rtc stream %q has invalid %s", s.ID, field.name)
			}
		}
		if s.PlaybackMode == "webrtc_tcp" && !c.WebRTC.Enabled {
			return fmt.Errorf("WebRTC/TCP playback requires LAN WebRTC")
		}
	}
	return nil
}
