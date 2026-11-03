package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Host archive tools never mount images. They may only write into a private
// temporary directory, which is then copied through the rooted publisher.

type archiveTools struct {
	sevenZip   string
	debugfs    string
	unsquashfs string
	btrfs      string
	erofs      string
	cramfs     string
	virtCopy   string
	bsdtar     string
}

type archiveToolKey struct{}

var errArchiveToolUnsupported = errors.New("archive tool does not support this format")

func withArchiveTools(ctx context.Context, tools archiveTools) context.Context {
	return context.WithValue(ctx, archiveToolKey{}, tools)
}

func toolsFor(ctx context.Context) archiveTools {
	if ctx != nil {
		if tools, ok := ctx.Value(archiveToolKey{}).(archiveTools); ok {
			return tools
		}
	}
	return detectArchiveTools()
}

func detectArchiveTools() archiveTools {
	return archiveTools{
		sevenZip:   findSevenZip(),
		debugfs:    lookTool("debugfs"),
		unsquashfs: lookTool("unsquashfs"),
		btrfs:      lookTool("btrfs"),
		erofs:      lookTool("fsck.erofs"),
		cramfs:     lookTool("fsck.cramfs"),
		virtCopy:   lookTool("virt-copy-out"),
		bsdtar:     detectBsdtar(),
	}
}

