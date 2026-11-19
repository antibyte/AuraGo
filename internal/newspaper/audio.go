package newspaper

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"aurago/internal/i18n"
)

// AudioNotices uses the reader's wording, with the print labels available offline.
func (e Edition) AudioNotices() (partial, singleSource string) {
	lang, _, _ := strings.Cut(e.Language, "-")
	localize := func(key, fallback string) string {
		if value := i18n.T(lang, key); value != key && value != "" {
			return value
		}
		return fallback
	}
	l := labels(e)
	return localize("newspaper.partial", l.Partial), localize("newspaper.single_source", l.SingleSource)
}

type Creation struct {
	Audio    *bool             `json:"audio,omitempty"`
	Channels map[string]string `json:"channels,omitempty"`
}

var deliveryChannels = []string{"email", "telegram", "whatsapp"}

func validFormat(v string) bool { return v == "edition" || v == "audio" || v == "both" }

func validateFormats(formats map[string]string, allowOff bool) error {
	for channel, format := range formats {
		if !contains(deliveryChannels, channel) || !(validFormat(format) || allowOff && format == "off") {
			return errors.New("invalid newspaper channel or format")
		}
	}
	return nil
}

func (p Profile) dailyChannel(channel string) bool {
	return channel == "email" && p.EmailDaily || channel == "telegram" && p.TelegramDaily || channel == "whatsapp" && p.WhatsAppDaily
}

func (p Profile) deliveryFormat(channel string) string {
	if format := p.Formats[channel]; format != "" {
		return format
	}
	return "edition"
}

type AudioFile struct {
	Data []byte
	MIME string
}

func (a AudioFile) Name(e Edition) string {
	ext := ".mp3"
	if a.MIME == "audio/wav" {
		ext = ".wav"
	}
	return fmt.Sprintf("newspaper-%s-r%d%s", e.LocalDate, e.Revision, ext)
}

const MaxAudioBytes = 32 << 20

type AudioState struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	MIME   string `json:"mime,omitempty"`
}

type audioJob struct {
	done chan struct{}
	err  error
}

func (s *Store) AudioState(ctx context.Context, id string) (AudioState, error) {
	var a AudioState
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(a.status,'missing'),COALESCE(a.reason,''),COALESCE(a.mime,'') FROM newspaper_editions e LEFT JOIN newspaper_audio a ON a.edition_id=e.id AND a.hash=e.hash WHERE e.id=?`, id).Scan(&a.Status, &a.Reason, &a.MIME)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return a, err
}

func (s *Store) Recording(ctx context.Context, id string) (AudioFile, error) {
	var a AudioFile
	err := s.db.QueryRowContext(ctx, `SELECT a.mime,a.data FROM newspaper_audio a JOIN newspaper_editions e ON e.id=a.edition_id AND e.hash=a.hash WHERE e.id=? AND a.status='ready'`, id).Scan(&a.MIME, &a.Data)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return a, err
}

// The service owns synthesis. Closing a browser or cancelling one waiter cannot stop other listeners.
func (s *Service) startAudio(ctx context.Context, e Edition, retry bool) (*audioJob, AudioState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.store.AudioState(ctx, e.ID)
	if err != nil || a.Status == "ready" {
		return nil, a, err
	}
	if job := s.audioJobs[e.ID]; job != nil {
		return job, a, nil
	}
	if a.Status == "failed" && !retry {
		return nil, a, errors.New("audio generation failed")
	}
	if !s.canWrite() || s.ctx.Err() != nil || s.narrate == nil {
		return nil, a, ErrDisabled
	}
	if len(s.audioJobs) >= 8 {
		return nil, a, ErrBusy
	}
	_, err = s.store.db.ExecContext(ctx, `INSERT INTO newspaper_audio(edition_id,hash,status) VALUES(?,?,'generating') ON CONFLICT(edition_id) DO UPDATE SET status='generating',reason=''`, e.ID, e.Hash)
	if err != nil {
		return nil, a, err
	}
	job := &audioJob{done: make(chan struct{})}
	s.audioJobs[e.ID] = job
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		work, cancel := context.WithTimeout(s.ctx, 15*time.Minute)
		defer cancel()
		var file AudioFile
		var err error
		select {
		case s.audioSlot <- struct{}{}:
			if s.canWrite() {
				file, err = s.narrate(work, e)
			} else {
				err = ErrDisabled
			}
			<-s.audioSlot
		case <-work.Done():
			err = work.Err()
		}
		if err == nil && (len(file.Data) == 0 || len(file.Data) > MaxAudioBytes || (file.MIME != "audio/mpeg" && file.MIME != "audio/wav")) {
			err = errors.New("invalid_audio")
		}
		if err == nil && (!s.canWrite() || work.Err() != nil) {
			err = ErrDisabled
		}
		if err == nil {
			_, err = s.store.db.ExecContext(work, `UPDATE newspaper_audio SET status='ready',mime=?,data=? WHERE edition_id=? AND hash=?`, file.MIME, file.Data, e.ID, e.Hash)
		}
		if err != nil {
			reason := "audio_failed"
			switch err.Error() {
			case "tts_unavailable", "script_failed", "script_budget_blocked", "script_model_failed", "tts_failed", "encoding_failed":
				reason = err.Error()
			}
			_, _ = s.store.db.Exec(`UPDATE newspaper_audio SET status='failed',reason=? WHERE edition_id=? AND status='generating'`, reason, e.ID)
		}
		s.mu.Lock()
		job.err = err
		delete(s.audioJobs, e.ID)
		close(job.done)
		s.mu.Unlock()
		s.wakeScheduler()
	}()
	return job, AudioState{Status: "generating"}, nil
}

func (s *Service) GenerateAudio(ctx context.Context, id string) (AudioState, error) {
	if !s.canWrite() {
		return AudioState{}, ErrDisabled
	}
	e, err := s.store.Get(ctx, id)
	if err != nil {
		return AudioState{}, err
	}
	_, a, err := s.startAudio(ctx, e, true)
	return a, err
}

func (s *Service) awaitRecording(ctx context.Context, e Edition, retry bool) (AudioFile, error) {
	job, _, err := s.startAudio(ctx, e, retry)
	if err != nil {
		return AudioFile{}, err
	}
	if job != nil {
		select {
		case <-ctx.Done():
			return AudioFile{}, ctx.Err()
		case <-job.done:
		}
		if job.err != nil {
			return AudioFile{}, job.err
		}
	}
	return s.store.Recording(ctx, e.ID)
}

func (s *Service) wakeScheduler() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) afterPublication(ctx context.Context, e Edition, p Profile, choice Creation) {
	render := p.AudioDaily
	if choice.Audio != nil {
		render = *choice.Audio
	}
	if render || (p.CastDaily && s.policy().Cast) {
		_, _, _ = s.startAudio(ctx, e, false)
	}
	for _, channel := range deliveryChannels {
		format, override := choice.Channels[channel]
		kind, key := "daily", "daily"
		if override {
			kind, key = "manual", "on-publication"
		} else {
			if !p.dailyChannel(channel) {
				continue
			}
			format = p.deliveryFormat(channel)
		}
		if format != "off" && ctx.Err() == nil {
			_, _ = s.deliverFormat(ctx, e.ID, channel, kind, key, format, false)
		}
	}
	s.wakeScheduler()
}
