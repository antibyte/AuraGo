package server

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"aurago/internal/tools"
)

const codeStudioFileExists = 73

// File bytes travel through Docker's archive API, never exec arguments. Only a
// generated regular staging file is extracted into /tmp, outside user projects.
func (a codeStudioDockerAdapter) PutArchive(ctx context.Context, containerID string, data []byte) error {
	endpoint := "/containers/" + url.PathEscape(containerID) + "/archive?path=%2Ftmp&copyUIDGID=true&noOverwriteDirNonDir=true"
	body, status, err := tools.DockerRequestBytesContext(ctx, a.cfg, http.MethodPut, endpoint, data, "application/x-tar")
	if err != nil {
		return fmt.Errorf("transfer code studio file: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("transfer code studio file: HTTP %d: %s", status, strings.TrimSpace(string(body)))
	}
	return nil
}

func (h codeStudioHandlers) writeContainerFile(ctx context.Context, containerID, path string, content []byte, createOnly bool) (codeStudioExecResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	name := ".aurago-code-write-" + rand.Text()
	var payload bytes.Buffer
	tw := tar.NewWriter(&payload)
	if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Size: int64(len(content)), Mode: 0600, Uid: 1000, Gid: 1000}); err != nil {
		return codeStudioExecResult{}, fmt.Errorf("prepare code studio file: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return codeStudioExecResult{}, fmt.Errorf("encode code studio file: %w", err)
	}
	if err := tw.Close(); err != nil {
		return codeStudioExecResult{}, fmt.Errorf("close code studio archive: %w", err)
	}
	stage := "/tmp/" + name
	defer func() {
		cleanup, cancelCleanup := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCleanup()
		_, _ = h.docker.Exec(cleanup, containerID, []string{"rm", "-f", "--", stage}, 5*time.Second)
	}()
	if err := h.docker.PutArchive(ctx, containerID, payload.Bytes()); err != nil {
		return codeStudioExecResult{}, err
	}
	return h.docker.Exec(ctx, containerID, []string{"python3", "-c", codeStudioInstallFileScript,
		codeStudioWorkspaceRoot, path, stage, strconv.FormatBool(createOnly), fmt.Sprintf("%x", sha256.Sum256(content)), strconv.Itoa(len(content))}, 30*time.Second)
}

// Open parent directories relative to no-follow descriptors, then publish a
// complete sibling file. link is exclusive; replace is atomic on that filesystem.
// Python is part of the managed Code Studio runtime; execution uses its normal user.
const codeStudioInstallFileScript = `import hashlib, os, secrets, stat, sys
root, target, source, create_only, digest, size = sys.argv[1:]
parent = None
temporary = None
try:
    parts = os.path.relpath(target, root).split('/')
    if any(p in ('', '.', '..') for p in parts):
        raise ValueError('invalid workspace file path')
    parent = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    for part in parts[:-1]:
        try:
            os.mkdir(part, 0o700, dir_fd=parent)
        except FileExistsError:
            pass
        child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
        os.close(parent)
        parent = child
    name = parts[-1]
    mode = 0o644
    try:
        existing = os.stat(name, dir_fd=parent, follow_symlinks=False)
    except FileNotFoundError:
        existing = None
    if existing is not None:
        if create_only == 'true':
            raise FileExistsError('file already exists')
        if not stat.S_ISREG(existing.st_mode):
            raise ValueError('target must be a regular file, not a symlink or directory')
        probe = os.open(name, os.O_WRONLY | os.O_NOFOLLOW, dir_fd=parent)
        os.close(probe)
        mode = stat.S_IMODE(existing.st_mode) & 0o777
    fd = os.open(source, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as staged:
        if not stat.S_ISREG(os.fstat(staged.fileno()).st_mode):
            raise ValueError('invalid staged file')
        content = staged.read(int(size) + 1)
    if len(content) != int(size) or hashlib.sha256(content).hexdigest() != digest:
        raise ValueError('incomplete file transfer')
    temporary = '.aurago-save-' + secrets.token_hex(16)
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=parent)
    with os.fdopen(fd, 'wb') as output:
        output.write(content)
        output.flush()
        os.fchmod(output.fileno(), mode)
        os.fsync(output.fileno())
    if create_only == 'true':
        os.link(temporary, name, src_dir_fd=parent, dst_dir_fd=parent, follow_symlinks=False)
    else:
        os.replace(temporary, name, src_dir_fd=parent, dst_dir_fd=parent)
        temporary = None
except FileExistsError:
    print('file already exists', file=sys.stderr)
    sys.exit(73)
except Exception as error:
    print(str(error), file=sys.stderr)
    sys.exit(1)
finally:
    if temporary is not None and parent is not None:
        try:
            os.unlink(temporary, dir_fd=parent)
        except FileNotFoundError:
            pass
    if parent is not None:
        os.close(parent)
`
