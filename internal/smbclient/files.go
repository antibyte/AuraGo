package smbclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	smb2 "github.com/hirochachacha/go-smb2/v2"
	"github.com/hirochachacha/go-smb2/v2/x/protocol"
)

// statusLogonFailure is NTSTATUS STATUS_LOGON_FAILURE.
const statusLogonFailure uint32 = 0xC000006D

// IsLogonFailure reports whether err is a rejected SMB logon (wrong user or
// password). It checks the protocol status first and falls back to the text.
func IsLogonFailure(err error) bool {
	if err == nil {
		return false
	}
	var respErr *protocol.ResponseError
	if errors.As(err, &respErr) && respErr != nil && respErr.Code == statusLogonFailure {
		return true
	}
	var respVal protocol.ResponseError
	if errors.As(err, &respVal) && respVal.Code == statusLogonFailure {
		return true
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "STATUS_LOGON_FAILURE") || strings.Contains(msg, "C000006D")
}

func parentOf(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

// Mkdir creates one directory below the share root. The parent must exist and
// must not be reached through a symbolic link.
func (c *Client) Mkdir(ctx context.Context, raw string) error {
	rel, err := cleanPath(raw, false)
	if err != nil {
		return err
	}
	if err := c.rejectSymlinkPath(ctx, parentOf(rel)); err != nil {
		return err
	}
	if err := c.share.Mkdir(ctx, smbPath(rel), 0o755); err != nil {
		return fmt.Errorf("create SMB directory: %w", err)
	}
	return nil
}

// Remove deletes a file or an empty directory without following symbolic links.
func (c *Client) Remove(ctx context.Context, raw string) error {
	rel, err := cleanPath(raw, false)
	if err != nil {
		return err
	}
	if _, err := c.lstatNoFollow(ctx, rel); err != nil {
		return fmt.Errorf("stat SMB path: %w", err)
	}
	if err := c.share.Remove(ctx, smbPath(rel)); err != nil {
		return fmt.Errorf("remove SMB path: %w", err)
	}
	return nil
}

// Rename moves a file or directory inside the share. Neither end may be the
// share root or reached through a symbolic link.
func (c *Client) Rename(ctx context.Context, from, to string) error {
	src, err := cleanPath(from, false)
	if err != nil {
		return err
	}
	dst, err := cleanPath(to, false)
	if err != nil {
		return err
	}
	if _, err := c.lstatNoFollow(ctx, src); err != nil {
		return fmt.Errorf("stat SMB path: %w", err)
	}
	if err := c.rejectSymlinkPath(ctx, parentOf(dst)); err != nil {
		return err
	}
	if err := c.share.Rename(ctx, smbPath(src), smbPath(dst)); err != nil {
		return fmt.Errorf("rename SMB path: %w", err)
	}
	return nil
}

// WriteFrom creates or truncates a regular file and fills it from r.
func (c *Client) WriteFrom(ctx context.Context, raw string, r io.Reader) (int64, error) {
	rel, err := cleanPath(raw, false)
	if err != nil {
		return 0, err
	}
	if err := c.rejectSymlinkPath(ctx, parentOf(rel)); err != nil {
		return 0, err
	}
	if info, statErr := c.share.Lstat(ctx, smbPath(rel)); statErr == nil {
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return 0, errors.New("SMB path is not a regular file")
		}
	}
	f, err := c.share.Create(ctx, smbPath(rel))
	if err != nil {
		return 0, fmt.Errorf("create SMB file: %w", err)
	}
	n, copyErr := f.ReadFrom(ctx, r)
	closeErr := f.Close(ctx)
	if copyErr != nil {
		return n, fmt.Errorf("write SMB file: %w", copyErr)
	}
	if closeErr != nil {
		return n, fmt.Errorf("close SMB file: %w", closeErr)
	}
	return n, nil
}

// OpenFile opens a regular file for reading and returns its size. Unlike Open,
// closing the reader closes only the file; the Client stays usable.
func (c *Client) OpenFile(ctx context.Context, raw string) (io.ReadCloser, int64, error) {
	rel, err := cleanPath(raw, false)
	if err != nil {
		return nil, 0, err
	}
	info, err := c.lstatNoFollow(ctx, rel)
	if err != nil {
		return nil, 0, fmt.Errorf("stat SMB file: %w", err)
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, errors.New("SMB path is not a regular file")
	}
	f, err := c.share.Open(ctx, smbPath(rel))
	if err != nil {
		return nil, 0, fmt.Errorf("open SMB file: %w", err)
	}
	return &fileReader{ctx: ctx, file: f}, info.Size(), nil
}

type fileReader struct {
	ctx    context.Context
	file   *smb2.File
	closed sync.Once
}

func (r *fileReader) Read(p []byte) (int, error) { return r.file.Read(r.ctx, p) }

func (r *fileReader) Close() error {
	var err error
	r.closed.Do(func() {
		// The request context may already be cancelled; close with a fresh one.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err = r.file.Close(ctx)
	})
	return err
}
