package detective

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
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
		if status == "read" {
			c.Run.Usage.Pages++
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
	err := x.update(func(c *Case) error {
		if len(c.Findings) >= 200 {
			return errors.New("finding limit reached")
		}
		for _, s := range c.Sources {
			if s.ID == f.SourceID && s.Status == "read" && strings.Contains(s.Excerpt, f.Quote) {
				c.Findings = append(c.Findings, f)
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
	r.Revision = len(c.Reports) + 1
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
	return x.update(func(c *Case) error {
		if x.published {
			return errors.New("report already published")
		}
		if len(c.Reports) >= 30 {
			return errors.New("report revision limit reached")
		}
		if err := ValidateReport(*c, r); err != nil {
			return err
		}
		sealReport(c, &r, x.service.now())
		c.Reports = append(c.Reports, r)
		c.Run.Phase = "finished"
		c.Run.Status = "completed"
		if r.Partial {
			c.Run.Status = "partial"
		}
		x.published = true
		x.service.eventLocked(c, "report", "report published")
		return nil
	})
}
