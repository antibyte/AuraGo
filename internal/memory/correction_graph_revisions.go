package memory

import "fmt"

func (kg *KnowledgeGraph) initCorrectionFieldRevisions() error {
	var exists, populated int
	if err := kg.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='kg_correction_field_revisions'`).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return nil
	}
	if err := kg.db.QueryRow(`SELECT COUNT(*) FROM kg_nodes`).Scan(&populated); err != nil {
		return err
	}
	if populated > 0 {
		if _, err := backupMainDatabase(kg.db, "memory-correction-revisions-v1"); err != nil {
			return err
		}
	}
	tx, err := kg.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Field revisions reject ABA changes without invalidating independent properties.
	_, err = tx.Exec(`CREATE TABLE kg_correction_field_revisions (
		node_id TEXT NOT NULL, field TEXT NOT NULL, revision INTEGER NOT NULL,
		PRIMARY KEY(node_id,field));
	CREATE TRIGGER kg_correction_revision_insert AFTER INSERT ON kg_nodes BEGIN
		INSERT INTO kg_correction_field_revisions VALUES(NEW.id,'',1)
		ON CONFLICT(node_id,field) DO UPDATE SET revision=revision+1;
	END;
	CREATE TRIGGER kg_correction_revision_delete AFTER DELETE ON kg_nodes BEGIN
		INSERT INTO kg_correction_field_revisions VALUES(OLD.id,'',1)
		ON CONFLICT(node_id,field) DO UPDATE SET revision=revision+1;
	END;
	CREATE TRIGGER kg_correction_revision_update AFTER UPDATE OF label,properties,protected ON kg_nodes BEGIN
		INSERT INTO kg_correction_field_revisions
		SELECT NEW.id,'',1 WHERE OLD.protected IS NOT NEW.protected
			OR json_extract(OLD.properties,'$.source') IS NOT json_extract(NEW.properties,'$.source')
			OR json_extract(OLD.properties,'$.type') IS NOT json_extract(NEW.properties,'$.type')
		ON CONFLICT(node_id,field) DO UPDATE SET revision=revision+1;
		INSERT INTO kg_correction_field_revisions
		SELECT NEW.id,'label',1 WHERE OLD.label IS NOT NEW.label
		ON CONFLICT(node_id,field) DO UPDATE SET revision=revision+1;
		INSERT INTO kg_correction_field_revisions
		SELECT NEW.id,k.key,1
		FROM (SELECT key FROM json_each(OLD.properties) UNION SELECT key FROM json_each(NEW.properties)) k
		LEFT JOIN json_each(OLD.properties) o ON o.key=k.key
		LEFT JOIN json_each(NEW.properties) n ON n.key=k.key
		WHERE o.value IS NOT n.value OR o.type IS NOT n.type
		ON CONFLICT(node_id,field) DO UPDATE SET revision=revision+1;
	END;`)
	if err != nil {
		return fmt.Errorf("create correction field revisions: %w", err)
	}
	return tx.Commit()
}
