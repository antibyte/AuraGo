package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"sync/atomic"

	"aurago/internal/desktop"
	"aurago/internal/storymaker"
)

// storymakerStore implements storymaker.Store (and render.Publisher) over the desktop workspace.
// The desktop service can be replaced when the configuration changes, so the store holds an atomic
// pointer that every request refreshes with the service it just obtained.
type storymakerStore struct {
	svc atomic.Pointer[desktop.Service]
}

// use points the store at the current desktop service.
func (st *storymakerStore) use(svc *desktop.Service) { st.svc.Store(svc) }

func (st *storymakerStore) service() (*desktop.Service, error) {
	svc := st.svc.Load()
	if svc == nil {
		return nil, errors.New("storymaker: the desktop workspace is not available")
	}
	return svc, nil
}

// mapStoreError turns the desktop errors that carry meaning for the project store into its errors.
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storymaker: %w", fs.ErrNotExist)
	}
	var conflict *desktopFileConflict
	if errors.As(err, &conflict) {
		return storymaker.ErrConflict
	}
	var pathConflict *desktop.PathConflict
	if errors.As(err, &pathConflict) {
		return storymaker.ErrConflict
	}
	return err
}

// ReadFile implements storymaker.Store.
func (st *storymakerStore) ReadFile(ctx context.Context, p string) ([]byte, string, error) {
	svc, err := st.service()
	if err != nil {
		return nil, "", err
	}
	data, _, err := svc.ReadFileBytes(ctx, p)
	if err != nil {
		return nil, "", mapStoreError(err)
	}
	return data, storymaker.ETag(data), nil
}

// WriteFile implements storymaker.Store: "" is unconditional, storymaker.MustNotExist creates only,
// anything else is the etag the file must still have. The check runs under the desktop mutation
// lock, in the same step as the write.
func (st *storymakerStore) WriteFile(ctx context.Context, p string, data []byte, ifMatch string) (string, error) {
	svc, err := st.service()
	if err != nil {
		return "", err
	}
	var precondition desktop.FileWritePrecondition
	switch ifMatch {
	case "":
	case storymaker.MustNotExist:
		precondition = func(state desktop.FileWriteState) error {
			if state.Exists {
				return storymaker.ErrConflict
			}
			return nil
		}
	default:
		precondition = func(state desktop.FileWriteState) error {
			if !state.Exists || desktop.NoteVersion(state.Data) != ifMatch {
				return storymaker.ErrConflict
			}
			return nil
		}
	}
	if _, err := svc.WriteFileBytesConditional(ctx, p, data, desktop.SourceUser, precondition); err != nil {
		return "", mapStoreError(err)
	}
	return storymaker.ETag(data), nil
}

// Remove implements storymaker.Store: a missing path is not an error, a folder goes with everything in it.
func (st *storymakerStore) Remove(ctx context.Context, p string) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	if err := svc.DeletePath(ctx, p, desktop.SourceUser); err != nil {
		if mapped := mapStoreError(err); errors.Is(mapped, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// List implements storymaker.Store: the base names of the direct children, sorted; a missing folder is (nil, nil).
func (st *storymakerStore) List(ctx context.Context, dir string) ([]string, error) {
	svc, err := st.service()
	if err != nil {
		return nil, err
	}
	entries, err := svc.ListFiles(ctx, dir)
	if err != nil {
		if mapped := mapStoreError(err); errors.Is(mapped, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names, nil
}

// MkdirAll implements storymaker.Store.
func (st *storymakerStore) MkdirAll(ctx context.Context, dir string) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	return svc.CreateDirectory(ctx, dir, desktop.SourceUser)
}

// Open implements storymaker.Store over OpenPreviewFile, which has no size limit, so audio above
// the 50 MB limit of ReadFile can still be served.
func (st *storymakerStore) Open(ctx context.Context, p string) (io.ReadSeekCloser, storymaker.FileInfo, error) {
	svc, err := st.service()
	if err != nil {
		return nil, storymaker.FileInfo{}, err
	}
	file, entry, mimeType, err := svc.OpenPreviewFile(ctx, p)
	if err != nil {
		return nil, storymaker.FileInfo{}, mapStoreError(err)
	}
	return file, storymaker.FileInfo{Name: entry.Name, Size: entry.Size, ModTime: entry.ModTime, MIME: mimeType}, nil
}

// WriteStream implements render.Publisher: it creates p from r and fails when p exists. The write
// goes through WriteFileStreamConditional, which stages the data and renames it into place.
func (st *storymakerStore) WriteStream(ctx context.Context, p string, r io.Reader, max int64) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	_, err = svc.WriteFileStreamConditional(ctx, p, r, max, desktop.SourceUser, nil)
	return err
}

// WriteLibraryVideo is render.Deps.Library: the create-only copy in Videos/Storymaker.
func (st *storymakerStore) WriteLibraryVideo(ctx context.Context, slug string, r io.Reader, size int64) (string, error) {
	svc, err := st.service()
	if err != nil {
		return "", err
	}
	return svc.WriteStorymakerVideo(ctx, slug, r, size, desktop.SourceUser)
}

// WriteBytes implements render.Publisher: a small file, replaced when it exists.
func (st *storymakerStore) WriteBytes(ctx context.Context, p string, data []byte) error {
	svc, err := st.service()
	if err != nil {
		return err
	}
	return svc.WriteFileBytes(ctx, p, data, desktop.SourceUser)
}
