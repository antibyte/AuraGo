package server

// All traversal stays relative to open, no-follow directory descriptors. A
// symlink exchanged after validation can never redirect an operation outside
// the workspace. Python 3 and GNU grep belong to the managed runtime.
func codeStudioPathCommand(operation, path string, args ...string) []string {
	return append([]string{"python3", "-c", codeStudioPathScript, codeStudioWorkspaceRoot, operation, path}, args...)
}

const codeStudioPathScript = `import base64, errno, fnmatch, json, os, stat, subprocess, sys
root, operation, target = sys.argv[1:4]
directory_flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW

def parts(path):
    relative = os.path.relpath(path, root)
    if relative == '.':
        return []
    result = relative.split('/')
    if any(p in ('', '.', '..') for p in result):
        raise ValueError('invalid workspace path')
    return result

def directory(names, create=False):
    fd = os.open(root, directory_flags)
    try:
        for name in names:
            if create:
                try:
                    os.mkdir(name, 0o700, dir_fd=fd)
                except FileExistsError:
                    pass
            child = os.open(name, directory_flags, dir_fd=fd)
            os.close(fd)
            fd = child
        return fd
    except BaseException:
        os.close(fd)
        raise

def remove(parent, name):
    entry = os.stat(name, dir_fd=parent, follow_symlinks=False)
    if not stat.S_ISDIR(entry.st_mode):
        os.unlink(name, dir_fd=parent)
        return
    fd = os.open(name, directory_flags, dir_fd=parent)
    try:
        opened = os.fstat(fd)
        with os.scandir(fd) as children:
            for child in children:
                remove(fd, child.name)
        current = os.stat(name, dir_fd=parent, follow_symlinks=False)
        if (opened.st_dev, opened.st_ino) != (current.st_dev, current.st_ino):
            raise ValueError('directory changed during deletion')
        os.rmdir(name, dir_fd=parent)
    finally:
        os.close(fd)

def open_file(path):
    pending, resolved, hops = parts(path), [], 0
    fd = directory([])
    try:
        while pending:
            name = pending.pop(0)
            if name in ('', '.'):
                continue
            if name == '..':
                if not resolved:
                    raise ValueError('symlink escapes workspace')
                resolved.pop()
                child = directory(resolved)
                os.close(fd)
                fd = child
                continue
            info = os.stat(name, dir_fd=fd, follow_symlinks=False)
            if stat.S_ISLNK(info.st_mode):
                hops += 1
                if hops > 16:
                    raise ValueError('symlink loop')
                link = os.readlink(name, dir_fd=fd)
                if link.startswith('/'):
                    if not link.startswith(root.rstrip('/') + '/'):
                        raise ValueError('symlink escapes workspace')
                    link = link[len(root.rstrip('/')) + 1:]
                    resolved = []
                    child = directory([])
                    os.close(fd)
                    fd = child
                pending = link.split('/') + pending
                continue
            child = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | (os.O_DIRECTORY if pending else os.O_NONBLOCK), dir_fd=fd)
            os.close(fd)
            fd = child
            resolved.append(name)
        if not stat.S_ISREG(os.fstat(fd).st_mode):
            raise ValueError('not a regular file')
        return os.dup(fd), root.rstrip('/') + '/' + '/'.join(resolved)
    finally:
        os.close(fd)

def search_file(fd, path, options, remaining):
    include, exclude = options['include'], options['exclude']
    if include and not fnmatch.fnmatchcase(os.path.basename(path), include):
        return
    if exclude and fnmatch.fnmatchcase(os.path.basename(path), exclude):
        return
    args = options['args']
    with subprocess.Popen([args[0], '-m', str(remaining[0])] + args[1:], stdin=fd, stdout=subprocess.PIPE, stderr=subprocess.PIPE) as process:
        try:
            while True:
                line = process.stdout.readline(1024 * 1024 + 1)
                if not line:
                    break
                if len(line) > 1024 * 1024:
                    raise ValueError('search result line exceeds 1 MiB')
                remaining[1] -= len(line) + len(os.fsencode(path)) + 1
                if remaining[1] < 0:
                    raise ValueError('search output exceeds 16 MiB')
                sys.stdout.buffer.write(os.fsencode(path) + b'\0' + line.rstrip(b'\n') + b'\n')
                remaining[0] -= 1
            error = process.stderr.read(65536)
            if process.wait() not in (0, 1):
                raise ValueError(error.decode('utf-8', 'replace'))
        finally:
            if process.poll() is None:
                process.kill()

def search(fd, path, options, remaining):
    # Open regular files ourselves. grep reads the descriptor as stdin and
    # never traverses a path; its filename is framed separately with NUL.
    with os.scandir(fd) as children:
        for entry in children:
            if remaining[0] <= 0:
                break
            logical = path.rstrip('/') + '/' + entry.name
            child = None
            try:
                child = os.open(entry.name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
                mode = os.fstat(child).st_mode
                if stat.S_ISDIR(mode):
                    if not options['exclude'] or not fnmatch.fnmatchcase(entry.name, options['exclude']):
                        search(child, logical, options, remaining)
                elif stat.S_ISREG(mode):
                    search_file(child, logical, options, remaining)
            except OSError as error:
                # Entries may disappear or become symlinks while scanning.
                if error.errno not in (errno.ENOENT, errno.ELOOP, errno.ENOTDIR):
                    raise
            finally:
                if child is not None:
                    os.close(child)

try:
    names = parts(target)
    if operation in ('probe', 'read'):
        fd, physical = open_file(target)
        with os.fdopen(fd, 'rb') as source:
            info = os.fstat(source.fileno())
            if operation == 'probe':
                sys.stdout.buffer.write(('%d|%d\n' % (info.st_size, info.st_mtime)).encode() + os.fsencode(physical) + b'\0')
            else:
                maximum = int(sys.argv[4])
                content = source.read(maximum + 1)
                if len(content) > maximum:
                    raise ValueError('file exceeds size limit')
                sys.stdout.buffer.write(base64.b64encode(content))
    elif operation in ('move', 'delete'):
        if not names:
            raise ValueError('cannot change workspace root')
        parent = directory(names[:-1])
        try:
            if operation == 'delete':
                remove(parent, names[-1])
            else:
                destination = parts(sys.argv[4])
                if not destination:
                    raise ValueError('cannot replace workspace root')
                other = directory(destination[:-1], True)
                try:
                    os.rename(names[-1], destination[-1], src_dir_fd=parent, dst_dir_fd=other)
                finally:
                    os.close(other)
        finally:
            os.close(parent)
    elif operation == 'search':
        options = json.loads(sys.argv[4])
        parent = directory(names[:-1])
        try:
            fd = os.open(names[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent) if names else os.dup(parent)
            try:
                mode = os.fstat(fd).st_mode
                remaining = [options['limit'], 16 * 1024 * 1024]
                if stat.S_ISDIR(mode):
                    search(fd, target, options, remaining)
                elif stat.S_ISREG(mode):
                    search_file(fd, target, options, remaining)
                else:
                    raise ValueError('not a regular file or directory')
            finally:
                os.close(fd)
        finally:
            os.close(parent)
    else:
        fd = directory(names, operation == 'mkdir')
        try:
            if operation == 'list':
                with os.scandir(fd) as children:
                    for entry in children:
                        info = entry.stat(follow_symlinks=False)
                        kind = 'd' if stat.S_ISDIR(info.st_mode) else 'f'
                        prefix = '%s|%d|%.9f|' % (kind, info.st_size, info.st_mtime)
                        sys.stdout.buffer.write(prefix.encode() + os.fsencode(target.rstrip('/') + '/' + entry.name) + b'\0')
            elif operation != 'mkdir':
                raise ValueError('unknown workspace operation')
        finally:
            os.close(fd)
except Exception as error:
    print(str(error), file=sys.stderr)
    sys.exit(2)
`
