package bluetooth

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

var playbackSequence atomic.Uint64

type playbackSession struct {
	status  PlaybackStatus
	process runningProcess
}

type execPlaybackProcess struct {
	cancel context.CancelFunc
	cmds   []*exec.Cmd
	done   chan error
	once   sync.Once
}

func startPlaybackProcess(_ context.Context, backend, target, source string) (runningProcess, error) {
	if _, err := os.Stat(source); err != nil {
		return nil, fmt.Errorf("open playback source: %w", err)
	}
	playCtx, cancel := context.WithCancel(context.Background())
	process := &execPlaybackProcess{cancel: cancel, done: make(chan error, 1)}

	switch backend {
	case "pipewire":
		ffmpeg := exec.CommandContext(playCtx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-i", source, "-vn", "-f", "wav", "pipe:1")
		pwPlay := exec.CommandContext(playCtx, "pw-play", "--target", target, "-")
		reader, writer := io.Pipe()
		ffmpeg.Stdout = writer
		pwPlay.Stdin = reader
		process.cmds = []*exec.Cmd{ffmpeg, pwPlay}
		if err := pwPlay.Start(); err != nil {
			cancel()
			_ = reader.Close()
			_ = writer.Close()
			return nil, fmt.Errorf("start pw-play: %w", err)
		}
		if err := ffmpeg.Start(); err != nil {
			cancel()
			_ = reader.Close()
			_ = writer.Close()
			_ = pwPlay.Wait()
			return nil, fmt.Errorf("start FFmpeg decoder: %w", err)
		}
		go func() {
			ffmpegErr := ffmpeg.Wait()
			_ = writer.CloseWithError(ffmpegErr)
			pwErr := pwPlay.Wait()
			_ = reader.Close()
			if ffmpegErr != nil {
				process.done <- ffmpegErr
				return
			}
			process.done <- pwErr
		}()
	case "pulse":
		ffmpeg := exec.CommandContext(playCtx, "ffmpeg",
			"-nostdin", "-hide_banner", "-loglevel", "error",
			"-i", source, "-vn",
			"-device", target, "-f", "pulse", "AuraGo Bluetooth")
		process.cmds = []*exec.Cmd{ffmpeg}
		if err := ffmpeg.Start(); err != nil {
			cancel()
			return nil, fmt.Errorf("start FFmpeg PulseAudio playback: %w", err)
		}
		go func() {
			process.done <- ffmpeg.Wait()
		}()
	default:
		cancel()
		return nil, fmt.Errorf("unsupported audio backend %q", backend)
	}
	return process, nil
}

func (p *execPlaybackProcess) Wait() error {
	if p == nil {
		return nil
	}
	return <-p.done
}

func (p *execPlaybackProcess) Stop() error {
	if p == nil {
		return nil
	}
	p.once.Do(p.cancel)
	return nil
}

// Play starts asynchronous playback and replaces any previous AuraGo playback.
func (m *Manager) Play(ctx context.Context, source, requestedDevice string) (PlaybackStatus, error) {
	options, status, err := m.requirePlayback()
	if err != nil {
		return PlaybackStatus{}, err
	}
	device, err := m.ResolveTarget(ctx, requestedDevice)
	if err != nil {
		return PlaybackStatus{}, err
	}
	if !device.Paired {
		return PlaybackStatus{}, codedError(ErrorDeviceNotPaired, fmt.Sprintf("Bluetooth device %q is not paired.", displayDeviceName(device)), nil)
	}
	if !device.Connected {
		if err := m.adapter.Connect(ctx, device.Address); err != nil {
			return PlaybackStatus{}, fmt.Errorf("connect paired Bluetooth audio device: %w", err)
		}
	}

	var target string
	deadline := time.Now().Add(10 * time.Second)
	for {
		target, err = resolveAudioTarget(ctx, m.runner, status.Audio.Backend, device.Address)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return PlaybackStatus{}, err
		}
		select {
		case <-ctx.Done():
			return PlaybackStatus{}, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}

	process, err := m.runner.Start(context.Background(), status.Audio.Backend, target, source)
	if err != nil {
		return PlaybackStatus{}, fmt.Errorf("start Bluetooth playback: %w", err)
	}
	playback := &playbackSession{
		process: process,
		status: PlaybackStatus{
			ID:            fmt.Sprintf("bt-%d-%d", time.Now().UTC().UnixNano(), playbackSequence.Add(1)),
			State:         "playing",
			Source:        source,
			DeviceAddress: device.Address,
			DeviceName:    displayDeviceName(device),
			Backend:       status.Audio.Backend,
			Target:        target,
			StartedAt:     time.Now().UTC(),
		},
	}

	m.mu.Lock()
	previous := m.playback
	m.playback = playback
	m.options = options
	m.mu.Unlock()
	if previous != nil && previous.process != nil {
		_ = previous.process.Stop()
	}

	go m.waitForPlayback(playback)
	return playback.status, nil
}

func (m *Manager) waitForPlayback(playback *playbackSession) {
	err := playback.process.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.playback != playback {
		return
	}
	if playback.status.State == "stopped" {
		return
	}
	playback.status.FinishedAt = time.Now().UTC()
	if err != nil {
		playback.status.State = "error"
		playback.status.Error = err.Error()
	} else {
		playback.status.State = "finished"
	}
}

// PlaybackStatus returns the current or most recently completed AuraGo playback.
func (m *Manager) PlaybackStatus() PlaybackStatus {
	if m == nil {
		return PlaybackStatus{State: "idle"}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.playback == nil {
		return PlaybackStatus{State: "idle"}
	}
	return m.playback.status
}

// Stop stops only AuraGo-owned Bluetooth playback.
func (m *Manager) Stop() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	playback := m.playback
	if playback != nil && playback.status.State == "playing" {
		playback.status.State = "stopped"
		playback.status.FinishedAt = time.Now().UTC()
	}
	m.mu.Unlock()
	if playback != nil && playback.process != nil {
		return playback.process.Stop()
	}
	return nil
}

func displayDeviceName(device Device) string {
	if device.Alias != "" {
		return device.Alias
	}
	if device.Name != "" {
		return device.Name
	}
	return device.Address
}
