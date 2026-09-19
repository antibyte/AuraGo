package personalradio

import (
	"slices"
	"strings"
	"time"
)

func trackAllowed(t Track, p Station) bool {
	if t.Blocked || t.DurationMS <= 0 {
		return false
	}
	if p.Mode == "local" && t.Origin != "local" || p.Mode == "generated" && t.Origin != "generated" {
		return false
	}
	if t.Genre == "" {
		return true
	}
	for _, g := range p.Genres {
		if strings.EqualFold(g.Name, t.Genre) {
			return true
		}
	}
	return false
}

func (s *Service) fillQueueLocked(p Station, tracks []Track) {
	recent, err := s.playedIDs(p.ID)
	if err != nil {
		s.state.Code = "radio_storage_error"
		return
	}
	now := s.now()
	last := map[string]time.Time{}
	for _, t := range tracks {
		last[t.ID] = t.LastPlayed
	}
	if !s.state.MusicReady && p.Strict {
		remaining := slices.Clone(tracks)
		simRecent := slices.Clone(recent)
		simLast := map[string]time.Time{}
		for k, v := range last {
			simLast[k] = v
		}
		at := now
		var buffer int64
		count := 0
		for len(remaining) > 0 {
			t, _, ok := selectTrack(p, remaining, simRecent, simLast, at, nil)
			if !ok {
				break
			}
			buffer += t.DurationMS
			count++
			simRecent = append(simRecent, t.ID)
			simLast[t.ID] = at
			at = at.Add(time.Duration(t.DurationMS) * time.Millisecond)
			remaining = slices.DeleteFunc(remaining, func(x Track) bool { return x.ID == t.ID })
		}
		if buffer < s.state.RequiredMS || count < p.MinTracks {
			return
		}
	}
	for _, x := range s.state.Queue {
		if x.TrackID != "" {
			if len(recent) == 0 || recent[len(recent)-1] != x.TrackID {
				recent = append(recent, x.TrackID)
			}
			last[x.TrackID] = now
		}
		now = now.Add(time.Duration(x.DurationMS) * time.Millisecond)
	}
	for len(s.state.Queue) < 4 {
		t, relaxed, ok := selectTrack(p, tracks, recent, last, now, s.preferred)
		if !ok {
			break
		}
		s.state.Relaxed = relaxed
		s.state.Queue = append(s.state.Queue, Segment{ID: newID(), AssetID: t.ID, TrackID: t.ID, Kind: "music", Title: t.Title, DurationMS: t.DurationMS, Rate: t.Rate, Channels: t.Channels})
		recent = append(recent, t.ID)
		last[t.ID] = now
		now = now.Add(time.Duration(t.DurationMS) * time.Millisecond)
		s.preferred = slices.DeleteFunc(s.preferred, func(id string) bool { return id == t.ID })
	}
}
func (s *Service) playedIDs(station string) ([]string, error) {
	recent := []string{}
	rows, err := s.db.Query("SELECT asset FROM plays WHERE station=? ORDER BY started DESC LIMIT 201", station)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			recent = append(recent, id)
		}
	}
	rows.Close()
	slices.Reverse(recent)
	return recent, rows.Err()
}

func selectTrack(p Station, tracks []Track, recent []string, last map[string]time.Time, at time.Time, preferred []string) (Track, bool, bool) {
	var candidates []Track
	relaxed := false
	for pass := 0; pass < 2; pass++ {
		for _, t := range tracks {
			if !trackAllowed(t, p) {
				continue
			}
			if len(recent) > 0 && recent[len(recent)-1] == t.ID {
				continue
			}
			if pass == 0 && ((!last[t.ID].IsZero() && at.Sub(last[t.ID]) < time.Duration(p.RepeatMinutes)*time.Minute) || slices.Contains(recent[max(0, len(recent)-p.RepeatTracks):], t.ID)) {
				continue
			}
			candidates = append(candidates, t)
		}
		if len(candidates) > 0 {
			relaxed = pass > 0
			break
		}
		if p.Strict {
			break
		}
	}
	if len(candidates) == 0 {
		return Track{}, false, false
	}
	var generated, total int64
	byID := make(map[string]Track, len(tracks))
	genreMinutes := map[string]int64{}
	genreWeights := map[string]int{}
	preference := map[string]int{}
	for _, t := range tracks {
		byID[t.ID] = t
	}
	for _, g := range p.Genres {
		genreWeights[strings.ToLower(g.Name)] = g.Weight
	}
	for i, id := range preferred {
		preference[id] = len(preferred) - i
	}
	for _, id := range recent {
		if t, ok := byID[id]; ok {
			total += t.DurationMS
			genreMinutes[strings.ToLower(t.Genre)] += t.DurationMS
			if t.Origin == "generated" {
				generated += t.DurationMS
			}
		}
	}

	wantGenerated := total == 0 || generated*100 < int64(p.GeneratedPercent)*total
	score := func(t Track) float64 {
		v := float64(t.Plays) * 10
		if !last[t.ID].IsZero() {
			v += mathRecency(at.Sub(last[t.ID]))
		}
		v -= float64(t.Weight) / 100
		if t.Favorite {
			v -= 0.4
		}
		if p.Mode == "mixed" && (t.Origin == "generated") == wantGenerated {
			v -= 100
		}
		v -= float64(preference[t.ID]) * 0.2
		if weight := genreWeights[strings.ToLower(t.Genre)]; weight > 0 {
			v += float64(genreMinutes[strings.ToLower(t.Genre)]) / 60000 / float64(weight)
		}

		return v
	}
	best := candidates[0]
	bestScore := score(best)
	for _, candidate := range candidates[1:] {
		value := score(candidate)
		if value < bestScore || (value == bestScore && candidate.ID < best.ID) {
			best = candidate
			bestScore = value
		}
	}
	return best, relaxed, true
}
func mathRecency(d time.Duration) float64 { return max(0, 60-d.Minutes()) }

func nextNews(now time.Time, p Station) time.Time {
	if p.NewsMinutes == 0 {
		return time.Time{}
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return time.Time{}
	}
	// Iterate absolute minutes to handle DST folds and gaps without duplicating
	// the same deadline or inventing a nonexistent local half hour.
	t := now.Truncate(time.Minute).Add(time.Minute)
	for i := 0; i < 125; i++ {
		if t.In(loc).Minute()%p.NewsMinutes == 0 {
			return t
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}
}
