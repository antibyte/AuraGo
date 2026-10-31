package kaggle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/fileutil"
	"aurago/internal/security"
)

func (s *Service) CreateProject(ctx context.Context, title string) (Project, error) {
	snap := s.current()
	if !snap.Config.Enabled || !snap.Access.Write {
		return Project{}, errors.New("project creation requires enabled Kaggle and filesystem write permission")
	}
	if len(title) > 200 || strings.TrimSpace(title) == "" {
		return Project{}, errors.New("project title must contain 1–200 characters")
	}
	p, err := s.Store.addProject(ctx, title)
	if err != nil {
		return p, err
	}
	err = os.Mkdir(filepath.Join(s.Root, p.ID), 0700)
	return p, err
}
func validFile(name string) bool {
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return false
	}
	return name != "" && name != "." && name != ".." && len(name) < 240 && !strings.ContainsAny(name, "/\\:\x00") && !strings.HasPrefix(name, ".") && strings.TrimSpace(name) == name && !strings.HasSuffix(name, ".")
}
func (s *Service) projectRoot(id string) (*os.Root, error) {
	if !validID(id) {
		return nil, errors.New("invalid project ID")
	}
	root, err := os.OpenRoot(s.Root)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(id)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("invalid project directory")
	}
	return root.OpenRoot(id)
}
func (s *Service) OpenFile(project, name string) (*os.File, error) {
	if !validFile(name) {
		return nil, errors.New("invalid project filename")
	}
	r, err := s.projectRoot(project)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	i, err := r.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() {
		return nil, errors.New("project file must be a regular file")
	}
	return r.Open(name)
}

type File struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func (s *Service) Files(project string) ([]File, error) {
	r, err := s.projectRoot(project)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	d, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	defer d.Close()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	out := []File{}
	for _, e := range entries {
		if !validFile(e.Name()) || e.IsDir() || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		i, err := e.Info()
		if err == nil && i.Mode().IsRegular() {
			out = append(out, File{e.Name(), i.Size(), i.ModTime()})
		}
	}
	return out, nil
}
func (s *Service) storageBytes() (int64, error) {
	var total int64
	err := filepath.WalkDir(s.Root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.Type()&os.ModeSymlink != 0 {
			return errors.New("symlink in Kaggle managed storage")
		}
		if !e.IsDir() {
			i, err := e.Info()
			if err != nil {
				return err
			}
			if !i.Mode().IsRegular() {
				return errors.New("special file in Kaggle storage")
			}
			total += i.Size()
		}
		return nil
	})
	return total, err
}

type checkedReader struct {
	ctx    context.Context
	reader io.Reader
	check  func() error
}

func (r checkedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if err := r.check(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if report, ok := r.ctx.Value(transferProgressKey{}).(func(int)); ok {
		report(n)
	}
	return n, err
}

type transferProgressKey struct{}

// PublicError omits host paths from errors returned through tools or HTTP.
func PublicError(err error) string {
	var path *os.PathError
	if errors.As(err, &path) {
		return "project file operation failed: " + path.Err.Error()
	}
	return err.Error()
}

// WriteFile publishes a regular file only after quota, cancellation and current-grant checks.
// Files are opaque; ZIP archives are never extracted on this path.
func (s *Service) WriteFile(ctx context.Context, project, name string, reader io.Reader, check func() error) (int64, error) {
	return s.WriteFileVersion(ctx, project, name, "", reader, check)
}

var ErrFileConflict = errors.New("project file revision changed")

func (s *Service) WriteFileVersion(ctx context.Context, project, name, expected string, reader io.Reader, check func() error) (int64, error) {
	if !validFile(name) {
		return 0, errors.New("invalid project filename")
	}
	s.filesMu.Lock()
	defer s.filesMu.Unlock()
	if check == nil {
		check = func() error {
			c := s.current()
			if !c.Config.Enabled || !c.Access.Write {
				return errors.New("project writes are disabled")
			}
			return nil
		}
	}
	if err := check(); err != nil {
		return 0, err
	}
	r, err := s.projectRoot(project)
	if err != nil {
		return 0, err
	}
	defer r.Close()
	if expected != "" {
		info, statErr := r.Lstat(name)
		if expected == "*" {
			if !errors.Is(statErr, os.ErrNotExist) {
				return 0, ErrFileConflict
			}
		} else {
			if statErr != nil || !info.Mode().IsRegular() {
				return 0, ErrFileConflict
			}
			f, err := r.Open(name)
			if err != nil {
				return 0, err
			}
			hash := sha256.New()
			_, err = io.Copy(hash, f)
			f.Close()
			if err != nil {
				return 0, err
			}
			if hex.EncodeToString(hash.Sum(nil)) != expected {
				return 0, ErrFileConflict
			}
		}
	}
	if info, e := r.Lstat(name); e == nil && !info.Mode().IsRegular() {
		return 0, errors.New("cannot replace a non-regular file")
	}
	used, err := s.storageBytes()
	if err != nil {
		return 0, err
	}
	c := s.current().Config
	limit := int64(c.MaxTransferMB) << 20
	remaining := (int64(c.MaxStorageMB) << 20) - used
	if remaining < limit {
		limit = remaining
	}
	free, err := fileutil.FreeDiskBytes(s.Root)
	if err != nil {
		return 0, err
	}
	if free-(64<<20) < limit {
		limit = free - (64 << 20)
	}
	if limit <= 0 {
		return 0, errors.New("Kaggle storage is full")
	}
	tmp := ".transfer-" + newID()
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, err
	}
	defer r.Remove(tmp)
	n, copyErr := io.Copy(f, io.LimitReader(checkedReader{ctx, reader, check}, limit+1))
	if copyErr == nil && n > limit {
		copyErr = errors.New("transfer exceeds available storage or configured size limit")
	}
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr != nil {
		return 0, copyErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if err = check(); err != nil {
		return 0, err
	}
	if err = fileutil.RenameRootContext(ctx, r, tmp, name); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Service) download(ctx context.Context, c *Client, op Operation, params map[string]any, req Request, check func() error) (json.RawMessage, error) {
	if req.ProjectID == "" {
		return nil, errors.New("download requires a destination project")
	}
	name := req.File
	if name == "" {
		name = req.ID + ".zip"
	}
	if !validFile(name) {
		return nil, errors.New("invalid destination filename")
	}
	resp, err := c.request(ctx, op, params)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		resp.Body.Close()
		h, err := s.storageFactory(ctx, location, 0)
		if err != nil {
			return nil, err
		}
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
		if err != nil {
			return nil, errors.New("invalid storage request")
		}
		resp, err = h.Do(r)
		if err != nil {
			return nil, errors.New("Kaggle download failed")
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Status: resp.StatusCode}
	}
	if resp.ContentLength > int64(s.current().Config.MaxTransferMB)<<20 {
		return nil, errors.New("download exceeds configured transfer limit")
	}
	n, err := s.WriteFileVersion(ctx, req.ProjectID, name, "*", resp.Body, check)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(map[string]any{"project_id": req.ProjectID, "file": name, "bytes": n})
	return b, nil
}

