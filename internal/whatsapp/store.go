package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/fileutil"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"

	_ "modernc.org/sqlite"
)

// storeDSNQuery enables the foreign keys whatsmeow's Upgrade insists on and
// waits for a concurrent writer instead of failing with SQLITE_BUSY.
const storeDSNQuery = "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"

var errNoDataDir = errors.New("whatsapp device store: directories.data_dir is not set")

// sqliteDeviceStore is one opened <data_dir>/whatsapp/device.db with its first
// and only device. A fresh store holds an unsaved, unlinked device.
type sqliteDeviceStore struct {
	db     *sql.DB
	device *store.Device
	log    waLog.Logger
}

// openDeviceStore creates the private store on first use, migrates whatsmeow's
// schema and loads the device.
func openDeviceStore(ctx context.Context, dataDir string, log waLog.Logger) (*sqliteDeviceStore, error) {
	path := config.WhatsAppDeviceDBPath(dataDir)
	if path == "" {
		return nil, errNoDataDir
	}
	if log == nil {
		log = waLog.Noop
	}
	if err := preparePrivateStoreFile(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", sqliteURI(path, storeDSNQuery))
	if err != nil {
		return nil, fmt.Errorf("open whatsapp device store: %w", err)
	}
	container := sqlstore.NewWithDB(db, "sqlite", log.Sub("Store"))
	if err := container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate whatsapp device store: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("read whatsapp device: %w", err)
	}
	return &sqliteDeviceStore{db: db, device: device, log: log}, nil
}

// preparePrivateStoreFile creates the directory (0700) and the file (0600) and
// restricts both to the owner, so the keys never sit in a readable file.
func preparePrivateStoreFile(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create whatsapp store directory: %w", err)
	}
	if err := fileutil.RestrictToOwner(dir); err != nil {
		return fmt.Errorf("protect whatsapp store directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("create whatsapp device store: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("create whatsapp device store: %w", err)
	}
	if err := fileutil.RestrictToOwner(path); err != nil {
		return fmt.Errorf("protect whatsapp device store: %w", err)
	}
	return nil
}

// sqliteURI builds a file: URI that also works for Windows drive paths and
// names with spaces, like internal/memory's snapshot URIs.
func sqliteURI(path, query string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	value := filepath.ToSlash(abs)
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return (&url.URL{Scheme: "file", Path: value, RawQuery: query}).String()
}

// Linked reports whether the store holds a paired device.
func (s *sqliteDeviceStore) Linked() bool { return s.device != nil && s.device.ID != nil }

// OwnJID is the linked account's phone JID without a device part.
func (s *sqliteDeviceStore) OwnJID() types.JID {
	if !s.Linked() {
		return types.EmptyJID
	}
	return s.device.ID.ToNonAD()
}

// DeleteLocal removes the linked device's keys from the store.
func (s *sqliteDeviceStore) DeleteLocal(ctx context.Context) error {
	if !s.Linked() {
		return nil
	}
	if err := s.device.Delete(ctx); err != nil {
		return fmt.Errorf("delete whatsapp device: %w", err)
	}
	return nil
}

// Close releases the database handle. Callers must Disconnect every client
// obtained from NewClient first: a connected client keeps using the database.
func (s *sqliteDeviceStore) Close() error { return s.db.Close() }
