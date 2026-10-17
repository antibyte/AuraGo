package detective

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func canonicalURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", errors.New("source must use a public HTTP(S) URL without credentials")
	}
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		l := strings.ToLower(k)
		if strings.HasPrefix(l, "utm_") || l == "fbclid" || l == "gclid" {
			q.Del(k)
		}
		if strings.Contains(l, "token") || strings.Contains(l, "password") || strings.Contains(l, "api_key") || strings.Contains(l, "secret") {
			q.Set(k, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// RecordSource is only called by the server after an actual retrieval or search.
func (x *Session) RecordSource(raw, title, method, excerpt string, read bool) (Source, error) {
	u, err := canonicalURL(raw)
	if err != nil {
		return Source{}, err
	}
	return x.recordSource(u, "", title, method, excerpt, read)
}

// RecordReceipt records a server-observed integration response. A tool receipt
// is explicitly not a website URL and cannot be supplied by the report tool.
func (x *Session) RecordReceipt(method, locator, excerpt string) (Source, error) {
	return x.recordSource("", bounded(locator, 400), locator, method, excerpt, true)
}

func (x *Session) recordSource(u, locator, title, method, excerpt string, read bool) (Source, error) {
	excerpt = bounded(excerpt, 32000)
	h := sha256.Sum256([]byte(excerpt))
	hash := hex.EncodeToString(h[:])
	status := "search_hit"
	if read && excerpt != "" {
		status = "read"
	}
	out := Source{ID: id("src_"), URL: u, Locator: locator, Title: bounded(title, 400), Method: method, Status: status, Excerpt: excerpt, Hash: hash, RetrievedAt: x.service.now().UTC()}
	err := x.update(func(c *Case) error {
		if !activeStatus(c.Run.Status) || x.published {
			return context.Canceled
		}
		var stored []byte
		err := x.service.db.QueryRow("SELECT body FROM detective_sources WHERE case_id=? AND fingerprint=? ORDER BY seq LIMIT 1", c.ID, sourceFingerprint(out)).Scan(&stored)
		if err == nil {
			if err = json.Unmarshal(stored, &out); err != nil {
				return err
			}
			if out.Status == "read" {
				c.Run.SuccessfulChecks++
				if !slices.Contains(c.Run.CheckedSources, out.ID) && len(c.Run.CheckedSources) < 400 {
					c.Run.CheckedSources = append(c.Run.CheckedSources, out.ID)
				}
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		for _, v := range c.Sources {
			if v.URL == u && v.Locator == locator && v.Hash == hash && v.Status == status {
				out = v
				return nil
			}
		}
		if len(c.Sources) >= 400 || c.Run.Usage.Bytes+len(excerpt) > 8<<20 {
			return errors.New("source storage budget exhausted")
		}
		c.Sources = append(c.Sources, out)
		c.pendingSources = append(c.pendingSources, out)
		c.SourceCount++
		c.LatestSourceID = out.ID
		if status == "read" {
			c.Run.Usage.Pages++
			c.Run.SuccessfulChecks++
			c.Run.CheckedSources = append(c.Run.CheckedSources, out.ID)
		}
		c.Run.Usage.Bytes += len(excerpt)
		x.service.eventLocked(c, "source", out.Title)
		return nil
	})
	return out, err
}

func (x *Session) AddFinding(f Finding) (Finding, error) {
	f.ID = id("ev_")
	f.Text = bounded(f.Text, 4000)
	f.Quote = bounded(f.Quote, 2000)
	if f.Text == "" || f.Quote == "" {
		return f, errors.New("finding text and exact supporting quote are required")
	}
	if utf8.RuneCountInString(f.Quote) < 8 {
		return f, errors.New("quote is too short to support a finding")
	}
	err := x.update(func(c *Case) error {
		if !activeStatus(c.Run.Status) || x.published {
			return context.Canceled
		}
		var stored []byte
		var published int
		err := x.service.db.QueryRow("SELECT body,published_revision FROM detective_findings WHERE case_id=? AND fingerprint=? ORDER BY seq LIMIT 1", c.ID, findingFingerprint(f)).Scan(&stored, &published)
		if err == nil {
			if err = json.Unmarshal(stored, &f); err != nil {
				return err
			}
			if c.Run.Automatic && published == 0 && slices.Contains(c.Run.CheckedSources, f.SourceID) && !slices.Contains(c.Run.NewFindings, f.ID) {
				if len(c.Findings) >= 200 {
					return errors.New("finding limit reached")
				}
				c.Run.NewFindings = append(c.Run.NewFindings, f.ID)
				c.Findings = append(c.Findings, f)
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if len(c.Findings) >= 200 {
			return errors.New("finding limit reached")
		}
		source, err := x.service.sourceLocked(c.ID, f.SourceID)
		if err != nil {
			return err
		}
		if c.Run.Automatic && !slices.Contains(c.Run.CheckedSources, f.SourceID) {
			return errors.New("update findings require a source read during this run")
		}
		for _, s := range []Source{source} {
			if s.ID == f.SourceID && s.Status == "read" && strings.Contains(s.Excerpt, f.Quote) {
				c.Findings = append(c.Findings, f)
				c.pendingFindings = append(c.pendingFindings, f)
				c.FindingCount++
				if c.Run.Automatic {
					c.Run.NewFindings = append(c.Run.NewFindings, f.ID)
				}
				x.service.eventLocked(c, "finding", f.Text)
				return nil
			}
		}
		return errors.New("quote is not present in a retrieved source excerpt")
	})
	return f, err
}

func ValidateReport(c Case, r Report) error {
	encoded, err := json.Marshal(r.Blocks)
	if err != nil || len(encoded) > 1<<20 {
		return errors.New("report content exceeds 1 MiB")
	}
	if strings.TrimSpace(r.Title) == "" || len(r.Title) > 500 || len(r.Summary) > 8000 || len(r.Limitations) > 8000 || len(r.Blocks) == 0 || len(r.Blocks) > 100 {
		return errors.New("invalid report size or missing report blocks")
	}
	evidence := map[string]bool{}
	for _, f := range c.Findings {
		evidence[f.ID] = true
	}
	cited := 0
	for _, b := range r.Blocks {
		if len(b.Text) > 12000 || len(b.Items) > 100 || len(b.Rows) > 100 || len(b.Evidence) > 30 {
			return errors.New("report block exceeds limits")
		}
		switch b.Type {
		case "heading", "paragraph", "list", "table", "quote":
		default:
			return errors.New("unsupported report block")
		}
		for _, row := range b.Rows {
			if len(row) > 12 {
				return errors.New("table exceeds 12 columns")
			}
			for _, cell := range row {
				if len(cell) > 2000 {
					return errors.New("table cell too long")
				}
			}
		}
		for _, item := range b.Items {
			if len(item) > 2000 {
				return errors.New("list item too long")
			}
		}
		for _, ref := range b.Evidence {
			if !evidence[ref] {
				return fmt.Errorf("unknown evidence %q", ref)
			}
			cited++
		}
		if b.Type != "heading" && len(b.Evidence) == 0 && !r.Partial {
			return errors.New("each substantive block requires recorded evidence; describe research gaps in limitations")
		}
	}
	if cited == 0 && !r.Partial {
		return errors.New("report has no recorded evidence")
	}
	return nil
}
func sealReport(c *Case, r *Report, now time.Time) {
	r.Revision = c.LatestRevision + 1
	c.LatestRevision = r.Revision
	r.CreatedAt = now.UTC()
	r.Findings = append([]Finding(nil), c.Findings...)
	sources, _ := reportSources(Report{Blocks: r.Blocks, Findings: r.Findings, Sources: c.Sources})
	r.Sources = sources
	// Quotes are already snapshotted in Findings; keep source provenance without
	// duplicating every retrieved article into every report revision.
	for i := range r.Sources {
		r.Sources[i].Excerpt = ""
	}
}
func (x *Session) Submit(r Report) error {
	err := x.update(func(c *Case) error {
		if x.published {
			return errors.New("report already published")
		}
		if !activeStatus(c.Run.Status) {
			return context.Canceled
		}
		r.Sources, r.Findings, r.BaseRevision = nil, nil, 0
		if err := x.service.reportEvidenceLocked(c, &r); err != nil {
			return err
		}
		validation := *c
		validation.Findings, validation.Sources = r.Findings, r.Sources
		if err := ValidateReport(validation, r); err != nil {
			return err
		}
		if c.Run.Automatic {
			if r.Partial {
				return errors.New("automatic updates require a completed report")
			}
			fresh := false
			for _, f := range r.Findings {
				if slices.Contains(c.Run.NewFindings, f.ID) {
					fresh = true
				}
			}
			if !fresh {
				return errors.New("update must cite new findings; otherwise finish with no_changes")
			}
			if c.Run.ReportStyle == "append" {
				r.BaseRevision = c.Run.BaseRevision
			}
		}
		r.Revision, r.CreatedAt = c.LatestRevision+1, x.service.now().UTC()
		c.LatestRevision = r.Revision
		c.Reports = []Report{r}
		c.pendingReports = append(c.pendingReports, r)
		c.Run.Phase = "finished"
		c.Run.Status = "completed"
		if r.Partial {
			c.Run.Status = "partial"
		}
		if c.Run.Automatic {
			c.AutoUpdate.LastCheckedAt, c.AutoUpdate.LastChangedAt = r.CreatedAt, r.CreatedAt
			c.AutoUpdate.Outcome, c.AutoUpdate.Error = "updated", ""
		}
		if !r.Partial {
			nextUpdate(c, x.service.now())
		}
		return nil
	})
	if err == nil {
		x.service.mu.Lock()
		x.published = true
		x.service.mu.Unlock()
		x.Event("report", "report published")
	}
	return err
}
