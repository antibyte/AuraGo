package personalradio

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

type Options struct {
	Directory string
	Adapters  Adapters
	Now       func() time.Time
	Manual    bool
	Enabled   func() bool
}
type Service struct {
	mu                sync.Mutex
	db                *sql.DB
	dir               string
	adapters          Adapters
	now               func() time.Time
	ctx               context.Context
	cancel            context.CancelFunc
	runCtx            context.Context
	runCancel         context.CancelFunc
	wg                sync.WaitGroup
	state             State
	leaseUntil        time.Time
	lastProgress      time.Time
	lastPosition      int64
	closed            bool
	enabled           func() bool
	musicRetry        time.Time
	editorRetry       time.Time
	latencies         []time.Duration
	preferred         []string
	musicIdea         string
	lastEditorialPlay int
	plays             int
	newsAttempt       time.Time
	musicActive       bool
	editorActive      bool
	pendingSpeech     []Segment
	lastStopOwner     string
	lastStopEpoch     string
	lastSkipped       []string
}

func New(o Options) (*Service, error) {
	db, err := openStore(o.Directory)
	if err != nil {
		return nil, err
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, dir: o.Directory, adapters: o.Adapters, now: o.Now, enabled: o.Enabled, ctx: ctx, cancel: cancel, state: State{Status: "stopped", Queue: []Segment{}, News: []Segment{}}}
	if !o.Manual {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.Tick()
				}
			}
		}()
	}
	return s, nil
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.stopLocked()
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
	return s.db.Close()
}
func (s *Service) stopLocked() {
	if s.state.Status != "stopped" {
		s.lastStopOwner = s.state.Owner
		s.lastStopEpoch = s.state.Epoch
	}
	if s.runCancel != nil {
		s.runCancel()
	}
	for _, x := range append(slices.Clone(s.state.Queue), s.pendingSpeech...) {
		if x.Kind != "music" {
			_ = os.Remove(filepath.Join(s.dir, x.AssetID+".wav"))
		}
	}
	s.pendingSpeech = nil
	s.state.Status = "stopped"
	s.state.Owner = ""
	s.state.Epoch = newID()
	s.state.Current = ""
	s.state.Queue = []Segment{}
	s.leaseUntil = time.Time{}
	s.preferred = nil
}

func (s *Service) Start(id, device string, takeover bool) (State, error) {
	if len(device) < 16 || len(device) > 80 {
		return State{}, ErrLease
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return State{}, ErrConflict
	}
	if s.state.Status != "stopped" && s.now().Before(s.leaseUntil) {
		if s.state.Owner == device && s.state.StationID == id {
			return s.snapshotLocked(), nil
		}
		if !takeover {
			return State{}, ErrLease
		}
	}
	p, err := s.station(id)
	if err != nil {
		return State{}, err
	}
	s.stopLocked()
	s.runCtx, s.runCancel = context.WithCancel(s.ctx)
	s.state = State{Status: "preparing", StationID: id, Owner: device, Epoch: newID(), Queue: []Segment{}, News: s.loadNews(id)}
	s.leaseUntil = s.now().Add(3 * time.Minute)
	s.lastProgress = s.now()
	s.lastPosition = -1
	s.plays = 0
	s.lastSkipped = nil
	s.lastEditorialPlay = -1
	s.newsAttempt = time.Time{}
	s.musicRetry = time.Time{}
	s.editorRetry = time.Time{}
	s.state.NextNews = nextNews(s.now(), p)
	s.updateBufferLocked(p)
	return s.snapshotLocked(), nil
}

