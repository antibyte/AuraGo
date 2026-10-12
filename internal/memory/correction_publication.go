package memory

import "fmt"

func (s *SQLiteMemory) initCorrectionPublication() error {
	var exists, populated int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memory_correction_vectors'`).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return nil
	}
	if err := s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM core_memory)+(SELECT COUNT(*) FROM memory_meta)+(SELECT COUNT(*) FROM user_profile)+(SELECT COUNT(*) FROM memory_corrections)`).Scan(&populated); err != nil {
		return err
	}
	if populated > 0 {
		if _, err := backupMainDatabase(s.db, "memory-correction-publication-v1"); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE memory_correction_vectors (
		doc_id TEXT PRIMARY KEY, correction_id TEXT NOT NULL, published INTEGER NOT NULL DEFAULT 0);
	INSERT INTO memory_correction_vectors(doc_id,correction_id,published)
		SELECT json_extract(i.value,'$.replacement_id'),c.id,json_extract(i.value,'$.state')='done'
		FROM memory_corrections c,json_each(c.payload,'$.items') i
		WHERE json_extract(i.value,'$.reference.store')='ltm' AND json_extract(i.value,'$.operation')='replace'
		AND COALESCE(json_extract(i.value,'$.replacement_id'),'')<>'';
	INSERT INTO memory_meta(doc_id,verification_status,archived_at,archived_reason,source_type)
		SELECT doc_id,'archived',CURRENT_TIMESTAMP,'staged correction '||correction_id,'memory_correction'
		FROM memory_correction_vectors WHERE published=0
		ON CONFLICT(doc_id) DO UPDATE SET verification_status='archived',archived_at=CURRENT_TIMESTAMP,archived_reason=excluded.archived_reason;
	INSERT INTO memory_correction_tombstones(reference,correction_id,previous_value)
		SELECT json_object('store','profile','category',l.category,'key',l.key),l.correction_id,json_extract(i.value,'$.value')
		FROM memory_profile_corrections l JOIN memory_corrections c ON c.id=l.correction_id,json_each(c.payload,'$.items') i
		WHERE json_extract(i.value,'$.reference.store')='profile' AND json_extract(i.value,'$.reference.category')=l.category
		AND json_extract(i.value,'$.reference.key')=l.key AND json_extract(i.value,'$.state')='done'
		AND json_extract(i.value,'$.operation')='replace'
		AND NOT EXISTS(SELECT 1 FROM user_profile p WHERE p.category=l.category AND p.key=l.key)
		ON CONFLICT(reference) DO NOTHING;
	CREATE TRIGGER correction_vector_insert BEFORE INSERT ON memory_meta
	WHEN EXISTS(SELECT 1 FROM memory_correction_vectors WHERE doc_id=NEW.doc_id AND published=0)
		AND COALESCE(NEW.verification_status,'')<>'archived' AND COALESCE(NEW.archived_at,'')=''
	BEGIN SELECT RAISE(ABORT,'correction replacement is not published'); END;
	CREATE TRIGGER correction_vector_update BEFORE UPDATE ON memory_meta
	WHEN (EXISTS(SELECT 1 FROM memory_correction_vectors WHERE doc_id=OLD.doc_id AND published=0) AND NEW.doc_id<>OLD.doc_id)
		OR (EXISTS(SELECT 1 FROM memory_correction_vectors WHERE doc_id=NEW.doc_id AND published=0)
		AND COALESCE(NEW.verification_status,'')<>'archived' AND COALESCE(NEW.archived_at,'')='')
	BEGIN SELECT RAISE(ABORT,'correction replacement is not published'); END;
	CREATE TRIGGER correction_vector_delete BEFORE DELETE ON memory_meta
	WHEN EXISTS(SELECT 1 FROM memory_correction_vectors WHERE doc_id=OLD.doc_id AND published=0)
	BEGIN SELECT RAISE(ABORT,'retain unpublished correction metadata'); END;`)
	if err != nil {
		return fmt.Errorf("create correction publication guards: %w", err)
	}
	return tx.Commit()
}
