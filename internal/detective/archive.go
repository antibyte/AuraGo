package detective

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// The case body is a bounded working set; immutable evidence lives in the archive.
func sourceFingerprint(v Source) string {
	return digest(v.URL + "\x00" + v.Locator + "\x00" + v.Hash + "\x00" + v.Status)
}

func findingFingerprint(v Finding) string {
	return digest(strings.ToLower(strings.Join(strings.Fields(v.Text), " ")))
}

func digest(v string) string {
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:])
}

func insertSource(tx *sql.Tx, key string, v Source) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT OR IGNORE INTO detective_sources(case_id,id,fingerprint,body) VALUES(?,?,?,?)", key, v.ID, sourceFingerprint(v), b)
	return err
}

func insertFinding(tx *sql.Tx, key string, v Finding) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT OR IGNORE INTO detective_findings(case_id,id,fingerprint,body) VALUES(?,?,?,?)", key, v.ID, findingFingerprint(v), b)
	return err
}

func insertReport(tx *sql.Tx, key string, v Report) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO detective_reports(case_id,revision,body) VALUES(?,?,?)", key, v.Revision, b)
	if err != nil {
		return err
	}
	for _, block := range v.Blocks {
		for _, id := range block.Evidence {
			if _, err = tx.Exec("UPDATE detective_findings SET published_revision=? WHERE case_id=? AND id=? AND published_revision=0", v.Revision, key, id); err != nil {
				return err
			}
		}
	}
	return err
}

func saveCaseTx(tx *sql.Tx, c *Case) error {
	for _, v := range c.pendingSources {
		if err := insertSource(tx, c.ID, v); err != nil {
			return err
		}
	}
	for _, v := range c.pendingFindings {
		if err := insertFinding(tx, c.ID, v); err != nil {
			return err
		}
	}
	for _, v := range c.pendingReports {
		if err := insertReport(tx, c.ID, v); err != nil {
			return err
		}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	// Keep the bounded summary in its own table; the live view and the list read only
	// this row instead of the evidence body.
	summary, err := json.Marshal(caseSummary(c))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO detective_summaries(case_id, body) VALUES(?,?)
ON CONFLICT(case_id) DO UPDATE SET body=excluded.body`, c.ID, string(summary)); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO detective_cases(id,body,status,updated,active_ms) VALUES(?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET body=excluded.body,status=excluded.status,updated=excluded.updated,active_ms=excluded.active_ms`,
		c.ID, b, c.Run.Status, c.UpdatedAt.Format(time.RFC3339Nano), c.Run.Usage.ActiveMS)
	return err
}

type ArchivePage struct {
	Reports   []Report  `json:"reports,omitempty"`
	Sources   []Source  `json:"sources,omitempty"`
	Findings  []Finding `json:"findings,omitempty"`
	Next      int64     `json:"next"`
	NextBlock int       `json:"next_block,omitempty"`
}

func (s *Service) Archive(key, kind string, before int64, limit int, query string) (ArchivePage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.archiveLocked(key, kind, before, limit, query)
}

func (s *Service) archiveLocked(key, kind string, before int64, limit int, query string) (ArchivePage, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()
	var page ArchivePage
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 || before < 0 || len(query) > 200 {
		return page, errors.New("invalid archive page")
	}
	table, sequence, body, field := "", "seq", "body", "$.text"
	switch kind {
	case "sources":
		table, field = "detective_sources", "$.title"
	case "findings":
		table = "detective_findings"
	case "reports":
		table, sequence, field = "detective_reports", "revision", "$.title"
		body = "json_remove(CAST(body AS TEXT),'$.blocks','$.sources','$.findings')"
	default:
		return page, errors.New("unknown archive kind")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM detective_cases WHERE id=?", key).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return page, ErrNotFound
	} else if err != nil {
		return page, err
	}
	q := "SELECT " + sequence + "," + body + " FROM " + table + " WHERE case_id=?"
	args := []any{key}
	if before > 0 {
		q += " AND " + sequence + "<?"
		args = append(args, before)
	}
	if query != "" {
		q += " AND instr(lower(json_extract(body,?)),lower(?))>0"
		args = append(args, field, query)
	}
	q += " ORDER BY " + sequence + " DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var last int64
	for n := 0; rows.Next(); n++ {
		var seq int64
		var b []byte
		if err = rows.Scan(&seq, &b); err != nil {
			return page, err
		}
		if n == limit {
			page.Next = last
			break
		}
		last = seq
		switch kind {
		case "sources":
			var v Source
			err = json.Unmarshal(b, &v)
			page.Sources = append(page.Sources, v)
		case "findings":
			var v Finding
			err = json.Unmarshal(b, &v)
			page.Findings = append(page.Findings, v)
		case "reports":
			var v Report
			err = json.Unmarshal(b, &v)
			page.Reports = append(page.Reports, v)
		}
		if err != nil {
			return page, err
		}
	}
	return page, rows.Err()
}