func lookTool(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func findSevenZip() string {
	for _, name := range []string{"7zz", "7z", "7za"} {
		if path := lookTool(name); path != "" {
			return path
		}
	}
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if base == "" {
			continue
		}
		candidate := filepath.Join(base, "7-Zip", "7z.exe")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

var (
	bsdtarOnce sync.Once
	bsdtarPath string
)

func detectBsdtar() string {
	bsdtarOnce.Do(func() {
		if path := lookTool("bsdtar"); path != "" {
			bsdtarPath = path
			return
		}
		if runtime.GOOS != "windows" {
			return
		}
		path := lookTool("tar")
		if path != "" && libarchiveTar(path) {
			bsdtarPath = path
		}
	})
	return bsdtarPath
}

func libarchiveTar(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = archiveToolEnv()
	cmd.SysProcAttr = archiveSysProcAttr()
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	text := strings.ToLower(string(out))
	return strings.Contains(text, "bsdtar") || strings.Contains(text, "libarchive")
}

func archiveToolEnv() []string {
	env := []string{"LANG=C", "LC_ALL=C"}
	if path := os.Getenv("PATH"); path != "" {
		env = append(env, "PATH="+path)
	}
	if runtime.GOOS == "windows" {
		for _, key := range []string{"SYSTEMROOT", "WINDIR", "TEMP", "TMP", "PATHEXT"} {
			if value := os.Getenv(key); value != "" {
				env = append(env, key+"="+value)
			}
		}
	}
	return env
}

func hostMissingError(kind archiveKind) error {
	tool := "7-Zip"
	switch kind {
	case archiveExt:
		tool = "7-Zip or debugfs"
	case archiveSquashfs:
		tool = "7-Zip or unsquashfs"
	case archiveBtrfs:
		tool = "7-Zip or btrfs"
	case archiveErofs:
		tool = "7-Zip or fsck.erofs"
	case archiveCramfs:
		tool = "7-Zip or fsck.cramfs"
	case archiveXfs, archiveQcow, archiveVmdk, archiveVhd, archiveVdi, archiveDmg, archiveUdf:
		tool = "7-Zip or virt-copy-out"
	case archiveIso, archiveCab, archiveCpio, archiveXar, archiveArj, archiveLzh, archiveDeb, archiveRpm:
		tool = "7-Zip or bsdtar"
	}
	if kind.filesystem() {
		return fmt.Errorf("%s images need %s on this host", kind, tool)
	}
	return fmt.Errorf("%s archives need %s on this host", kind, tool)
}

type toolAttempt struct {
	name   string
	args   []string
	warnOK bool
}

type fileID struct {
	dev uint64
	ino uint64
}

func hostAttempts(kind archiveKind, tools archiveTools, image, dest string) ([]toolAttempt, error) {
	var attempts []toolAttempt
	if tools.sevenZip != "" {
		attempts = append(attempts,
			toolAttempt{name: tools.sevenZip, args: []string{"x", "-y", "-bd", "-snl", "-o" + dest, image}, warnOK: true},
			toolAttempt{name: tools.sevenZip, args: []string{"x", "-y", "-bd", "-o" + dest, image}, warnOK: true},
		)
	}
	switch kind {
	case archiveExt:
		if tools.debugfs != "" {
			quoted, err := quoteDebugfsPath(dest)
			if err != nil {
				return nil, err
			}
			attempts = append(attempts, toolAttempt{name: tools.debugfs, args: []string{"-R", "rdump / " + quoted, image}})
		}
	case archiveSquashfs:
		if tools.unsquashfs != "" {
			attempts = append(attempts, toolAttempt{name: tools.unsquashfs, args: []string{"-d", dest, "-no-progress", "-f", image}})
		}
	case archiveBtrfs:
		if tools.btrfs != "" {
			attempts = append(attempts, toolAttempt{name: tools.btrfs, args: []string{"restore", image, dest}})
		}
	case archiveErofs:
		if tools.erofs != "" {
			attempts = append(attempts, toolAttempt{name: tools.erofs, args: []string{"--extract=" + dest, image}})
		}
	case archiveCramfs:
		if tools.cramfs != "" {
			attempts = append(attempts, toolAttempt{name: tools.cramfs, args: []string{"-x", dest, image}})
		}
	}
	if kind.filesystem() && tools.virtCopy != "" {
		attempts = append(attempts, toolAttempt{name: tools.virtCopy, args: []string{"-a", image, "/", dest}})
	}
	if kind.bsdtarReadable() && tools.bsdtar != "" {
		attempts = append(attempts, toolAttempt{name: tools.bsdtar, args: []string{"-x", "-f", image, "-C", dest}})
	}
	if len(attempts) == 0 {
		return nil, hostMissingError(kind)
	}
	return attempts, nil
}

func quoteDebugfsPath(path string) (string, error) {
	if strings.ContainsAny(path, "\"\r\n") {
		return "", fmt.Errorf("archive tool path is invalid")
	}
	return `"` + path + `"`, nil
}

func (s *Service) listHost(ctx context.Context, kind archiveKind, image string) (ArchiveListing, error) {
	tools := toolsFor(ctx)
	if tools.sevenZip != "" {
		members, err := listWith7z(ctx, tools.sevenZip, image)
		if err == nil {
			return listingFromMembers(kind, members)
		}
		if errors.Is(err, errArchiveEncrypted) || !errors.Is(err, errArchiveToolUnsupported) {
			return ArchiveListing{}, err
		}
	}
	members, err := materializedMembers(ctx, kind, image, tools, false)
	if err != nil {
		return ArchiveListing{}, err
	}
	return listingFromMembers(kind, members)
}

func (s *Service) extractHost(ctx context.Context, kind archiveKind, image, dst, source string, root *os.Root) error {
	tools := toolsFor(ctx)
	members, err := materializedMembers(ctx, kind, image, tools, true)
	if err != nil {
		return err
	}
	return s.publishMembers(ctx, kind, members, dst, source, root)
}

func materializedMembers(ctx context.Context, kind archiveKind, image string, tools archiveTools, keep bool) ([]archiveMember, error) {
	dir, err := os.MkdirTemp("", "aurago-archive-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	attempts, err := hostAttempts(kind, tools, image, dir)
	if err != nil {
		return nil, err
	}
	var last error
	for i, attempt := range attempts {
		if i > 0 {
			if err := resetDir(dir); err != nil {
				return nil, err
			}
		}
		_, err := runArchiveCommand(ctx, dir, 1<<20, false, attempt.warnOK, attempt.name, attempt.args...)
		if err == nil {
			return membersFromTree(dir, keep)
		}
		last = err
		if errors.Is(err, errArchiveEncrypted) || !errors.Is(err, errArchiveToolUnsupported) {
			return nil, err
		}
	}
	if last == nil || errors.Is(last, errArchiveToolUnsupported) {
		return nil, hostMissingError(kind)
	}
	return nil, last
}

func listWith7z(ctx context.Context, bin, image string) ([]archiveMember, error) {
	argSets := [][]string{
		{"l", "-slt", "-ba", "-sccUTF-8", "-y", image},
		{"l", "-slt", "-ba", "-y", image},
		{"l", "-slt", "-y", image},
	}
	var last error
	for _, args := range argSets {
		out, err := runArchiveCommand(ctx, "", 32<<20, false, true, bin, args...)
		if err == nil {
			return parse7zList(string(out))
		}
		last = err
		if errors.Is(err, errArchiveEncrypted) {
			return nil, err
		}
		if !errors.Is(err, errArchiveToolUnsupported) {
			return nil, err
		}
	}
	if last == nil {
		last = errArchiveToolUnsupported
	}
	return nil, last
}

func (s *Service) readHostPreview(ctx context.Context, kind archiveKind, image, want string, maxBytes int64) (ArchiveEntry, error) {
	tools := toolsFor(ctx)
	if tools.sevenZip != "" {
		if members, listErr := listWith7z(ctx, tools.sevenZip, image); listErr == nil {
			for _, member := range members {
				name, skip, cleanErr := cleanMemberName(member.Name)
				if skip || cleanErr != nil || name != want {
					continue
				}
				if member.IsDir {
					return ArchiveEntry{}, ErrArchiveEntryIsDirectory
				}
				if member.Link || member.Special {
					return ArchiveEntry{}, ErrArchiveEntryNotPreviewable
				}
				break
			}
		} else if errors.Is(listErr, errArchiveEncrypted) || errors.Is(listErr, ErrArchiveTooManyEntries) {
			return ArchiveEntry{}, listErr
		}
		out, err := runArchiveCommand(ctx, "", int(maxBytes)+1, true, true, tools.sevenZip, "x", "-so", "-y", "-bd", "-snl", image, want)
		if err == nil {
			if int64(len(out)) > maxBytes {
				return ArchiveEntry{}, ErrArchiveEntryTooLarge
			}
			return ArchiveEntry{
				Name:     filepath.Base(want),
				Size:     int64(len(out)),
				ModTime:  time.Now().UTC(),
				MIMEType: MIMETypeForName(want),
				Data:     out,
			}, nil
		}
		switch {
		case errors.Is(err, errArchiveEncrypted), errors.Is(err, ErrArchiveEntryTooLarge):
			return ArchiveEntry{}, err
		case errors.Is(err, errArchiveEntryMissing):
			return ArchiveEntry{}, ErrArchiveEntryNotFound
		case !errors.Is(err, errArchiveToolUnsupported):
			return ArchiveEntry{}, err
		}
	}
	members, err := materializedMembers(ctx, kind, image, tools, true)
	if err != nil {
		return ArchiveEntry{}, err
	}
	for _, member := range members {
		name, skip, cleanErr := cleanMemberName(member.Name)
		if skip || cleanErr != nil || name != want {
			continue
		}
		if member.IsDir {
			return ArchiveEntry{}, ErrArchiveEntryIsDirectory
		}
		if member.Link || member.Special || member.Read == nil {
			return ArchiveEntry{}, ErrArchiveEntryNotPreviewable
		}
		if member.Size > maxBytes {
			return ArchiveEntry{}, ErrArchiveEntryTooLarge
		}
		data, err := member.Read(maxBytes)
		if err != nil {
			return ArchiveEntry{}, err
		}
		if int64(len(data)) > maxBytes {
			return ArchiveEntry{}, ErrArchiveEntryTooLarge
		}
		mod := member.ModTime
		if mod.IsZero() {
			mod = time.Now().UTC()
		}
		return ArchiveEntry{
			Name:     filepath.Base(want),
			Size:     int64(len(data)),
			ModTime:  mod,
			MIMEType: MIMETypeForName(want),
			Data:     data,
		}, nil
	}
	return ArchiveEntry{}, ErrArchiveEntryNotFound
}

var errArchiveEntryMissing = errors.New("archive entry not found by tool")

func parse7zList(text string) ([]archiveMember, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var members []archiveMember
	for _, block := range strings.Split(text, "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, " = ")
			if !ok {
				continue
			}
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		path := fields["Path"]
		if path == "" || fields["Type"] != "" {
			continue
		}
		if fields["Encrypted"] == "+" {
			return nil, errArchiveEncrypted
		}
		if len(members) >= ArchiveEntryLimit {
			return nil, ErrArchiveTooManyEntries
		}
		member := archiveMember{
			Name:       strings.ReplaceAll(path, "\\", "/"),
			Size:       parseDecimal(fields["Size"]),
			Compressed: parseDecimal(fields["Packed Size"]),
		}
		if modified := fields["Modified"]; modified != "" {
			if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", modified, time.Local); err == nil {
				member.ModTime = parsed
			}
		}
		attr := fields["Attributes"]
		if fields["Folder"] == "+" || strings.HasPrefix(attr, "D") {
			member.IsDir = true
		}
		if attributeIsLink(attr) {
			member.Link = true
			member.IsDir = false
		} else if attributeIsSpecial(attr) {
			member.Special = true
			member.IsDir = false
		}
		members = append(members, member)
	}
	return members, nil
}

func attributeIsLink(attr string) bool {
	for _, field := range strings.Fields(attr) {
		if strings.HasPrefix(field, "l") && len(field) >= 10 {
			return true
		}
	}
	return strings.HasPrefix(attr, "L")
}

func attributeIsSpecial(attr string) bool {
	for _, field := range strings.Fields(attr) {
		if len(field) >= 10 && strings.ContainsRune("cbps", rune(field[0])) {
			return true
		}
	}
	return false
}

func parseDecimal(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

type treeItem struct {
	rel   string
	path  string
	mode  os.FileMode
	size  int64
	mod   time.Time
	id    fileID
	links uint64
	hasID bool
}

func membersFromTree(root string, keep bool) ([]archiveMember, error) {
	var items []treeItem
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if !isWithinPath(root, path) {
			return fmt.Errorf("archive path escapes destination")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("archive path escapes destination")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if len(items) >= ArchiveEntryLimit {
			return ErrArchiveTooManyEntries
		}
		mode := info.Mode()
		item := treeItem{rel: filepath.ToSlash(rel), path: path, mode: mode, size: info.Size(), mod: info.ModTime()}
		switch {
		case mode&os.ModeSymlink != 0:
			item.size = 0
			items = append(items, item)
			if mode.IsDir() {
				return fs.SkipDir
			}
		case mode.IsDir():
			item.size = 0
			items = append(items, item)
		case mode.IsRegular():
			id, links, idErr := regularFileIdentity(path, info)
			if idErr != nil {
				return fmt.Errorf("invalid archive entry")
			}
			item.id = id
			item.links = links
			item.hasID = true
			items = append(items, item)
		default:
			item.size = 0
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	counts := map[fileID]int{}
	for _, item := range items {
		if item.hasID {
			counts[item.id]++
		}
	}
	members := make([]archiveMember, 0, len(items))
	var used int64
	for _, item := range items {
		member := archiveMember{Name: item.rel, Size: item.size, ModTime: item.mod}
		switch {
		case item.mode&os.ModeSymlink != 0:
			member.Link = true
			member.Size = 0
		case item.mode.IsDir():
			member.IsDir = true
			member.Size = 0
		case item.hasID && item.links > uint64(counts[item.id]):
			member.Link = true
			member.Size = 0
		case item.mode.IsRegular():
			if keep {
				if item.size < 0 || used > archiveExpansionLimit-item.size {
					return nil, fmt.Errorf("archive expanded size exceeds limit")
				}
				file, openErr := openFileNoFollow(item.path, os.O_RDONLY, 0)
				if openErr != nil {
					return nil, openErr
				}
				opened, statErr := file.Stat()
				if statErr != nil || !opened.Mode().IsRegular() {
					file.Close()
					return nil, fmt.Errorf("invalid archive entry")
				}
				data, readErr := readLimited(file, archiveExpansionLimit-used)
				file.Close()
				if readErr != nil {
					return nil, readErr
				}
				payload := data
				member.Size = int64(len(payload))
				used += member.Size
				member.Read = func(int64) ([]byte, error) { return payload, nil }
			}
		default:
			member.Special = true
			member.Size = 0
		}
		members = append(members, member)
	}
	return members, nil
}

func resetDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

type cappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.buf.Len() >= c.limit {
		c.overflow = true
		return len(p), nil
	}
	remain := c.limit - c.buf.Len()
	if len(p) > remain {
		c.overflow = true
		_, _ = c.buf.Write(p[:remain])
		return len(p), nil
	}
	return c.buf.Write(p)
}

func runArchiveCommand(ctx context.Context, dest string, stdoutLimit int, preview, warnOK bool, name string, args ...string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = archiveToolEnv()
	cmd.SysProcAttr = archiveSysProcAttr()
	cmd.Stdin = bytes.NewReader(nil)
	cmd.WaitDelay = 3 * time.Second
	stdout := &cappedBuffer{limit: stdoutLimit}
	stderr := &cappedBuffer{limit: 1 << 20}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	var over atomic.Bool
	stop := func() {}
	if dest != "" {
		stop = watchArchiveDir(ctx, dest, &over, cmd)
	}
	err := cmd.Run()
	stop()
	if over.Load() {
		return nil, fmt.Errorf("archive expanded size exceeds limit")
	}
	if stdout.overflow && preview {
		return nil, ErrArchiveEntryTooLarge
	}
	if stdout.overflow || stderr.overflow {
		return nil, fmt.Errorf("archive tool output exceeds limit")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		classified := classifyToolError(stderr.buf.String(), append([]string{name}, args...))
		var exitErr *exec.ExitError
		if warnOK && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 &&
			!errors.Is(classified, errArchiveEncrypted) &&
			!errors.Is(classified, errArchiveToolUnsupported) &&
			!errors.Is(classified, errArchiveEntryMissing) {
			return stdout.buf.Bytes(), nil
		}
		return nil, classified
	}
	return stdout.buf.Bytes(), nil
}

func classifyToolError(stderr string, args []string) error {
	text := strings.ToLower(stderr)
	switch {
	case strings.Contains(text, "wrong password"),
		strings.Contains(text, "enter password"),
		strings.Contains(text, "encrypted archive"),
		strings.Contains(text, "encrypted file"),
		strings.Contains(text, "is encrypted"):
		return errArchiveEncrypted
	case strings.Contains(text, "no files to process"),
		strings.Contains(text, "cannot find file"),
		strings.Contains(text, "can not find"):
		return errArchiveEntryMissing
	case strings.Contains(text, "cannot open the file as archive"),
		strings.Contains(text, "can not open the file as archive"),
		strings.Contains(text, "is not supported archive"),
		strings.Contains(text, "unsupported method"),
		strings.Contains(text, "unsupported command"),
		strings.Contains(text, "incorrect command line"),
		strings.Contains(text, "unknown switch"),
		strings.Contains(text, "bad switch"):
		return errArchiveToolUnsupported
	}
	message := strings.TrimSpace(stderr)
	for _, arg := range args {
		if arg != "" {
			message = strings.ReplaceAll(message, arg, "archive")
		}
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 300 {
		message = message[:300]
	}
	if message == "" {
		return fmt.Errorf("archive tool failed")
	}
	return fmt.Errorf("archive tool failed: %s", message)
}

func watchArchiveDir(ctx context.Context, dir string, over *atomic.Bool, cmd *exec.Cmd) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if directoryBytes(dir) > archiveExpansionLimit {
					over.Store(true)
					if cmd.Process != nil {
						_ = cmd.Process.Kill()
					}
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

func directoryBytes(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			return nil
		}
		total += info.Size()
		if total > archiveExpansionLimit {
			return fsStopWalk
		}
		return nil
	})
	return total
}

var fsStopWalk = errors.New("stop")
