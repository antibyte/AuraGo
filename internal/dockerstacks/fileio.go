package dockerstacks

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// File access goes through the Engine's archive API on the parked IO container,
// which works on a container that was never started.
var (
	// errIOContainerGone: the IO container does not exist (the host is not prepared any more).
	errIOContainerGone = errors.New("the Docker Manager IO container is missing")
	// errArchivePathMissing: the path inside the IO container does not exist.
	errArchivePathMissing = errors.New("the path does not exist on the Docker host")
)

func archivePath(p string) string {
	return "containers/" + url.PathEscape(IOContainerName) + "/archive?path=" + url.QueryEscape(p)
}

// archivePathMissing tells the Engine's "Could not find the file ... in
// container ..." 404 from "No such container: ...", which is also a 404. The
// Engine gives no other way to tell them apart.
func archivePathMissing(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "could not find the file")
}

// classifyArchive maps a 404 of the archive API to a sentinel.
func classifyArchive(err error) error {
	if isNotFound(err) {
		if archivePathMissing(err) {
			return errArchivePathMissing
		}
		return errIOContainerGone
	}
	return err
}

// pathExists asks whether a file or folder exists (HEAD, so nothing is
// transferred: a stack's data folder can hold gigabytes).
func (e engine) pathExists(ctx context.Context, p string) (bool, error) {
	resp, err := e.request(ctx, http.MethodHead, archivePath(p), nil, "", false)
	if err == nil {
		_ = resp.Body.Close()
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

// readFile reads one regular file of at most MaxFileBytes. A file that does not
// exist is (nil, false, nil); a larger one is CodeFileTooLarge.
func (e engine) readFile(ctx context.Context, p string) ([]byte, bool, error) {
	resp, err := e.request(ctx, http.MethodGet, archivePath(p), nil, "", false)
	if err != nil {
		classified := classifyArchive(err)
		if errors.Is(classified, errArchivePathMissing) {
			return nil, false, nil
		}
		return nil, false, classified
	}
	defer resp.Body.Close()
	tr := tar.NewReader(resp.Body)
	hdr, err := tr.Next()
	if err == io.EOF {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", p, err)
	}
	if hdr.Typeflag != tar.TypeReg {
		return nil, false, newError(CodeStacksDirInvalid, "%s is not a regular file (a link or a folder)", p)
	}
	if hdr.Size > MaxFileBytes {
		return nil, false, tooLarge(p)
	}
	data, err := io.ReadAll(io.LimitReader(tr, MaxFileBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", p, err)
	}
	if len(data) > MaxFileBytes {
		return nil, false, tooLarge(p)
	}
	return data, true, nil
}

func tooLarge(what string) *Error {
	return newError(CodeFileTooLarge, "%s is larger than %d KiB", what, MaxFileBytes>>10)
}

// putArchive extracts a tar into a folder of the host (PUT .../archive). The
// folder must exist.
func (e engine) putArchive(ctx context.Context, dir string, archive []byte) error {
	resp, err := e.request(ctx, http.MethodPut, archivePath(dir)+"&noOverwriteDirNonDir=true", bytes.NewReader(archive), "application/x-tar", false)
	if err != nil {
		return classifyArchive(err)
	}
	return resp.Body.Close()
}

// tarEntry is one entry of the archives AuraGo sends: owned by root, named
// relative to the folder the archive is extracted in.
type tarEntry struct {
	name    string
	dir     bool
	mode    int64
	content []byte
}

// buildTar writes a USTAR archive of the entries, all owned by uid 0 and gid 0.
func buildTar(entries []tarEntry, modTime time.Time) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		hdr := &tar.Header{Name: entry.name, Mode: entry.mode, ModTime: modTime, Format: tar.FormatUSTAR}
		if entry.dir {
			hdr.Typeflag = tar.TypeDir
			hdr.Name = strings.TrimSuffix(entry.name, "/") + "/"
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = int64(len(entry.content))
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if !entry.dir {
			if _, err := tw.Write(entry.content); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// File modes of what AuraGo writes: the compose file is readable by anyone, the
// .env, which holds secrets, only by root.
const (
	composeFileMode = 0o644
	envFileMode     = 0o600
	stackDirMode    = 0o755
)
