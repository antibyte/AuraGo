package personalradio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func (s *Service) scheduleProductionLocked(p Station, tracks []Track) {
	if s.musicActive {
		s.state.MusicBusy = true
	}
	if s.editorActive {
		s.state.EditorBusy = true
	}
	goal := int64(p.LibraryMinutes) * 60000
	if s.state.Status == "paused" {
		goal = max(s.state.RequiredMS+15*60000, int64(p.ReserveMinutes)*60000)
	}
	if p.Mode != "local" && s.state.BufferMS < goal && !s.musicActive && !s.now().Before(s.musicRetry) {
		var pending Production
		var pendingID, body string
		if s.db.QueryRow("SELECT id,result FROM jobs WHERE station=? AND kind='music' AND status='ready_to_import' ORDER BY rowid LIMIT 1", p.ID).Scan(&pendingID, &body) == nil && json.Unmarshal([]byte(body), &pending) == nil {
			s.musicActive = true
			s.state.MusicBusy = true
			s.wg.Add(1)
			go s.produceMusic(s.runCtx, s.state.Epoch, pendingID, p, pending.Genre, "", &pending)
		} else if s.adapters.Generate == nil {
			s.state.Code = "radio_music_unavailable"
		} else if id, err := s.reserve(p.ID, "music", 1, p.DailyGenerations); err != nil {
			s.state.Code = errorCode(err, "radio_storage_error")
			s.musicRetry = s.now().Add(time.Minute)
		} else {
			s.musicActive = true
			s.state.MusicBusy = true
			ctx, epoch := s.runCtx, s.state.Epoch
			genre := generationGenre(p, tracks)
			idea := s.musicIdea
			s.wg.Add(1)
			go s.produceMusic(ctx, epoch, id, p, genre, idea, nil)
		}
	}
	if s.editorActive || s.now().Before(s.editorRetry) || s.state.Status == "paused" || s.state.Status == "preparing" || s.adapters.Plan == nil {
		return
	}
	news := p.NewsMinutes > 0 && !s.state.NextNews.IsZero() && !s.now().Before(s.state.NextNews.Add(-8*time.Minute)) && s.newsAttempt != s.state.NextNews && s.state.Current != ""
	if news && s.adapters.Research == nil {
		s.state.NewsCode = "radio_news_unavailable"
		s.newsAttempt = s.state.NextNews
		news = false
	}
	cadence := 4
	if p.Moderation == "little" {
		cadence = 6
	}
	if p.Moderation == "much" {
		cadence = 2
	}
	moderation := s.lastEditorialPlay < 0 || s.plays-s.lastEditorialPlay >= cadence
	if !news && !moderation {
		return
	}
	id, err := s.reserve(p.ID, "editorial", 1, p.DailyEditorial)
	if err != nil {
		if news {
			s.state.NewsCode = errorCode(err, "radio_storage_error")
		} else {
			s.state.EditorialCode = errorCode(err, "radio_storage_error")
		}
		s.editorRetry = s.now().Add(time.Minute)
		return
	}
	s.editorActive = true
	s.state.EditorBusy = true
	s.lastEditorialPlay = s.plays
	if news {
		s.newsAttempt = s.state.NextNews
	}
	req := EditorialRequest{Station: p, Tracks: slices.Clone(tracks[:min(40, len(tracks))]), Recent: s.recent(p.ID), News: news}
	ctx, epoch, due := s.runCtx, s.state.Epoch, s.state.NextNews
	s.wg.Add(1)
	go s.produceEditorial(ctx, epoch, id, req, due)
}
func generationGenre(p Station, tracks []Track) string {
	best := p.Genres[0].Name
	lowest := float64(1 << 62)
	for _, g := range p.Genres {
		var total int64
		for _, t := range tracks {
			if t.Origin == "generated" && strings.EqualFold(t.Genre, g.Name) {
				total += t.DurationMS
			}
		}
		ratio := float64(total) / float64(g.Weight)
		if ratio < lowest {
			best = g.Name
			lowest = ratio
		}
	}
	return best
}
func (s *Service) produceMusic(parent context.Context, epoch, job string, p Station, genre, idea string, pending *Production) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()
	start := s.now()
	var result Production
	var err error
	if pending != nil {
		result = *pending
	} else {
		result, err = s.adapters.Generate(ctx, p, genre, idea)
	}
	if err == nil {
		body, _ := json.Marshal(result)
		s.mu.Lock()
		_, err = s.db.Exec("UPDATE jobs SET status='ready_to_import',result=? WHERE id=?", string(body), job)
		s.mu.Unlock()
	}
	if err == nil && result.MediaID == 0 && s.adapters.Register != nil {
		result, err = s.adapters.Register(ctx, result)
	}
	var track Track
	if err == nil {
		f, e := os.Open(result.Path)
		if e != nil {
			err = e
		} else {
			track, err = s.Import(ctx, p.ID, f, filepath.Ext(result.Path), result.Title, "generated", genre, result.MediaID)
			f.Close()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.musicActive = false
	s.state.MusicBusy = false
	if err == nil || result.Path == "" || strings.HasPrefix(err.Error(), "radio_invalid") || strings.HasPrefix(err.Error(), "radio_format") {
		s.finishJob(job, err, track.ID)
	}
	if s.state.Epoch != epoch {
		return
	}
	if err != nil {
		s.state.Code = errorCode(err, "radio_generation_failed")
		s.musicRetry = s.now().Add(time.Minute)
	} else {
		s.state.Code = ""
		s.latencies = append(s.latencies, s.now().Sub(start))
		if len(s.latencies) > 20 {
			s.latencies = s.latencies[1:]
		}
	}
	if s.adapters.Issue != nil && !errors.Is(err, context.Canceled) {
		s.adapters.Issue("music", err != nil)
	}
}
func (s *Service) produceEditorial(parent context.Context, epoch, job string, req EditorialRequest, due time.Time) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(parent, 4*time.Minute)
	defer cancel()
	var err error
	var plan Plan
	var audio Audio
	var segment Segment
	var ttsJob string
	if req.News {
		req.Sources, err = s.adapters.Research(ctx, req.Station)
		if err == nil {
			s.mu.Lock()
			prior := s.loadNews(req.Station.ID)
			s.mu.Unlock()
			req.Sources = slices.DeleteFunc(req.Sources, func(src Source) bool {
				for _, edition := range prior {
					if edition.Aired.IsZero() {
						continue
					}
					for _, used := range edition.Sources {
						if used.ID == src.ID && used.Published.Equal(src.Published) {
							return true
						}
					}
				}
				return false
			})
		}
		if err == nil && len(req.Sources) == 0 {
			err = errors.New("radio_no_verified_news")
		}
	}
	if err == nil {
		plan, err = s.adapters.Plan(ctx, req)
	}
	text := strings.TrimSpace(plan.Moderation)
	kind := "moderation"
	spoken := req.News || req.Station.Moderation != "off"
	if req.News {
		text = strings.TrimSpace(plan.NewsText)
		kind = "news"
	}
	if err == nil && spoken {
		limit := 800
		if req.News {
			limit = 3200
		}
		if text == "" || len([]rune(text)) > limit {
			err = errors.New("radio_invalid_editorial")
		}
	}
	var sources []Source
	if err == nil && req.News {
		for _, id := range plan.SourceIDs {
			idx := slices.IndexFunc(req.Sources, func(x Source) bool { return x.ID == id })
			if idx < 0 {
				err = errors.New("radio_invalid_sources")
				break
			}
			src := req.Sources[idx]
			src.Text = ""
			sources = append(sources, src)
		}
		if len(sources) == 0 {
			err = errors.New("radio_invalid_sources")
		}
	}
	if err == nil && spoken {
		s.mu.Lock()
		if s.state.Epoch != epoch || s.closed {
			err = context.Canceled
		} else {
			ttsJob, err = s.reserve(req.Station.ID, "tts_chars", len([]rune(text)), req.Station.DailyTTSChars)
		}
		s.mu.Unlock()
	}
	if err == nil && spoken {
		if s.adapters.Speak == nil {
			err = errors.New("radio_tts_unavailable")
		} else {
			audio, err = s.adapters.Speak(ctx, req.Station, text)
		}
	}
	if err == nil && spoken {
		id := newID()
		var track Track
		track, err = prepareAudio(ctx, bytes.NewReader(audio.Data), audio.Extension, filepath.Join(s.dir, id+".wav"))
		if err == nil {
			segment = Segment{ID: newID(), AssetID: id, Kind: kind, Title: req.Station.Name, Text: text, DurationMS: track.DurationMS, Rate: track.Rate, Channels: track.Channels, Sources: sources, Expires: s.now().Add(30 * time.Minute)}
			if req.News {
				segment.Due = due
				segment.Expires = due.Add(time.Duration(req.Station.NewsMinutes) * time.Minute)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.editorActive = false
	s.state.EditorBusy = false
	s.finishJob(job, err, segment.ID)
	if ttsJob != "" {
		s.finishJob(ttsJob, err, segment.ID)
	}
	if s.state.Epoch == epoch && s.adapters.Issue != nil && !errors.Is(err, context.Canceled) {
		operation := "moderation"
		if req.News {
			operation = "news"
		}
		s.adapters.Issue(operation, err != nil)
	}
	if err != nil || s.state.Epoch != epoch || s.closed {
		if segment.AssetID != "" {
			_ = os.Remove(filepath.Join(s.dir, segment.AssetID+".wav"))
		}
		if s.state.Epoch == epoch {
			if req.News {
				s.state.NewsCode = errorCode(err, "radio_editorial_failed")
			} else {
				s.state.EditorialCode = errorCode(err, "radio_editorial_failed")
			}
			s.editorRetry = s.now().Add(time.Minute)
		}
		return
	}
	if req.News {
		s.state.NewsCode = ""
	} else {
		s.state.EditorialCode = ""
	}
	s.state.Theme = bounded(plan.Theme, 200)
	s.musicIdea = bounded(plan.MusicIdea, 1000)
	s.preferred = nil
	for _, id := range plan.TrackIDs {
		if slices.Contains(s.preferred, id) {
			continue
		}
		if slices.ContainsFunc(req.Tracks, func(t Track) bool { return t.ID == id }) {
			s.preferred = append(s.preferred, id)
		}
	}
	if !spoken {
		return
	}
	if req.News {
		// At most one edition per deadline; it may air late at a title boundary,
		// but never after its expiry or be replayed after a long pause.
		b, _ := json.Marshal(segment)
		if _, err = s.db.Exec("INSERT INTO news(id,station,body) VALUES(?,?,?)", segment.ID, req.Station.ID, string(b)); err != nil {
			s.state.NewsCode = "radio_storage_error"
			os.Remove(filepath.Join(s.dir, segment.AssetID+".wav"))
			return
		}
		s.state.News = append([]Segment{segment}, s.state.News...)
		_, _ = s.db.Exec("DELETE FROM news WHERE rowid NOT IN (SELECT rowid FROM news ORDER BY rowid DESC LIMIT 384)")
		if len(s.state.News) > 12 {
			s.state.News = s.state.News[:12]
		}
	}
	if !s.now().Before(segment.Expires) {
		os.Remove(filepath.Join(s.dir, segment.AssetID+".wav"))
		return
	}
	// Keep the current and next music transition immutable for the player's
	// lookahead. Future news is retained separately until its due time.
	s.pendingSpeech = append(s.pendingSpeech, segment)
	_, err = s.db.Exec("INSERT INTO editorial(station,created,body) VALUES(?,?,?)", req.Station.ID, s.now().UTC().Format(time.RFC3339Nano), bounded(text, 500))
	if err != nil {
		s.state.NewsCode = "radio_storage_error"
	}
	_, _ = s.db.Exec("DELETE FROM editorial WHERE rowid NOT IN (SELECT rowid FROM editorial ORDER BY created DESC LIMIT 500)")
}
func bounded(v string, n int) string {
	r := []rune(strings.TrimSpace(v))
	return string(r[:min(n, len(r))])
}

func (s *Service) insertSpeechLocked(p Station) {
	pending := s.pendingSpeech[:0]
	for _, x := range s.pendingSpeech {
		if !s.now().Before(x.Expires) {
			os.Remove(filepath.Join(s.dir, x.AssetID+".wav"))
			continue
		}
		if !x.Due.IsZero() && s.now().Before(x.Due) {
			pending = append(pending, x)
			continue
		}
		if len(s.state.Queue) < 2 {
			pending = append(pending, x)
			continue
		}
		idx := 2
		if s.state.Queue[0].Kind != "music" || s.state.Queue[1].Kind != "music" {
			idx = 3
		}
		if len(s.state.Queue) < idx || (len(s.state.Queue) > idx && s.state.Queue[idx].Kind != "music") {
			pending = append(pending, x)
			continue
		}
		// Moderation must not promise a specific title: users may skip freely.
		s.state.Queue = slices.Insert(s.state.Queue, idx, x)
	}
	s.pendingSpeech = pending
	if p.NewsMinutes > 0 && !s.state.NextNews.IsZero() && s.now().After(s.state.NextNews.Add(time.Duration(p.NewsMinutes)*time.Minute)) {
		s.state.NextNews = nextNews(s.now(), p)
	} else if p.NewsMinutes > 0 && s.newsAttempt == s.state.NextNews && !s.now().Before(s.state.NextNews) {
		s.state.NextNews = nextNews(s.now(), p)
	}
}
