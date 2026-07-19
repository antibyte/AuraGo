package bluetooth

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type commandRunner interface {
	LookPath(string) (string, error)
	Output(context.Context, string, ...string) ([]byte, error)
	Start(context.Context, string, string, string) (runningProcess, error)
}

type runningProcess interface {
	Wait() error
	Stop() error
}

type execCommandRunner struct{}

func (execCommandRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execCommandRunner) Start(ctx context.Context, backend, target, source string) (runningProcess, error) {
	return startPlaybackProcess(ctx, backend, target, source)
}

func probeAudioBackend(ctx context.Context, runner commandRunner, requested string) AudioStatus {
	if runner == nil {
		return AudioStatus{Reason: "Bluetooth audio command runner is unavailable."}
	}
	requested = strings.ToLower(strings.TrimSpace(requested))
	if requested == "" {
		requested = "auto"
	}
	if _, err := runner.LookPath("ffmpeg"); err != nil {
		return AudioStatus{Reason: "FFmpeg is required for Bluetooth music and TTS playback."}
	}

	probePipeWire := func() AudioStatus {
		if _, err := runner.LookPath("pw-dump"); err != nil {
			return AudioStatus{Backend: "pipewire", Reason: "pw-dump is not installed."}
		}
		if _, err := runner.LookPath("pw-play"); err != nil {
			return AudioStatus{Backend: "pipewire", Reason: "pw-play is not installed."}
		}
		if _, err := runner.Output(ctx, "pw-dump"); err != nil {
			return AudioStatus{Backend: "pipewire", Reason: "PipeWire is installed but its user session is not reachable."}
		}
		return AudioStatus{Usable: true, Backend: "pipewire"}
	}
	probePulse := func() AudioStatus {
		if _, err := runner.LookPath("pactl"); err != nil {
			return AudioStatus{Backend: "pulse", Reason: "pactl is not installed."}
		}
		muxers, err := runner.Output(ctx, "ffmpeg", "-hide_banner", "-muxers")
		if err != nil || !hasFFmpegPulseMuxer(string(muxers)) {
			return AudioStatus{Backend: "pulse", Reason: "FFmpeg does not provide the PulseAudio output muxer."}
		}
		if _, err := runner.Output(ctx, "pactl", "info"); err != nil {
			return AudioStatus{Backend: "pulse", Reason: "PulseAudio is installed but its user session is not reachable."}
		}
		return AudioStatus{Usable: true, Backend: "pulse"}
	}

	switch requested {
	case "pipewire":
		return probePipeWire()
	case "pulse":
		return probePulse()
	default:
		if status := probePipeWire(); status.Usable {
			return status
		}
		if status := probePulse(); status.Usable {
			return status
		}
		return AudioStatus{Reason: "Neither a reachable PipeWire session nor a reachable PulseAudio session was detected."}
	}
}

func hasFFmpegPulseMuxer(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		for _, field := range fields {
			if field == "pulse" {
				return true
			}
		}
	}
	return false
}

type pipeWireObject struct {
	Type string `json:"type"`
	Info struct {
		Props map[string]interface{} `json:"props"`
	} `json:"info"`
}

type pulseSink struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Properties  map[string]string `json:"properties"`
}

func resolveAudioTarget(ctx context.Context, runner commandRunner, backend, address string) (string, error) {
	normalized, err := NormalizeAddress(address)
	if err != nil {
		return "", err
	}
	compact := compactAddress(normalized)
	switch backend {
	case "pipewire":
		raw, err := runner.Output(ctx, "pw-dump")
		if err != nil {
			return "", fmt.Errorf("query PipeWire nodes: %w", err)
		}
		return parsePipeWireTarget(raw, compact)
	case "pulse":
		raw, err := runner.Output(ctx, "pactl", "-f", "json", "list", "sinks")
		if err != nil {
			return "", fmt.Errorf("query PulseAudio sinks: %w", err)
		}
		return parsePulseTarget(raw, compact)
	default:
		return "", codedError(ErrorAudioTargetUnavailable, fmt.Sprintf("unsupported Bluetooth audio backend %q", backend), nil)
	}
}

func parsePipeWireTarget(raw []byte, compactAddressValue string) (string, error) {
	var objects []pipeWireObject
	if err := json.Unmarshal(raw, &objects); err != nil {
		return "", fmt.Errorf("parse PipeWire nodes: %w", err)
	}
	for _, object := range objects {
		if object.Type != "PipeWire:Interface:Node" {
			continue
		}
		props := object.Info.Props
		if stringProperty(props, "media.class") != "Audio/Sink" {
			continue
		}
		haystack := strings.Join([]string{
			stringProperty(props, "api.bluez5.address"),
			stringProperty(props, "bluez5.address"),
			stringProperty(props, "device.name"),
			stringProperty(props, "node.name"),
		}, " ")
		if strings.Contains(compactAddress(haystack), compactAddressValue) {
			target := stringProperty(props, "node.name")
			if target != "" {
				return target, nil
			}
		}
	}
	return "", codedError(ErrorAudioTargetUnavailable, "The connected Bluetooth device has no PipeWire audio sink yet.", nil)
}

func parsePulseTarget(raw []byte, compactAddressValue string) (string, error) {
	var sinks []pulseSink
	if err := json.Unmarshal(raw, &sinks); err != nil {
		return "", fmt.Errorf("parse PulseAudio sinks: %w", err)
	}
	for _, sink := range sinks {
		haystack := sink.Name + " " + sink.Description
		for key, value := range sink.Properties {
			if strings.Contains(strings.ToLower(key), "bluez") ||
				strings.Contains(strings.ToLower(key), "address") ||
				strings.Contains(strings.ToLower(key), "device") {
				haystack += " " + value
			}
		}
		if strings.Contains(compactAddress(haystack), compactAddressValue) && sink.Name != "" {
			return sink.Name, nil
		}
	}
	return "", codedError(ErrorAudioTargetUnavailable, "The connected Bluetooth device has no PulseAudio sink yet.", nil)
}

func stringProperty(props map[string]interface{}, key string) string {
	value, _ := props[key].(string)
	return value
}

func compactAddress(value string) string {
	value = strings.ToUpper(value)
	var builder strings.Builder
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'A' && char <= 'F') {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}