func (s *Service) Snapshot() State { s.mu.Lock(); defer s.mu.Unlock(); return s.snapshotLocked() }
func (s *Service) snapshotLocked() State {
	b, _ := json.Marshal(s.state)
	var out State
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Service) checkLease(device, epoch string) error {
	if s.state.Owner != device || s.state.Epoch != epoch || s.state.Status == "stopped" || s.now().After(s.leaseUntil) {
		return ErrLease
	}
	return nil
}
func (s *Service) Heartbeat(device, epoch, current string, position int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLease(device, epoch); err != nil {
		return err
	}
	if current == s.state.Current && position >= 0 && position != s.lastPosition {
		s.lastProgress = s.now()
		s.lastPosition = position
	}
	// A live tab may prepare/pause, but a stalled "playing" client cannot fund
	// endless generation merely by sending keepalives.
	if s.state.Status == "playing" && s.now().Sub(s.lastProgress) > 3*time.Minute {
		return ErrLease
	}
	s.leaseUntil = s.now().Add(3 * time.Minute)
	return nil
}
func (s *Service) Control(device, epoch, action, current string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if action == "stop" && s.state.Status == "stopped" && device == s.lastStopOwner && epoch == s.lastStopEpoch && epoch != "" {
		return nil
	}
	if err := s.checkLease(device, epoch); err != nil {
		return err
	}
	switch action {
	case "stop":
		s.stopLocked()
	case "pause":
		if s.state.Status == "playing" || s.state.Status == "ready" || s.state.Status == "buffering" {
			s.state.Status = "paused"
		}
	case "resume":
		if s.state.Status == "paused" {
			s.state.Status = "ready"
			if len(s.state.Queue) > 0 && s.state.Queue[0].ID == s.state.Current && !s.state.Queue[0].Expires.IsZero() && !s.now().Before(s.state.Queue[0].Expires) {
				s.dropFirstLocked("expired")
				s.state.Current = ""
			}
			if s.state.Current != "" {
				s.state.Status = "playing"
			}
			s.lastProgress = s.now()
			s.discardExpiredLocked()
		}
	case "skip":
		if slices.Contains(s.lastSkipped, current) {
			return nil
		}
		if current == "" || len(s.state.Queue) == 0 || s.state.Queue[0].ID != current {
			return ErrConflict
		}
		s.lastSkipped = append(s.lastSkipped, current)
		if len(s.lastSkipped) > 32 {
			s.lastSkipped = s.lastSkipped[1:]
		}
		if len(s.state.Queue) > 0 {
			s.dropFirstLocked("skipped")
		}
		s.state.Current = ""
		if s.state.Status != "paused" {
			s.state.Status = "ready"
		}
	default:
		return ErrConflict
	}
	return nil
}

// Playback events are accepted only for the current transition, and recorded
// once. A browser refresh or retry cannot count a planned track as heard twice.
func (s *Service) Playback(device, epoch, id, kind string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkLease(device, epoch); err != nil {
		return err
	}
	if kind != "started" && kind != "ended" && kind != "failed" {
		return ErrConflict
	}
	if kind == "started" && s.state.Current == id {
		return nil
	}
	idx := slices.IndexFunc(s.state.Queue, func(x Segment) bool { return x.ID == id })
	if idx < 0 {
		var exists int
		if s.db.QueryRow("SELECT 1 FROM plays WHERE segment=? AND station=?", id, s.state.StationID).Scan(&exists) == nil {
			return nil
		}
		return ErrConflict
	}
	if kind == "started" {
		if idx > 1 || s.state.Status == "preparing" || s.state.Status == "paused" {
			return ErrConflict
		}
		x := s.state.Queue[idx]
		if !x.Expires.IsZero() && !s.now().Before(x.Expires) {
			return ErrConflict
		}
		if !x.Due.IsZero() && s.now().Before(x.Due) {
			return ErrConflict
		}
		if idx == 1 && s.state.Current != s.state.Queue[0].ID {
			return ErrConflict
		}
		if x.TrackID != "" {
			r, err := s.db.Exec("INSERT INTO plays(segment,station,asset,started) VALUES(?,?,?,?) ON CONFLICT DO NOTHING", x.ID, s.state.StationID, x.TrackID, s.now().UTC().Format(time.RFC3339Nano))
			if err != nil {
				return err
			}
			n, _ := r.RowsAffected()
			if n > 0 {
				s.plays++
			}
		}
		if idx == 1 {
			s.dropFirstLocked("ended")
		}
		s.state.Current = id
		s.state.Status = "playing"
		if x.Kind == "news" {
			for i := range s.state.News {
				if s.state.News[i].ID == id {
					s.state.News[i].Aired = s.now()
					b, _ := json.Marshal(s.state.News[i])
					_, _ = s.db.Exec("UPDATE news SET body=? WHERE id=?", string(b), id)
				}
			}
		}
		s.lastProgress = s.now()
		s.lastPosition = -1
	} else if idx == 0 {
		// A transient fetch failure must not permanently blacklist valid music.
		s.dropFirstLocked(kind)
		s.state.Current = ""
		if s.state.Status != "paused" {
			s.state.Status = "ready"
		}
	} else if kind == "failed" && idx >= 1 {
		x := s.state.Queue[idx]
		s.state.Queue = slices.Delete(s.state.Queue, idx, idx+1)
		if x.Kind != "music" {
			_ = os.Remove(filepath.Join(s.dir, x.AssetID+".wav"))
		}
	}
	return nil
}
func (s *Service) dropFirstLocked(outcome string) {
	if len(s.state.Queue) == 0 {
		return
	}
	x := s.state.Queue[0]
	_, err := s.db.Exec("UPDATE plays SET ended=?,outcome=? WHERE segment=? AND ended IS NULL", s.now().UTC().Format(time.RFC3339Nano), outcome, x.ID)
	if err != nil {
		s.state.Code = "radio_storage_error"
	}
	s.state.Queue = s.state.Queue[1:]
	if x.Kind != "music" {
		_ = os.Remove(filepath.Join(s.dir, x.AssetID+".wav"))
	}
}

