package server

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/persondetect"
	"aurago/internal/tools"
)

// personDetectionFrameWidth is the frame width person detection requests from go2rtc.
const personDetectionFrameWidth = 1280

// personDetectionZoneEdgeSnap is the distance from a frame edge within which a zone
// coordinate is mapped onto the edge. Hand-edited YAML and older editors leave
// zones a hair short of the edge, and a person cut off by the frame (Y2 = 1) would
// then miss a zone drawn "to the bottom". The stored config is not rewritten.
const personDetectionZoneEdgeSnap = 0.01

// personDetectionSettingsFromConfig maps the go2rtc configuration to the service
// settings: detection runs only while go2rtc and go2rtc.detection are enabled, and
// only streams with the stream and its detection enabled become cameras. It reads
// the configuration only, never the go2rtc container state: a stopped sidecar makes
// the cameras report frame errors until go2rtc runs again.
func personDetectionSettingsFromConfig(cfg *config.Config) persondetect.Settings {
	if cfg == nil {
		return persondetect.Settings{Location: time.Local}
	}
	detection := config.NormalizeGo2RTCDetectionSettings(cfg.Go2RTC.Detection)
	settings := persondetect.Settings{
		Enabled:       cfg.Go2RTC.Enabled && detection.Enabled,
		DataDir:       strings.TrimSpace(cfg.Directories.DataDir),
		Model:         persondetect.ModelTier(detection.Model),
		Backend:       persondetect.Backend(detection.Backend),
		RetentionDays: detection.RetentionDays,
		MaxStorageMB:  detection.MaxStorageMB,
		Schedule:      personDetectionWindows(detection.Schedule),
		Cameras:       []persondetect.Camera{},
		// There is no AuraGo-wide time zone setting: schedules follow the server's zone.
		Location: time.Local,
		Language: cfg.Server.UILanguage,
	}
	for _, stream := range cfg.Go2RTC.Streams {
		if !stream.Enabled || !stream.Detection.Enabled {
			continue
		}
		settings.Cameras = append(settings.Cameras, personDetectionCamera(stream.ID, stream.Name, stream.Detection))
	}
	return settings
}

// personDetectionWindows maps schedule windows with the config parsers, so days and
// clocks mean exactly what config validation accepted. Invalid entries (which
// config.Load rejects) are skipped instead of reaching the service.
func personDetectionWindows(windows []config.Go2RTCDetectionWindow) []persondetect.Window {
	out := make([]persondetect.Window, 0, len(windows))
	for _, window := range windows {
		from, fromOK := config.ParseGo2RTCDetectionClock(strings.TrimSpace(window.From))
		to, toOK := config.ParseGo2RTCDetectionClock(strings.TrimSpace(window.To))
		if !fromOK || !toOK {
			continue
		}
		days := make([]time.Weekday, 0, len(window.Days))
		for _, day := range window.Days {
			if weekday, ok := config.ParseGo2RTCDetectionDay(day); ok {
				days = append(days, weekday)
			}
		}
		if len(days) == 0 {
			continue
		}
		out = append(out, persondetect.Window{Days: days, From: from, To: to})
	}
	return out
}

// snapPersonDetectionCoordinate maps a zone coordinate within
// personDetectionZoneEdgeSnap of a frame edge onto that edge.
func snapPersonDetectionCoordinate(v float64) float64 {
	switch {
	case v <= personDetectionZoneEdgeSnap:
		return 0
	case v >= 1-personDetectionZoneEdgeSnap:
		return 1
	default:
		return v
	}
}

// personDetectionCamera maps one stream's (normalized) detection settings.
func personDetectionCamera(id, name string, d config.Go2RTCStreamDetectionConfig) persondetect.Camera {
	d = config.NormalizeGo2RTCStreamDetection(d)
	if strings.TrimSpace(name) == "" {
		name = id
	}
	zones := make([]persondetect.Zone, 0, len(d.Zones))
	for _, zone := range d.Zones {
		points := make([]persondetect.Point, 0, len(zone.Points))
		for _, p := range zone.Points {
			points = append(points, persondetect.Point{X: snapPersonDetectionCoordinate(p[0]), Y: snapPersonDetectionCoordinate(p[1])})
		}
		zones = append(zones, persondetect.Zone{Name: zone.Name, Points: points})
	}
	return persondetect.Camera{
		ID:       id,
		Name:     name,
		Interval: time.Duration(d.IntervalSeconds) * time.Second,
		MinScore: d.MinScore,
		Cooldown: time.Duration(d.CooldownSeconds) * time.Second,
		Zones:    zones,
		Actions: persondetect.Actions{
			Message: d.Actions.Message, Channel: d.Actions.Channel, Agent: d.Actions.Agent,
			AgentInstruction: d.Actions.AgentInstruction, Desktop: d.Actions.Desktop,
		},
	}
}

// go2RTCFrameSource adapts the go2rtc manager to persondetect.FrameSource.
type go2RTCFrameSource struct{ s *Server }

// Frame returns the current JPEG of an enabled stream, 1280 px wide. Frames are
// neither cached nor stored in the media registry.
func (f go2RTCFrameSource) Frame(ctx context.Context, cameraID string) ([]byte, error) {
	if f.s == nil || f.s.Go2RTC == nil {
		return nil, errors.New("go2rtc manager is not initialized")
	}
	_, data, err := f.s.Go2RTC.SnapshotBytes(ctx, cameraID, tools.Go2RTCSnapshotOptions{Width: personDetectionFrameWidth})
	return data, err
}

func newPersonDetectionService(s *Server, logger *slog.Logger) *persondetect.Service {
	return persondetect.NewService(persondetect.Deps{
		Logger: logger,
		Frames: go2RTCFrameSource{s: s},
		Alerts: s.newPersonAlertDispatcher(),
	})
}

// syncPersonDetectionSettings configures the service from the current snapshot.
// replaceConfigSnapshot calls it on every publication; it does nothing before
// startPersonDetection. personDetectSyncMu keeps overlapping publications in order.
func (s *Server) syncPersonDetectionSettings() {
	if s == nil || s.PersonDetect == nil || !s.personDetectStarted.Load() {
		return
	}
	s.personDetectSyncMu.Lock()
	defer s.personDetectSyncMu.Unlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	s.PersonDetect.Configure(personDetectionSettingsFromConfig(cfg))
}

// startPersonDetection lets config publications configure the service and applies
// the current snapshot.
func (s *Server) startPersonDetection() {
	if s == nil || s.PersonDetect == nil {
		return
	}
	s.personDetectStarted.Store(true)
	s.syncPersonDetectionSettings()
}

// shutdownPersonDetection withdraws the service from the agent tool and stops it.
func (s *Server) shutdownPersonDetection(ctx context.Context) {
	if s == nil || s.PersonDetect == nil {
		return
	}
	if persondetect.Default() == s.PersonDetect {
		persondetect.SetDefault(nil)
	}
	if err := s.PersonDetect.Shutdown(ctx); err != nil && s.Logger != nil {
		s.Logger.Warn("[PersonDetect] Shutdown did not complete", "error", err)
	}
}
