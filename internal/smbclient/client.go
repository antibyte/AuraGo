// Package smbclient provides a small, read-only SMB2/3 filesystem client for
// explicitly configured Bookshelf sources.
package smbclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	smb2 "github.com/hirochachacha/go-smb2/v2"
	"github.com/hirochachacha/go-smb2/v2/auth"
)

// Config contains one explicit share and named user's credentials. Empty
// credentials are rejected; the client never falls back to guest access.
type Config struct {
	Host     string
	Share    string
	Username string
	Domain   string
	Password string
}

// Entry is read-only metadata for one share-relative file or directory.
type Entry struct {
	Path     string
	Name     string
	Size     int64
	Modified time.Time
	IsDir    bool
	Mode     os.FileMode
}

// Client holds a single authenticated SMB session and mounted share.
type Client struct {
	ctx     context.Context
	session *smb2.Session
	share   *smb2.Share
	close   sync.Once
}

// Dial opens only the configured host and share. SMB1 dialects are not
// offered, and message signing is mandatory. The dial timeout also bounds
// name resolution and TCP setup.
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Share = strings.TrimSpace(cfg.Share)
	cfg.Username = strings.TrimSpace(cfg.Username)
	if err := validateCoordinates(cfg); err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	dialer := &smb2.Dialer{
		Credentials:           auth.NTLMCredential{User: cfg.Username, Password: cfg.Password, Domain: cfg.Domain},
		TransportDialer:       smb2.TCPDialer{Dialer: &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}},
		RequireMessageSigning: true,
		SpecifiedDialects:     []smb2.Dialect{smb2.SMB311, smb2.SMB302, smb2.SMB300, smb2.SMB210, smb2.SMB202},
	}
	session, err := dialer.Dial(dialCtx, cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("connect to configured SMB server: %w", err)
	}
	share, err := session.Mount(dialCtx, cfg.Share)
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("mount configured SMB share: %w", err)
	}
	return &Client{ctx: ctx, session: session, share: share}, nil
}

func validateCoordinates(cfg Config) error {
	if cfg.Host == "" || cfg.Share == "" || cfg.Username == "" || cfg.Password == "" {
		return errors.New("SMB host, share, username, and password are required")
	}
	return ValidateTarget(cfg)
}

// ValidateTarget checks explicit SMB coordinates without requiring a password,
// which allows source configuration validation before looking up its Vault secret.
func ValidateTarget(cfg Config) error {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.Share) == "" || strings.TrimSpace(cfg.Username) == "" {
		return errors.New("SMB host, share, and username are required")
	}
	if strings.ContainsAny(cfg.Host, "/\\\x00\r\n ") || strings.ContainsAny(cfg.Share, "/\\\x00\r\n") || strings.ContainsAny(cfg.Username, "\x00\r\n") || strings.ContainsAny(cfg.Domain, "\x00\r\n") {
		return errors.New("invalid SMB coordinates")
	}
	if strings.Contains(cfg.Host, ":") || strings.HasPrefix(cfg.Host, "[") || strings.HasSuffix(cfg.Host, "]") {
		return errors.New("SMB host must be a hostname or IP address without a port")
	}
	return nil
}

// List returns one directory's immediate children. Symlinks are omitted so
// recursive scans cannot traverse a target outside the selected share path.
func (c *Client) List(ctx context.Context, raw string) ([]Entry, error) {
	rel, err := cleanPath(raw, true)
	if err != nil {
		return nil, err
	}
	if err := c.rejectSymlinkPath(ctx, rel); err != nil {
		return nil, err
	}
	items, err := c.share.ReadDir(ctx, smbPath(rel))
	if err != nil {
		return nil, fmt.Errorf("list configured SMB directory: %w", err)
	}
	if len(items) > 10000 {
		return nil, errors.New("SMB directory exceeds 10000 entries")
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Name()) < strings.ToLower(items[j].Name()) })
	entries := make([]Entry, 0, len(items))
	for _, info := range items {
		name := info.Name()
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		child := path.Join(rel, name)
		entries = append(entries, fromFileInfo(child, info))
	}
	return entries, nil
}

// Stat returns no-follow metadata for a path below the mounted share.
func (c *Client) Stat(ctx context.Context, raw string) (Entry, error) {
	rel, err := cleanPath(raw, false)
	if err != nil {
		return Entry{}, err
	}
	info, err := c.lstatNoFollow(ctx, rel)
	if err != nil {
		return Entry{}, fmt.Errorf("stat configured SMB path: %w", err)
	}
	return fromFileInfo(rel, info), nil
}

// Open opens a regular file for reading and transfers ownership of the file,
// mounted share, and SMB session to the returned reader.
func (c *Client) Open(ctx context.Context, raw string) (io.ReadCloser, error) {
	rel, err := cleanPath(raw, false)
	if err != nil {
		return nil, err
	}
	info, err := c.lstatNoFollow(ctx, rel)
	if err != nil {
		return nil, fmt.Errorf("stat configured SMB file: %w", err)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("SMB path is not a regular file")
	}
	f, err := c.share.Open(ctx, smbPath(rel))
	if err != nil {
		return nil, fmt.Errorf("open configured SMB file: %w", err)
	}
	return &remoteReader{ctx: ctx, file: f, client: c}, nil
}

func (c *Client) lstatNoFollow(ctx context.Context, rel string) (os.FileInfo, error) {
	if err := c.rejectSymlinkPath(ctx, rel); err != nil {
		return nil, err
	}
	info, err := c.share.Lstat(ctx, smbPath(rel))
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("SMB symbolic links are not allowed")
	}
	return info, nil
}

func (c *Client) rejectSymlinkPath(ctx context.Context, rel string) error {
	if rel == "" {
		return nil
	}
	current := ""
	for _, part := range strings.Split(rel, "/") {
		current = path.Join(current, part)
		info, err := c.share.Lstat(ctx, smbPath(current))
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("SMB symbolic links are not allowed")
		}
	}
	return nil
}

func fromFileInfo(name string, info os.FileInfo) Entry {
	return Entry{Path: name, Name: path.Base(name), Size: info.Size(), Modified: info.ModTime(), IsDir: info.IsDir(), Mode: info.Mode()}
}

func cleanPath(raw string, allowEmpty bool) (string, error) {
	if strings.ContainsAny(raw, "\\\x00") || strings.HasPrefix(raw, "/") || strings.Contains(raw, ":") {
		return "", errors.New("invalid share-relative SMB path")
	}
	clean := path.Clean(raw)
	if clean == "." && allowEmpty {
		return "", nil
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("invalid share-relative SMB path")
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("invalid share-relative SMB path")
		}
	}
	return clean, nil
}

func smbPath(relative string) string {
	if relative == "" {
		return "."
	}
	return strings.ReplaceAll(relative, "/", `\`)
}

type remoteReader struct {
	ctx    context.Context
	file   *smb2.File
	client *Client
	closed sync.Once
}

func (r *remoteReader) Read(p []byte) (int, error) { return r.file.Read(r.ctx, p) }

func (r *remoteReader) Close() error {
	var err error
	r.closed.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = r.file.Close(ctx)
		err = r.client.Close(ctx)
	})
	return err
}

// Close disconnects the share and session. It is safe to call more than once.
func (c *Client) Close(ctx context.Context) error {
	if c == nil {
		return nil
	}
	var err error
	c.close.Do(func() {
		if c.share != nil {
			_ = c.share.Unmount(ctx)
		}
		if c.session != nil {
			err = c.session.Close()
		}
	})
	return err
}