func (s *Service) upload(ctx context.Context, c *Client, op Operation, params map[string]any, req Request, account string, check func() error) (json.RawMessage, error) {
	if s.secrets == nil {
		return nil, errors.New("Kaggle uploads require the encrypted Vault")
	}
	f, err := s.OpenFile(req.ProjectID, req.File)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > int64(s.current().Config.MaxTransferMB)<<20 {
		return nil, errors.New("upload exceeds configured transfer limit")
	}
	params["fileName"] = req.File
	params["contentLength"] = info.Size()
	params["lastModifiedEpochSeconds"] = info.ModTime().Unix()
	if err = check(); err != nil {
		return nil, err
	}
	b, err := c.Call(ctx, op, params)
	if err != nil {
		return nil, err
	}
	var response struct {
		Token     string `json:"token"`
		CreateURL string `json:"createUrl"`
	}
	if json.Unmarshal(b, &response) != nil || response.Token == "" || response.CreateURL == "" {
		return nil, &APIError{Uncertain: true}
	}
	security.RegisterSensitive(response.Token)
	security.RegisterSensitive(response.CreateURL)
	h, err := s.storageFactory(ctx, response.CreateURL, 0)
	if err != nil {
		return nil, err
	}
	// A storage PUT is never replayed: an interrupted upload remains an unknown receipt.
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	r, err := http.NewRequestWithContext(ctx, http.MethodPut, response.CreateURL, checkedReader{ctx, f, check})
	if err != nil {
		return nil, errors.New("invalid upload URL")
	}
	r.ContentLength = info.Size()
	r.Header.Set("Content-Type", "application/octet-stream")
	resp, err := h.Do(r)
	if err != nil {
		return nil, &APIError{Uncertain: true}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Uncertain: true}
	}
	secretKey := "kaggle_upload_" + req.ID
	if err = s.secrets.WriteSecret(secretKey, response.Token); err != nil {
		return nil, errors.New("store Kaggle upload receipt in Vault failed")
	}
	_, err = s.Store.db.ExecContext(ctx, `INSERT INTO uploads(id,account,token) VALUES(?,?,?)`, req.ID, account, secretKey)
	if err != nil {
		return nil, err
	}
	result, _ := json.Marshal(map[string]any{"upload_ref": "upload:" + req.ID, "bytes": info.Size(), "file": req.File})
	return result, nil
}
func (s *Service) resolveUploads(ctx context.Context, v any, account string) error {
	var resolve func(any) (any, error)
	resolve = func(x any) (any, error) {
		switch a := x.(type) {
		case string:
			if strings.HasPrefix(a, "upload:") {
				var token string
				err := s.Store.db.QueryRowContext(ctx, `SELECT token FROM uploads WHERE id=? AND account=?`, strings.TrimPrefix(a, "upload:"), account).Scan(&token)
				if err != nil {
					return nil, errors.New("upload reference does not belong to this account")
				}
				if s.secrets == nil || !strings.HasPrefix(token, "kaggle_upload_") {
					return nil, errors.New("upload receipt is unavailable in Vault")
				}
				secret, err := s.secrets.ReadSecret(token)
				if err != nil {
					return nil, errors.New("upload receipt is unavailable in Vault")
				}
				security.RegisterSensitive(secret)
				return secret, nil
			}
		case map[string]any:
			for k, v := range a {
				r, err := resolve(v)
				if err != nil {
					return nil, err
				}
				a[k] = r
			}
		case []any:
			for i, v := range a {
				r, err := resolve(v)
				if err != nil {
					return nil, err
				}
				a[i] = r
			}
		}
		return x, nil
	}
	_, err := resolve(v)
	return err
}

func (s *Service) RequireWrite() error {
	snap := s.current()
	if !snap.Config.Enabled || !snap.Access.Write {
		return fmt.Errorf("Kaggle project writes are disabled")
	}
	return nil
}