func (s *Service) ReportRevision(key string, revision int) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reportLocked(key, revision)
}

func (s *Service) reportLocked(key string, revision int) (Report, error) {
	var r Report
	var b []byte
	err := s.db.QueryRow("SELECT body FROM detective_reports WHERE case_id=? AND revision=?", key, revision).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(b, &r)
	}
	return r, err
}

func (s *Service) Source(key, id string) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sourceLocked(key, id)
}

func (s *Service) sourceLocked(key, id string) (Source, error) {
	var v Source
	var b []byte
	err := s.db.QueryRow("SELECT body FROM detective_sources WHERE case_id=? AND id=?", key, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	if err == nil {
		err = json.Unmarshal(b, &v)
	}
	return v, err
}

// Resolve only the requested immutable chain, with a ceiling before allocation.
func (s *Service) resolvedReport(ctx context.Context, key string, revision int) (Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var parts []Report
	bytes := 0
	for next := revision; next > 0; {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		var b []byte
		var size int
		if err := s.db.QueryRow("SELECT length(body) FROM detective_reports WHERE case_id=? AND revision=?", key, next).Scan(&size); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = ErrNotFound
			}
			return Report{}, err
		}
		bytes += size
		if bytes > 16<<20 || len(parts) >= 4096 {
			return Report{}, errors.New("report is too large to export; select an earlier revision")
		}
		if err := s.db.QueryRow("SELECT body FROM detective_reports WHERE case_id=? AND revision=?", key, next).Scan(&b); err != nil {
			return Report{}, err
		}
		var r Report
		if err := json.Unmarshal(b, &r); err != nil {
			return Report{}, err
		}
		if r.BaseRevision < 0 || r.BaseRevision >= next {
			return Report{}, errors.New("invalid report ancestry")
		}
		parts = append(parts, r)
		next = r.BaseRevision
	}
	if len(parts) == 0 {
		return Report{}, ErrNotFound
	}
	out := parts[len(parts)-1]
	if len(parts) > 1 && out.Limitations != "" {
		out.Blocks = append(out.Blocks, Block{Type: "heading", Text: "Limitations"}, Block{Type: "paragraph", Text: out.Limitations})
		out.Limitations = ""
	}
	seenSources, seenFindings := map[string]bool{}, map[string]bool{}
	for _, v := range out.Sources {
		seenSources[v.ID] = true
	}
	for _, v := range out.Findings {
		seenFindings[v.ID] = true
	}
	for i := len(parts) - 2; i >= 0; i-- {
		r := parts[i]
		out.Blocks = append(out.Blocks, Block{Type: "heading", Text: r.CreatedAt.UTC().Format("2006-01-02 15:04 UTC") + " · " + r.Title})
		if r.Summary != "" {
			out.Blocks = append(out.Blocks, Block{Type: "paragraph", Text: r.Summary})
		}
		out.Blocks = append(out.Blocks, r.Blocks...)
		if r.Limitations != "" {
			out.Blocks = append(out.Blocks, Block{Type: "heading", Text: "Limitations"}, Block{Type: "paragraph", Text: r.Limitations})
		}
		for _, v := range r.Sources {
			if !seenSources[v.ID] {
				out.Sources = append(out.Sources, v)
				seenSources[v.ID] = true
			}
		}
		for _, v := range r.Findings {
			if !seenFindings[v.ID] {
				out.Findings = append(out.Findings, v)
				seenFindings[v.ID] = true
			}
		}
	}
	out.Revision, out.CreatedAt = revision, parts[0].CreatedAt
	out.BaseRevision = 0
	return out, nil
}

func (s *Service) reportEvidenceLocked(c *Case, r *Report) error {
	if len(r.Blocks) == 0 || len(r.Blocks) > 100 {
		return errors.New("report requires between 1 and 100 blocks")
	}
	body, err := json.Marshal(r.Blocks)
	if err != nil || len(body) > 1<<20 {
		return errors.New("report content exceeds 1 MiB")
	}
	for _, block := range r.Blocks {
		if len(block.Evidence) > 30 {
			return errors.New("report block exceeds 30 evidence references")
		}
	}
	seen := map[string]bool{}
	seenSources := map[string]bool{}
	for _, block := range r.Blocks {
		for _, id := range block.Evidence {
			if seen[id] {
				continue
			}
			seen[id] = true
			var b []byte
			if err := s.db.QueryRow("SELECT body FROM detective_findings WHERE case_id=? AND id=?", c.ID, id).Scan(&b); err != nil {
				return fmt.Errorf("unknown evidence %q", id)
			}
			var f Finding
			if err := json.Unmarshal(b, &f); err != nil {
				return err
			}
			src, err := s.sourceLocked(c.ID, f.SourceID)
			if err != nil {
				return err
			}
			r.Findings = append(r.Findings, f)
			src.Excerpt = ""
			if !seenSources[src.ID] {
				r.Sources = append(r.Sources, src)
				seenSources[src.ID] = true
			}
		}
	}
	return nil
}
