package detective

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func defaultAutoUpdate(v AutoUpdateSettings) AutoUpdateSettings {
	if v.IntervalHours == 0 {
		v.IntervalHours = 24
	}
	if v.ReportStyle == "" {
		v.ReportStyle = "append"
	}
	if v.Effort == "" {
		v.Effort = "quick"
	}
	return v
}

func (s *Service) validateAutoUpdate(v AutoUpdateSettings) error {
	if v.IntervalHours < 1 || v.IntervalHours > 365*24 {
		return errors.New("update interval must be between 1 hour and 365 days")
	}
	if v.ReportStyle != "append" && v.ReportStyle != "rewrite" {
		return errors.New("unknown update report style")
	}
	if _, ok := s.profiles[v.Effort]; !ok {
		return errors.New("unknown update effort")
	}
	return nil
}

func (s *Service) SetAutoUpdate(key string, settings AutoUpdateSettings) (Case, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Case{}, ErrClosed
	}
	settings = defaultAutoUpdate(settings)
	if err := s.validateAutoUpdate(settings); err != nil {
		return Case{}, err
	}
	c, err := s.getLocked(key)
	if err != nil {
		return c, err
	}
	wasEnabled := c.AutoUpdate.Enabled
	intervalChanged := settings.IntervalHours != c.AutoUpdate.IntervalHours
	c.AutoUpdate.AutoUpdateSettings = settings
	c.Request.AutoUpdate = settings
	if !settings.Enabled {
		c.AutoUpdate.NextRunAt = time.Time{}
		c.AutoUpdate.Outcome = "paused"
		if c.Run.Automatic && activeStatus(c.Run.Status) {
			c.Run.Status, c.Run.Reason = "cancelled", "user_stopped"
			if s.active != nil && s.active.CaseID == key {
				s.active.stopReason = "user_stopped"
				s.active.cancel()
			}
		}
	} else if c.Run.Status == "waiting_for_user" {
		c.AutoUpdate.Outcome = "waiting_for_user"
		c.AutoUpdate.NextRunAt = time.Time{}
	} else if (!wasEnabled || intervalChanged) && !activeStatus(c.Run.Status) {
		c.AutoUpdate.Error = ""
		c.AutoUpdate.Outcome = "scheduled"
		if c.completeRevision() > 0 {
			c.AutoUpdate.NextRunAt = s.now().UTC().Add(time.Duration(settings.IntervalHours) * time.Hour)
		}
	}
	err = s.saveLocked(&c)
	if err == nil {
		s.eventLocked(&c, "auto_update", c.AutoUpdate.Outcome)
		s.wake()
	}
	return c, err
}

func nextUpdate(c *Case, now time.Time) {
	if c.AutoUpdate.Enabled && c.completeRevision() > 0 {
		c.AutoUpdate.NextRunAt = now.UTC().Add(time.Duration(c.AutoUpdate.IntervalHours) * time.Hour)
	} else {
		c.AutoUpdate.NextRunAt = time.Time{}
	}
}

// Called under the service lock. A claimed occurrence and its run are one write.
func (s *Service) queueDueLocked() error {
	if s.closed || s.runner == nil {
		return nil
	}
	var key string
	err := s.db.QueryRow(`SELECT id FROM detective_cases
WHERE json_extract(body,'$.auto_update.enabled')=1 AND status NOT IN ('queued','running','waiting_for_user')
AND ((status='interrupted' AND json_extract(body,'$.run.automatic')=1 AND json_extract(body,'$.run.reason') IN ('server_restart','server_shutdown'))
OR (NULLIF(json_extract(body,'$.auto_update.next_run_at'),'0001-01-01T00:00:00Z')<=? AND COALESCE(NULLIF(json_extract(body,'$.latest_complete_revision'),0), json_extract(body,'$.latest_revision'))>0))
ORDER BY updated LIMIT 1`, s.now().UTC().Format(time.RFC3339Nano)).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	c, err := s.getLocked(key)
	if err != nil {
		return err
	}
	if c.Run.Automatic && c.Run.Status == "interrupted" && (c.Run.Reason == "server_restart" || c.Run.Reason == "server_shutdown") {
		if c.Run.Usage.ActiveMS >= int64(c.Run.Profile.Seconds)*1000 || c.Run.Usage.Iterations >= c.Run.Profile.Iterations || (c.Run.Profile.Tokens > 0 && c.Run.Usage.PromptTokens+c.Run.Usage.CompletionTokens >= c.Run.Profile.Tokens) {
			c.Run.Status, c.Run.Reason = "partial", "budget_exhausted"
			c.AutoUpdate.Outcome, c.AutoUpdate.Error = "error", "budget_exhausted"
			nextUpdate(&c, s.now())
			return s.saveLocked(&c)
		}
		_, err = s.startLocked(key, "continue", "", id("resume_"))
		return err
	}
	_, err = s.startLocked(key, "update", c.AutoUpdate.Effort, "auto_"+c.AutoUpdate.NextRunAt.Format(time.RFC3339Nano))
	return err
}

