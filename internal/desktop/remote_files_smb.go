package desktop

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"aurago/internal/smbclient"
)

// smbMaxListEntries is the desktop's directory listing cap (the library default
// of 10000 is too small for system folders).
const smbMaxListEntries = 50000

// smbFS adapts smbclient to remoteFS. One request uses one session.
type smbFS struct {
	client *smbclient.Client
	share  string
}

func openSMBFS(ctx context.Context, t fileTarget) (*smbFS, error) {
	client, err := smbclient.Dial(ctx, smbclient.Config{
		Host:           t.Host,
		Share:          t.Share,
		Username:       t.Username,
		Domain:         t.Domain,
		Password:       string(t.Secret),
		Port:           t.Port,
		MaxListEntries: smbMaxListEntries,
	})
	if err != nil {
		return nil, err
	}
	return &smbFS{client: client, share: t.Share}, nil
}

func smbIsAuthFailure(err error) bool { return smbclient.IsLogonFailure(err) }

func (f *smbFS) Backend() string   { return "smb" }
func (f *smbFS) RootLabel() string { return f.share }

func smbEntry(e smbclient.Entry) sftpEntry {
	return sftpEntry{
		Name:    e.Name,
		Size:    e.Size,
		Mode:    e.Mode.String(),
		ModTime: e.Modified.Format(time.RFC3339),
		IsDir:   e.IsDir,
	}
}

func (f *smbFS) ReadDir(ctx context.Context, p string) ([]sftpEntry, error) {
	entries, err := f.client.List(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]sftpEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, smbEntry(e))
	}
	return out, nil
}

func (f *smbFS) Stat(ctx context.Context, p string) (sftpEntry, error) {
	if p == "." {
		return sftpEntry{Name: f.share, Mode: (os.ModeDir | 0o755).String(), ModTime: time.Time{}.Format(time.RFC3339), IsDir: true}, nil
	}
	e, err := f.client.Stat(ctx, p)
	if err != nil {
		return sftpEntry{}, err
	}
	return smbEntry(e), nil
}

func (f *smbFS) Mkdir(ctx context.Context, p string) error { return f.client.Mkdir(ctx, p) }

func (f *smbFS) Remove(ctx context.Context, p string, _ bool) error { return f.client.Remove(ctx, p) }

func (f *smbFS) Rename(ctx context.Context, from, to string) error {
	return f.client.Rename(ctx, from, to)
}

// Replace renames over an existing file (SMB rename is created with ReplaceIfExists).
func (f *smbFS) Replace(ctx context.Context, from, to string) error {
	return f.client.Rename(ctx, from, to)
}

func (f *smbFS) Open(ctx context.Context, p string) (io.ReadCloser, int64, error) {
	return f.client.OpenFile(ctx, p)
}

// Create returns a writer that streams into the SMB file through a pipe; Close
// waits for the transfer and reports its error.
func (f *smbFS) Create(ctx context.Context, p string) (io.WriteCloser, error) {
	pr, pw := io.Pipe()
	w := &smbPipeWriter{pw: pw, done: make(chan error, 1)}
	go func() {
		_, err := f.client.WriteFrom(ctx, p, pr)
		_ = pr.CloseWithError(errSMBWriterClosed(err))
		w.done <- err
	}()
	return w, nil
}

func errSMBWriterClosed(err error) error {
	if err != nil {
		return err
	}
	return io.ErrClosedPipe
}

type smbPipeWriter struct {
	pw     *io.PipeWriter
	done   chan error
	once   sync.Once
	result error
}

func (w *smbPipeWriter) Write(b []byte) (int, error) { return w.pw.Write(b) }

// Abort fails the transfer: the SMB writer sees err instead of a clean EOF.
func (w *smbPipeWriter) Abort(err error) {
	w.once.Do(func() {
		if err == nil {
			err = io.ErrClosedPipe
		}
		_ = w.pw.CloseWithError(err)
		<-w.done
		w.result = fmt.Errorf("write SMB file: %w", err)
	})
}

func (w *smbPipeWriter) Close() error {
	w.once.Do(func() {
		_ = w.pw.Close()
		if err := <-w.done; err != nil {
			w.result = fmt.Errorf("write SMB file: %w", err)
		}
	})
	return w.result
}

func (f *smbFS) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return f.client.Close(ctx)
}
