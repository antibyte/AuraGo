package detective

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
)

// ExportRevision publishes complete bytes transactionally and reuses them on retry.
// The export slot is independent of the research worker and its budget.
func (s *Service) ExportRevision(ctx context.Context, key string, revision int, format string) (Artifact, error) {
	select {
	case s.exports <- struct{}{}:
		defer func() { <-s.exports }()
	case <-ctx.Done():
		return Artifact{}, ctx.Err()
	}
	if format != "md" && format != "pdf" && format != "docx" {
		return Artifact{}, errors.New("unsupported export format")
	}
	c, err := s.Get(key)
	if err != nil {
		return Artifact{}, err
	}
	if revision < 1 || revision > len(c.Reports) {
		return Artifact{}, ErrNotFound
	}
	s.mu.Lock()
	var a Artifact
	err = s.db.QueryRow("SELECT body,mime,filename,sha256 FROM detective_artifacts WHERE case_id=? AND revision=? AND format=?", key, revision, format).Scan(&a.Data, &a.MIME, &a.Filename, &a.SHA256)
	s.mu.Unlock()
	if err == nil {
		return a, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, err
	}
	if format == "pdf" && s.pdf != nil {
		a = Artifact{Filename: fmt.Sprintf("detective-report-r%d.pdf", revision), MIME: "application/pdf"}
		a.Data, err = s.pdf(ctx, c.Reports[revision-1])
	} else {
		a, err = Export(c.Reports[revision-1], format)
	}
	if err != nil {
		return a, err
	}
	if len(a.Data) > 32<<20 {
		return Artifact{}, errors.New("export exceeds 32 MiB")
	}
	if err = ctx.Err(); err != nil {
		return Artifact{}, err
	}
	sum := sha256.Sum256(a.Data)
	a.SHA256 = hex.EncodeToString(sum[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err = s.getLocked(key); err != nil {
		return Artifact{}, err
	}
	_, err = s.db.Exec("INSERT INTO detective_artifacts(case_id,revision,format,body,mime,filename,sha256) VALUES(?,?,?,?,?,?,?)", key, revision, format, a.Data, a.MIME, a.Filename, a.SHA256)
	if err != nil {
		return Artifact{}, fmt.Errorf("publish research export: %w", err)
	}
	return a, nil
}

func (s *Service) Ready() bool { s.mu.Lock(); defer s.mu.Unlock(); return !s.closed && s.runner != nil }
