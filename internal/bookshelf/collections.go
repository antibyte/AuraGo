package bookshelf

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) ListCollections(ctx context.Context) ([]Collection, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.name,c.position,c.revision,c.created_at,COUNT(bc.book_id)
		FROM collections c LEFT JOIN book_collections bc ON bc.collection_id=c.id GROUP BY c.id ORDER BY c.position,c.name COLLATE NOCASE,c.id`)
	if err != nil {
		return nil, fmt.Errorf("list bookshelf collections: %w", err)
	}
	defer rows.Close()
	var collections []Collection
	for rows.Next() {
		var collection Collection
		var created string
		if err := rows.Scan(&collection.ID, &collection.Name, &collection.Position, &collection.Revision, &created, &collection.BookCount); err != nil {
			return nil, err
		}
		collection.CreatedAt = parseTime(created)
		collections = append(collections, collection)
	}
	return collections, rows.Err()
}

func (s *Service) CreateCollection(ctx context.Context, name string, position int) (Collection, error) {
	if err := s.checkOpen(); err != nil {
		return Collection{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || !validateText(name, 200) || position < 0 {
		return Collection{}, fmt.Errorf("invalid bookshelf collection")
	}
	id, err := newID()
	if err != nil {
		return Collection{}, err
	}
	created := nowText()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO collections(id,name,position,created_at) VALUES(?,?,?,?)`, id, name, position, created); err != nil {
		return Collection{}, fmt.Errorf("create bookshelf collection: %w", err)
	}
	return Collection{ID: id, Name: name, Position: position, Revision: 1, CreatedAt: parseTime(created)}, nil
}

func (s *Service) UpdateCollection(ctx context.Context, id string, expectedRevision int64, name string, position int) (Collection, error) {
	if err := s.checkOpen(); err != nil {
		return Collection{}, err
	}
	name = strings.TrimSpace(name)
	if !validID(id) || name == "" || !validateText(name, 200) || position < 0 {
		return Collection{}, fmt.Errorf("invalid bookshelf collection")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE collections SET name=?,position=?,revision=revision+1 WHERE id=? AND revision=?`, name, position, id, expectedRevision)
	if err != nil {
		return Collection{}, fmt.Errorf("update bookshelf collection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Collection{}, checkCollectionRevision(ctx, s.db, id)
	}
	return s.GetCollection(ctx, id)
}

func (s *Service) GetCollection(ctx context.Context, id string) (Collection, error) {
	if err := s.checkOpen(); err != nil {
		return Collection{}, err
	}
	var collection Collection
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT c.id,c.name,c.position,c.revision,c.created_at,COUNT(bc.book_id)
		FROM collections c LEFT JOIN book_collections bc ON bc.collection_id=c.id WHERE c.id=? GROUP BY c.id`, id).
		Scan(&collection.ID, &collection.Name, &collection.Position, &collection.Revision, &created, &collection.BookCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Collection{}, ErrNotFound
	}
	if err != nil {
		return Collection{}, fmt.Errorf("get bookshelf collection: %w", err)
	}
	collection.CreatedAt = parseTime(created)
	return collection, nil
}

// CollectionBookIDs returns membership in display order, for revisioned edits
// that need to preserve the existing list before changing it.
func (s *Service) CollectionBookIDs(ctx context.Context, id string) ([]string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, ErrNotFound
	}
	if _, err := s.GetCollection(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT book_id FROM book_collections WHERE collection_id=? ORDER BY position,book_id`, id)
	if err != nil {
		return nil, fmt.Errorf("list bookshelf collection members: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var bookID string
		if err := rows.Scan(&bookID); err != nil {
			return nil, err
		}
		ids = append(ids, bookID)
	}
	return ids, rows.Err()
}

func (s *Service) DeleteCollection(ctx context.Context, id string, expectedRevision int64) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `DELETE FROM collections WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return fmt.Errorf("delete bookshelf collection: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return checkCollectionRevision(ctx, tx, id)
	}
	return tx.Commit()
}

// SetCollectionBooks replaces membership and order atomically after a revision check.
func (s *Service) SetCollectionBooks(ctx context.Context, id string, expectedRevision int64, bookIDs []string) (Collection, error) {
	if err := s.checkOpen(); err != nil {
		return Collection{}, err
	}
	if !validID(id) || len(bookIDs) > s.maxFiles {
		return Collection{}, ErrLimitExceeded
	}
	seen := make(map[string]struct{}, len(bookIDs))
	for _, bookID := range bookIDs {
		if !validID(bookID) {
			return Collection{}, fmt.Errorf("invalid bookshelf book id")
		}
		if _, ok := seen[bookID]; ok {
			return Collection{}, fmt.Errorf("duplicate bookshelf book id")
		}
		seen[bookID] = struct{}{}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Collection{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE collections SET revision=revision+1 WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return Collection{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Collection{}, checkCollectionRevision(ctx, tx, id)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM book_collections WHERE collection_id=?`, id); err != nil {
		return Collection{}, err
	}
	for position, bookID := range bookIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO book_collections(book_id,collection_id,position) VALUES(?,?,?)`, bookID, id, position); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Collection{}, ErrNotFound
			}
			var exists int
			checkErr := tx.QueryRowContext(ctx, `SELECT 1 FROM books WHERE id=?`, bookID).Scan(&exists)
			if errors.Is(checkErr, sql.ErrNoRows) {
				return Collection{}, ErrNotFound
			}
			return Collection{}, fmt.Errorf("set bookshelf collection books: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Collection{}, err
	}
	return s.GetCollection(ctx, id)
}

func checkCollectionRevision(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) error {
	var revision int64
	err := db.QueryRowContext(ctx, `SELECT revision FROM collections WHERE id=?`, id).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrConflict
}