func (s *Service) updateBufferLocked(p Station) []Track {
	tracks, err := s.tracks(p.ID)
	if err != nil {
		s.state.Code = "radio_storage_error"
		return nil
	}
	eligible := []Track{}
	s.state.BufferMS = 0
	for _, t := range tracks {
		if !trackAllowed(t, p) {
			continue
		}
		if fi, e := os.Stat(filepath.Join(s.dir, t.ID+".wav")); e != nil || fi.Size() != t.Bytes {
			continue
		}
		eligible = append(eligible, t)
		s.state.BufferMS += t.DurationMS
	}
	s.state.TrackCount = len(eligible)
	s.state.RequiredMS = int64(p.ReserveMinutes) * 60000
	if p.Mode != "local" && len(s.latencies) > 0 {
		lat := slices.Clone(s.latencies)
		slices.Sort(lat)
		estimate := lat[int(float64(len(lat)-1)*0.95)]
		s.state.RequiredMS = max(s.state.RequiredMS, (2*estimate + time.Minute).Milliseconds())
	}
	_ = s.db.QueryRow("SELECT COALESCE(SUM(amount),0) FROM jobs WHERE station=? AND kind='music' AND day=?", p.ID, s.now().UTC().Format("2006-01-02")).Scan(&s.state.GeneratedToday)
	return eligible
}

func (s *Service) Tick() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.state.Status == "stopped" {
		return
	}
	if s.enabled != nil && !s.enabled() {
		s.stopLocked()
		s.state.Code = "radio_unavailable"
		return
	}
	if s.now().After(s.leaseUntil) {
		s.stopLocked()
		s.state.Code = "radio_listener_gone"
		return
	}
	p, err := s.station(s.state.StationID)
	if err != nil {
		s.stopLocked()
		return
	}
	tracks := s.updateBufferLocked(p)
	s.discardExpiredLocked()
	if s.state.RequiredMS > 180*60000 {
		s.state.Code = "radio_production_too_slow"
		return
	}
	if s.state.Status == "preparing" {
		if s.state.BufferMS >= s.state.RequiredMS && len(tracks) >= p.MinTracks {
			s.fillQueueLocked(p, tracks)
			if len(s.state.Queue) >= 2 {
				s.state.Status = "ready"
				s.state.Code = ""
			} else {
				s.state.Code = "radio_more_music_needed"
			}
		} else if p.Mode == "local" {
			s.state.Code = "radio_more_music_needed"
		}
	}
	if s.state.Status != "preparing" {
		s.fillQueueLocked(p, tracks)
		if len(s.state.Queue) == 0 && s.state.Status != "paused" {
			s.state.Status = "buffering"
		} else if len(s.state.Queue) > 0 && s.state.Status == "buffering" {
			s.state.Status = "ready"
		}
	}
	s.insertSpeechLocked(p)
	s.scheduleProductionLocked(p, tracks)
}
func (s *Service) discardExpiredLocked() {
	q := s.state.Queue[:0]
	for _, x := range s.state.Queue {
		if x.ID != s.state.Current && !x.Expires.IsZero() && !s.now().Before(x.Expires) {
			_ = os.Remove(filepath.Join(s.dir, x.AssetID+".wav"))
			continue
		}
		q = append(q, x)
	}
	s.state.Queue = q
}
func (s *Service) loadNews(id string) []Segment {
	out := []Segment{}
	rows, err := s.db.Query("SELECT body FROM news WHERE station=? ORDER BY rowid DESC LIMIT 12", id)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		var x Segment
		if rows.Scan(&b) == nil && json.Unmarshal([]byte(b), &x) == nil {
			out = append(out, x)
		}
	}
	return out
}

func errorCode(err error, fallback string) string {
	if errors.Is(err, ErrLimit) {
		return "radio_limit"
	}
	if err != nil {
		switch err.Error() {
		case "radio_tts_unavailable", "radio_llm_unavailable", "radio_music_unavailable":
			return err.Error()
		}
	}
	return fallback
}
