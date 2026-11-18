package agent

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// PlayCastRecording publishes only the supplied recording. beforeLoad durably claims the external playback.
func PlayCastRecording(ctx context.Context, cfg *config.Config, addr string, port int, db *sql.DB, log *slog.Logger, data []byte, mime string, beforeLoad func() error) error {
	if cfg == nil || !cfg.Chromecast.Enabled || cfg.Chromecast.ReadOnly || beforeLoad == nil {
		return errors.New("Cast playback disabled")
	}
	name := "audio.mp3"
	if mime == "audio/wav" {
		name = "audio.wav"
	} else if mime != "audio/mpeg" {
		return errors.New("unsupported recording")
	}
	if len(data) == 0 || int64(len(data)) > tools.CastMediaMaxBytes {
		return errors.New("invalid recording size")
	}
	host, err := chromecastAdvertiseHost(cfg)
	if err != nil {
		return err
	}
	control := chromecastControlConfig(ctx, cfg, host, db, log)
	if err = tools.AuthorizeChromecastDial(addr, port, control.KnownDevices); err != nil {
		return err
	}
	req := chromecastArgs{ContentType: mime}
	if err = publishChromecastMedia(cfg, &req, name, bytes.NewReader(data)); err != nil {
		return err
	}
	attempted := false
	control.BeforeLoad = func() error {
		if err := beforeLoad(); err != nil {
			return err
		}
		attempted = true
		return nil
	}
	res := tools.ChromecastPlay(addr, port, req.URL, mime, "buffered", control, log)
	if !attempted {
		if u, err := url.Parse(req.URL); err == nil {
			_ = os.Remove(filepath.Join(tools.CastMediaDir(cfg.Directories.DataDir), filepath.Base(u.Path)))
		}
	}
	var result struct{ Status string }
	if json.Unmarshal([]byte(res), &result) != nil || result.Status != "success" {
		return errors.New("Cast playback was not confirmed")
	}
	return nil
}