func (x *Session) Checked() error {
	return x.update(func(c *Case) error {
		if !activeStatus(c.Run.Status) || x.published {
			return context.Canceled
		}
		c.Run.SuccessfulChecks++
		return nil
	})
}

func (x *Session) NoChanges() error {
	var topic string
	err := x.update(func(c *Case) error {
		if !c.Run.Automatic || !activeStatus(c.Run.Status) || x.published {
			return ErrConflict
		}
		if c.Run.SuccessfulChecks == 0 {
			return errors.New("complete a successful retrieval before reporting no changes")
		}
		if len(c.Run.NewFindings) > 0 {
			return errors.New("new findings must be reported or clarified")
		}
		c.Run.Status, c.Run.Phase, c.Run.Reason = "completed", "finished", "no_changes"
		c.AutoUpdate.LastCheckedAt = x.service.now().UTC()
		c.AutoUpdate.Outcome, c.AutoUpdate.Error = "no_changes", ""
		topic = c.Request.Topic
		nextUpdate(c, x.service.now())
		return nil
	})
	if err == nil {
		x.service.mu.Lock()
		x.published = true
		x.service.mu.Unlock()
		x.Event("auto_update", "no_changes")
		x.service.notify("info", x.CaseID, "Detective: no new information", topic)
	}
	return err
}

func (x *Session) History(kind string, before int64, query string, blockOffset int) (ArchivePage, error) {
	if kind == "report" {
		r, err := x.service.ReportRevision(x.CaseID, int(before))
		if err != nil {
			return ArchivePage{}, err
		}
		if blockOffset < 0 || blockOffset > len(r.Blocks) {
			return ArchivePage{}, errors.New("invalid report block offset")
		}
		end := min(blockOffset+5, len(r.Blocks))
		page := ArchivePage{}
		if end < len(r.Blocks) {
			page.NextBlock = end
		}
		r.Blocks, r.Findings, r.Sources = r.Blocks[blockOffset:end], nil, nil
		page.Reports = []Report{r}
		return page, nil
	}
	page, err := x.service.Archive(x.CaseID, kind, before, 20, query)
	for i := range page.Sources {
		page.Sources[i].Excerpt = bounded(page.Sources[i].Excerpt, 2000)
	}
	for i := range page.Findings {
		page.Findings[i].Text = bounded(page.Findings[i].Text, 2000)
		page.Findings[i].Quote = bounded(page.Findings[i].Quote, 1000)
	}
	return page, err
}

func PromptReport(r Report) Report {
	r.Summary, r.Limitations = bounded(r.Summary, 4000), bounded(r.Limitations, 2000)
	r.Sources, r.Findings = nil, nil
	blocks := append([]Block(nil), r.Blocks[:min(len(r.Blocks), 40)]...)
	for i := range blocks {
		blocks[i].Text = bounded(blocks[i].Text, 1000)
		blocks[i].Items = append([]string(nil), blocks[i].Items[:min(len(blocks[i].Items), 10)]...)
		for j := range blocks[i].Items {
			blocks[i].Items[j] = bounded(blocks[i].Items[j], 300)
		}
		blocks[i].Rows = append([][]string(nil), blocks[i].Rows[:min(len(blocks[i].Rows), 10)]...)
		for j := range blocks[i].Rows {
			blocks[i].Rows[j] = append([]string(nil), blocks[i].Rows[j]...)
			for k := range blocks[i].Rows[j] {
				blocks[i].Rows[j][k] = bounded(blocks[i].Rows[j][k], 100)
			}
		}
	}
	r.Blocks = blocks
	return r
}
