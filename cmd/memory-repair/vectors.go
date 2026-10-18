package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	chromem "github.com/philippgille/chromem-go"
)

// repairVectors reads existing canonical sources without runtime initialization.
// Legacy-only installations retain the existing stopped-agent repair path.
type repairVectors struct {
	legacy *chromem.Collection
	corpus string
}
type repairDocumentReader interface {
	GetByID(context.Context, string) (chromem.Document, error)
}

func (v *repairVectors) GetByID(ctx context.Context, id string) (chromem.Document, error) {
	if v.legacy != nil {
		return v.legacy.GetByID(ctx, id)
	}
	db, err := openSQLite(v.corpus, false)
	if err != nil {
		return chromem.Document{}, err
	}
	defer db.Close()
	var doc chromem.Document
	doc.ID = id
	var metadata string
	var vector []byte
	err = db.QueryRowContext(ctx, `SELECT content,metadata,vector FROM documents WHERE collection='aurago_memories' AND id=?`, id).Scan(&doc.Content, &metadata, &vector)
	if errors.Is(err, sql.ErrNoRows) {
		return doc, fmt.Errorf("document with ID '%v' not found", id)
	}
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal([]byte(metadata), &doc.Metadata); err != nil {
		return doc, err
	}
	err = json.Unmarshal(vector, &doc.Embedding)
	return doc, err
}

func (v *repairVectors) Count() int {
	if v.legacy != nil {
		return v.legacy.Count()
	}
	db, err := openSQLite(v.corpus, false)
	if err != nil {
		return -1
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM documents WHERE collection='aurago_memories'`).Scan(&count); err != nil {
		return -1
	}
	return count
}

func (v *repairVectors) AddDocument(ctx context.Context, doc chromem.Document) error {
	if v.legacy != nil {
		return v.legacy.AddDocument(ctx, doc)
	}
	db, err := openSQLite(v.corpus, true)
	if err != nil {
		return err
	}
	defer db.Close()
	metadata, err := json.Marshal(doc.Metadata)
	if err != nil {
		return err
	}
	vector, err := json.Marshal(doc.Embedding)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO documents(collection,id,content,metadata,vector,dimension,fingerprint,pending) VALUES('aurago_memories',?,?,?,?,?,?,1)
 ON CONFLICT(collection,id) DO UPDATE SET content=excluded.content,metadata=excluded.metadata,vector=excluded.vector,dimension=excluded.dimension,fingerprint=excluded.fingerprint,pending=1`, doc.ID, doc.Content, string(metadata), vector, len(doc.Embedding), doc.Metadata["embedding_fingerprint"])
	return err
}

func (v *repairVectors) Delete(ctx context.Context, where, whereDocument map[string]string, ids ...string) error {
	if v.legacy != nil {
		return v.legacy.Delete(ctx, where, whereDocument, ids...)
	}
	if len(where) > 0 || len(whereDocument) > 0 || len(ids) == 0 {
		return fmt.Errorf("offline corpus deletion requires explicit IDs")
	}
	db, err := openSQLite(v.corpus, true)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE collection='aurago_memories' AND id=?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
