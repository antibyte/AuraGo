package rtlsdr

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"github.com/gofrs/flock"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Service struct {
	mu           sync.Mutex
	deviceMu     sync.Mutex
	db           *sql.DB
	lock         *flock.Flock
	opts         Options
	state        State
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	active       string
	recordCancel context.CancelFunc
	manualStop   bool
	deviceLost   bool
	listeners    map[string]time.Time
	scanCancel   context.CancelFunc
	scanBlock    string
	scanError    string
	scanAgent    bool
	transcribing map[string]context.CancelFunc
	asrAgents    map[string]bool
	finishing    map[string]bool
	announced    map[string]time.Time
	recovering   bool
	lastRecovery time.Time
	asrMu        sync.Mutex
}

func New(opts Options) (*Service, error) {
	if opts.Backend == nil || opts.Policy == nil {
		return nil, ErrInvalid
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if err := os.MkdirAll(opts.Directory, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(opts.Directory, "receiver.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrBusy
	}
	db, state, err := openStore(opts.Directory)
	if err != nil {
		lock.Unlock()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, lock: lock, opts: opts, state: state, ctx: ctx, cancel: cancel, listeners: map[string]time.Time{}, transcribing: map[string]context.CancelFunc{}, asrAgents: map[string]bool{}, finishing: map[string]bool{}, announced: map[string]time.Time{}}
	for i := range s.state.Recordings {
		r := &s.state.Recordings[i]
		if validID(r.ID) {
			if _, e := os.Stat(s.audioPath(r.ID) + ".deleting"); e == nil {
				_ = os.Rename(s.audioPath(r.ID)+".deleting", s.audioPath(r.ID))
			}
		}
		if r.Status == "preparing" || r.Status == "recording" || r.Status == "transcribing" {
			r.Status = "interrupted"
			r.Error = "sdr_interrupted"
			if st, e := os.Stat(s.audioPath(r.ID) + ".part"); e == nil && st.Size() > 4 {
				_ = os.Rename(s.audioPath(r.ID)+".part", s.audioPath(r.ID))
				r.Bytes = st.Size()
			}
			// Audio may have been finalized before a full disk prevented the
			// final ledger update. Recover that file under its original job ID.
			if st, e := os.Stat(s.audioPath(r.ID)); e == nil && st.Size() > 4 {
				r.Bytes = st.Size()
			}
		}
	}
	if err = s.saveLocked(); err != nil {
		s.Close()
		return nil, err
	}
	s.wg.Add(1)
	go s.loop()
	return s, nil
}

func newID() string          { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func validID(id string) bool { return len(id) == 32 && strings.Trim(id, "0123456789abcdef") == "" }
func (s *Service) policy() Policy {
	p := s.opts.Policy()
	if p.QuotaBytes <= 0 {
		p.QuotaBytes = DefaultQuota
	}
	return p
}
func (s *Service) writable() error {
	p := s.policy()
	if !p.Enabled {
		return ErrDisabled
	}
	if p.ReadOnly {
		return ErrReadOnly
	}
	if s.ctx.Err() != nil {
		return ErrUnavailable
	}
	return nil
}
func (s *Service) audioPath(id string) string { return filepath.Join(s.opts.Directory, id+".flac") }
func (s *Service) Close() error {
	s.mu.Lock()
	s.cancel()
	if s.recordCancel != nil {
		s.recordCancel()
	}
	if s.scanCancel != nil {
		s.scanCancel()
	}
	for _, c := range s.transcribing {
		c()
	}
	s.mu.Unlock()
	s.wg.Wait()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.opts.Backend.Stop(ctx)
	err := s.db.Close()
	_ = s.lock.Unlock()
	return err
}
func (s *Service) usedLocked() int64 {
	var n int64
	for _, r := range s.state.Recordings {
		n += r.Bytes
	}
	return n
}
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.policy()
	state := s.state
	state.Recordings = append([]Recording{}, s.state.Recordings...)
	for i := range state.Recordings {
		state.Recordings[i].Segments = nil
	}
	state = copyState(state)
	for i := range state.Schedules {
		v := &state.Schedules[i]
		if v.Enabled {
			v.NextStart = v.Next(maxTime(v.LastSlot, s.opts.Now().Add(-time.Second)))
		}
	}
	return Snapshot{State: state, Active: s.active, Listening: len(s.listeners) > 0, Scanning: s.scanCancel != nil, ScanBlock: s.scanBlock, ScanError: s.scanError, UsedBytes: s.usedLocked(), QuotaBytes: p.QuotaBytes, ReadOnly: p.ReadOnly}
}
func (s *Service) Receiver(ctx context.Context) (Receiver, error) {
	if !s.policy().Enabled {
		return Receiver{}, ErrDisabled
	}
	return s.opts.Backend.Info(ctx)
}
func (s *Service) Tune(ctx context.Context, client string, t Tuning) error {
	if err := s.writable(); err != nil {
		return err
	}
	if client == "" || len(client) > 100 {
		return ErrInvalid
	}
	if err := t.Validate(); err != nil {
		return err
	}
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	busy := s.active != "" || s.scanCancel != nil
	if _, exists := s.listeners[client]; !exists && len(s.listeners) >= 64 {
		s.mu.Unlock()
		return ErrQuota
	}
	s.mu.Unlock()
	if busy {
		return ErrBusy
	}
	if err := s.opts.Backend.Tune(ctx, t); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Tuning = t
	s.listeners[client] = s.opts.Now()
	return s.saveLocked()
}

func (s *Service) recoverLive(backend interface {
	Recover(context.Context, Tuning) error
}) {
	defer s.wg.Done()
	defer func() { s.mu.Lock(); s.recovering = false; s.mu.Unlock() }()
	if !s.deviceMu.TryLock() {
		return
	}
	defer s.deviceMu.Unlock()
	s.mu.Lock()
	idle := s.active == "" && s.scanCancel == nil && len(s.listeners) > 0
	tuning := s.state.Tuning
	s.mu.Unlock()
	if !idle || s.writable() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	_ = backend.Recover(ctx, tuning)
}
func (s *Service) Heartbeat(client string) error {
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.listeners[client]; !ok {
		return ErrNotFound
	}
	s.listeners[client] = s.opts.Now()
	return nil
}
func (s *Service) Stop(ctx context.Context, client string) error {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	s.mu.Lock()
	delete(s.listeners, client)
	stop := len(s.listeners) == 0 && s.active == "" && s.scanCancel == nil
	s.mu.Unlock()
	if stop {
		return s.opts.Backend.Stop(ctx)
	}
	return nil
}
func (s *Service) Stream(ctx context.Context, client string) (io.ReadCloser, error) {
	if !s.policy().Enabled {
		return nil, ErrDisabled
	}
	s.mu.Lock()
	_, ok := s.listeners[client]
	s.mu.Unlock()
	if !ok {
		return nil, ErrNotFound
	}
	return s.opts.Backend.Stream(ctx)
}
func (s *Service) SaveFavorite(st Station) (Station, error) {
	if err := s.writable(); err != nil {
		return st, err
	}
	if st.Name == "" || len(st.Name) > 120 {
		return st, ErrInvalid
	}
	if err := st.Tuning.Validate(); err != nil {
		return st, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state.Favorites) >= 200 {
		return st, ErrQuota
	}
	st.ID = newID()
	s.state.Favorites = append(s.state.Favorites, st)
	err := s.saveLocked()
	if err != nil {
		s.state.Favorites = s.state.Favorites[:len(s.state.Favorites)-1]
	}
	return st, err
}
func (s *Service) DeleteFavorite(id string) error {
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.state.Favorites {
		if v.ID == id {
			previous := append([]Station{}, s.state.Favorites...)
			s.state.Favorites = append(s.state.Favorites[:i], s.state.Favorites[i+1:]...)
			err := s.saveLocked()
			if err != nil {
				s.state.Favorites = previous
			}
			return err
		}
	}
	return ErrNotFound
}
func (s *Service) Recording(id string) (Recording, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range copyState(s.state).Recordings {
		if r.ID == id {
			return r, nil
		}
	}
	return Recording{}, ErrNotFound
}
func (s *Service) Audio(id string) (*os.File, error) {
	r, err := s.Recording(id)
	if err != nil {
		return nil, err
	}
	if !validID(id) || r.Bytes == 0 {
		return nil, ErrNotFound
	}
	return os.Open(s.audioPath(id))
}
func (s *Service) DeleteRecording(id string) error {
	if err := s.writable(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == id || s.transcribing[id] != nil || s.finishing[id] {
		return ErrBusy
	}
	for i, r := range s.state.Recordings {
		if r.ID == id {
			if !validID(id) {
				return ErrNotFound
			}
			path := s.audioPath(id)
			err := os.Rename(path, path+".deleting")
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			previous := append([]Recording{}, s.state.Recordings...)
			s.state.Recordings = append(s.state.Recordings[:i], s.state.Recordings[i+1:]...)
			if err := s.saveLocked(); err != nil {
				s.state.Recordings = previous
				_ = os.Rename(path+".deleting", path)
				return err
			}
			_ = os.Remove(path + ".deleting")
			if s.opts.Unregister != nil {
				return s.opts.Unregister(path)
			}
			return nil
		}
	}
	return ErrNotFound
}
func (s *Service) Scan(ctx context.Context) error {
	return s.ScanAs(ctx, false)
}
func (s *Service) ScanAs(ctx context.Context, agent bool) error {
	if err := s.writable(); err != nil {
		return err
	}
	if agent && !s.policy().AllowAgent {
		return ErrDisabled
	}
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return ErrUnavailable
	}
	if s.active != "" || s.scanCancel != nil {
		return ErrBusy
	}
	work, cancel := context.WithCancel(s.ctx)
	s.scanCancel = cancel
	s.scanAgent = agent
	s.scanError = ""
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		s.deviceMu.Lock()
		defer s.deviceMu.Unlock()
		if s.writable() != nil || (agent && !s.policy().AllowAgent) {
			cancel()
		}
		err := s.opts.Backend.Scan(work, func(stations []Station, block string) {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.scanBlock = block
			for _, st := range stations {
				found := false
				for i, v := range s.state.Stations {
					if v.ID == st.ID {
						s.state.Stations[i] = st
						found = true
						break
					}
				}
				if !found && len(s.state.Stations) < 1000 {
					s.state.Stations = append(s.state.Stations, st)
				}
			}
			_ = s.saveLocked()
		})
		s.mu.Lock()
		s.scanCancel = nil
		s.scanBlock = ""
		if err != nil && !errors.Is(err, context.Canceled) {
			s.scanError = "sdr_scan_failed"
		}
		resume := len(s.listeners) > 0
		t := s.state.Tuning
		s.mu.Unlock()
		if s.writable() == nil {
			if resume {
				_ = s.opts.Backend.Tune(s.ctx, t)
			} else {
				_ = s.opts.Backend.Stop(s.ctx)
			}
		} else {
			_ = s.opts.Backend.Stop(s.ctx)
		}
	}()
	return nil
}
func (s *Service) CancelScan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scanCancel != nil {
		s.scanCancel()
	}
}
func (s *Service) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.tick()
		}
	}
}
func (s *Service) tick() {
	now := s.opts.Now()
	p := s.policy()
	connected := true
	if backend, ok := s.opts.Backend.(interface{ DeviceConnected() bool }); ok {
		connected = backend.DeviceConnected()
	}
	s.mu.Lock()
	if !connected && s.recordCancel != nil {
		s.deviceLost = true
		s.recordCancel()
	}
	hadListeners := len(s.listeners) > 0
	for k, v := range s.listeners {
		if now.Sub(v) > LeaseTTL || !p.Enabled || p.ReadOnly {
			delete(s.listeners, k)
		}
	}
	stop := hadListeners && len(s.listeners) == 0 && s.active == "" && s.scanCancel == nil
	if !p.AllowAgent {
		if s.scanAgent && s.scanCancel != nil {
			s.scanCancel()
		}
		for id, agent := range s.asrAgents {
			if agent && s.transcribing[id] != nil {
				s.transcribing[id]()
			}
		}
		for _, r := range s.state.Recordings {
			if r.Agent {
				if s.active == r.ID && s.recordCancel != nil {
					s.recordCancel()
				}
				if cancel := s.transcribing[r.ID]; cancel != nil {
					cancel()
				}
			}
		}
	}
	if !p.Enabled || p.ReadOnly {
		if s.recordCancel != nil {
			s.recordCancel()
		}
		if s.scanCancel != nil {
			s.scanCancel()
		}
		for _, cancel := range s.transcribing {
			cancel()
		}
		s.mu.Unlock()
		if stop {
			s.stopIfIdle()
		}
		return
	}
	var due *Schedule
	notify := false
	for id, slot := range s.announced {
		if now.After(slot) {
			delete(s.announced, id)
		}
	}
	for i := range s.state.Schedules {
		v := &s.state.Schedules[i]
		if !v.Enabled || (v.Agent && !p.AllowAgent) {
			continue
		}
		slot := v.Next(v.LastSlot)
		if slot.After(now) && !slot.After(now.Add(time.Minute)) && !s.announced[v.ID].Equal(slot) {
			s.announced[v.ID] = slot
			notify = true
		}
		if slot.IsZero() || slot.After(now.Add(Warmup)) {
			continue
		}
		if slot.Before(now.Add(-3 * time.Second)) {
			v.LastSlot = now.Add(-3 * time.Second)
			s.state.Recordings = append(s.state.Recordings, Recording{ID: newID(), Name: v.Name, Tuning: v.Tuning, Start: slot, Status: "missed", Error: "sdr_missed", ScheduleID: v.ID, Created: now, Segments: []Segment{}})
			_ = s.saveLocked()
			continue
		}
		if due == nil || slot.Before(due.Start) {
			copy := *v
			copy.Start = slot
			due = &copy
		}
	}
	if due != nil && s.scanCancel != nil {
		s.scanCancel()
	}
	busy := s.active != "" || s.scanCancel != nil
	if due == nil && !busy && len(s.listeners) > 0 && !s.recovering && now.Sub(s.lastRecovery) > 10*time.Second && s.ctx.Err() == nil {
		if backend, ok := s.opts.Backend.(interface {
			Recover(context.Context, Tuning) error
		}); ok {
			s.recovering = true
			s.lastRecovery = now
			s.wg.Add(1)
			go s.recoverLive(backend)
		}
	}
	s.mu.Unlock()
	if notify && s.opts.NotifyUpcoming != nil {
		s.opts.NotifyUpcoming()
	}
	if due != nil && !busy {
		_, _ = s.startRecording(Recording{Name: due.Name, Tuning: due.Tuning, Start: due.Start, Duration: due.Duration, Transcribe: due.Transcribe, ScheduleID: due.ID, Agent: due.Agent})
	} else if stop {
		s.stopIfIdle()
	}
}

func (s *Service) stopIfIdle() {
	s.deviceMu.Lock()
	defer s.deviceMu.Unlock()
	s.mu.Lock()
	idle := len(s.listeners) == 0 && s.active == "" && s.scanCancel == nil
	s.mu.Unlock()
	if idle {
		ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
		defer cancel()
		_ = s.opts.Backend.Stop(ctx)
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func sortedBlocks() []string {
	keys := make([]string, 0, len(DABBlocks))
	for k := range DABBlocks {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return DABBlocks[keys[i]] < DABBlocks[keys[j]] })
	return keys
}
