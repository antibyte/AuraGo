package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Machine-readable codes of the agent Compose argument and output denials.
const (
	dockerComposeArgumentDeniedCode = "docker_compose_argument_denied"
	dockerComposeOutputDeniedCode   = "docker_compose_output_denied"
)

// dockerComposeDeniedError is a Compose denial that carries a code, so the
// agent can tell it from a Compose failure.
type dockerComposeDeniedError struct {
	code    string
	message string
}

func (e *dockerComposeDeniedError) Error() string { return e.message }

func dockerComposeDenied(code, format string, args ...any) error {
	return &dockerComposeDeniedError{code: code, message: fmt.Sprintf(format, args...)}
}

// dockerComposeErrorJSON renders err like errJSON, plus a code field for the
// coded denials; every other error keeps the plain errJSON envelope.
func dockerComposeErrorJSON(err error) string {
	var denied *dockerComposeDeniedError
	if !errors.As(err, &denied) {
		return errJSON("%v", err)
	}
	payload, _ := json.Marshal(map[string]string{"status": "error", "code": denied.code, "message": denied.message})
	return string(payload)
}

// DockerComposeArgumentsDenial returns the tool error envelope for a Compose
// command whose arguments are never allowed (an unknown or blocked subcommand,
// --host, --environment, ...), or "" when they are acceptable. Callers use it to
// report a bad argument before any other gate that would hide it.
func DockerComposeArgumentsDenial(command string) string {
	if _, err := dockerComposeParts(command); err != nil {
		return dockerComposeErrorJSON(err)
	}
	return ""
}

// DockerComposeOutputDenial returns the coded denial DockerCompose would give
// a `config -o`/`convert -o` target, or "" (also for commands without an
// output flag and for arguments DockerComposeArgumentsDenial reports). Agent
// dispatch calls it before the preflight, which resolves the file.
func DockerComposeOutputDenial(cfg DockerConfig, command string) string {
	parts, err := dockerComposeParts(command)
	if err != nil {
		return ""
	}
	if _, err := planDockerComposeOutput(cfg, parts); err != nil {
		var denied *dockerComposeDeniedError
		if errors.As(err, &denied) {
			return dockerComposeErrorJSON(err)
		}
	}
	return ""
}

// dockerComposeOutputPlan is what one Compose command does with `config -o`.
// Compose never receives the validated workspace path: it writes the rendered
// file into a private staging directory and the file is published afterwards
// through a rooted writer, so no path trick (symlink, hardlink, FIFO, NTFS
// stream) can make Compose itself write outside the workspace.
type dockerComposeOutputPlan struct {
	// args are the command arguments without any -o/--output flag.
	args []string
	// insertAt is where the staged --output flag goes in args: the place of the
	// last output flag, never behind a `--`.
	insertAt int
	// root is the symlink-resolved jail root, rel the target below it and target
	// the absolute validated path. All three are empty when Compose writes no
	// file (no output flag, or an empty --output=, which means stdout).
	root   string
	rel    string
	target string
	// anchorRel is the deepest folder of rel that existed when the target was
	// checked ("." for the root), and anchor its identity; publish refuses
	// when that folder is no longer the same one. nil skips the check.
	anchorRel string
	anchor    os.FileInfo
}

func (p dockerComposeOutputPlan) writesFile() bool { return p.target != "" }

// argsWithOutput returns the arguments with the output flag pointing at staged.
func (p dockerComposeOutputPlan) argsWithOutput(staged string) []string {
	out := make([]string, 0, len(p.args)+1)
	out = append(out, p.args[:p.insertAt]...)
	out = append(out, "--output="+staged)
	return append(out, p.args[p.insertAt:]...)
}

// planDockerComposeOutput confines `config -o/--output` (also `convert`) to the
// agent workspace, the same jail the compose file gets: the process working
// directory when no workspace is configured. Relative targets resolve against
// that root, not AuraGo's working directory. Every output flag is validated;
// Compose keeps the last one, so that is the one planned. Other subcommands
// have no -o flag and are returned unchanged, and so is everything after `--`,
// which Compose reads as service names.
func planDockerComposeOutput(cfg DockerConfig, parts []string) (dockerComposeOutputPlan, error) {
	plan := dockerComposeOutputPlan{args: parts}
	if len(parts) == 0 || (parts[0] != "config" && parts[0] != "convert") {
		return plan, nil
	}
	args := []string{parts[0]}
	for i := 1; i < len(parts); i++ {
		if parts[i] == "--" {
			args = append(args, parts[i:]...)
			break
		}
		value, prefix, isOutput, consumesNext := dockerComposeOutputFlag(parts[i])
		if !isOutput {
			args = append(args, parts[i])
			continue
		}
		if consumesNext {
			if i+1 >= len(parts) {
				return dockerComposeOutputPlan{}, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose argument %q needs a file path", parts[i])
			}
			i++
			value = parts[i]
		}
		if prefix != "" {
			args = append(args, prefix)
		}
		plan.insertAt = len(args)
		plan.root, plan.rel, plan.target = "", "", ""
		plan.anchorRel, plan.anchor = "", nil
		if value == "" {
			continue // `--output=` writes to stdout: nothing to confine
		}
		root, rel, target, err := resolveDockerComposeOutputPath(cfg, value)
		if err != nil {
			return dockerComposeOutputPlan{}, err
		}
		anchorRel, anchor, err := dockerComposeOutputAnchor(root, rel)
		if err != nil {
			return dockerComposeOutputPlan{}, dockerComposeDenied(dockerComposeOutputDeniedCode, "cannot check the folder of compose --output target %q: %v", value, err)
		}
		plan.root, plan.rel, plan.target, plan.anchorRel, plan.anchor = root, rel, target, anchorRel, anchor
	}
	plan.args = args
	return plan, nil
}

// dockerComposeOutputFlag recognises --output, --output=x, -o, -o=x, -ox and
// short clusters such as -qo / -qox / -qo=x, with pflag's rules: the value of
// a short flag is the rest of its cluster, a leading "=" is dropped only when
// something follows it (`-o=` names the file "="), and without a rest the
// value is the next argument. prefix keeps the other short flags of a cluster
// (for -qo it is -q).
func dockerComposeOutputFlag(arg string) (value, prefix string, isOutput, consumesNext bool) {
	switch {
	case arg == "--output":
		return "", "", true, true
	case strings.HasPrefix(arg, "--output="):
		return strings.TrimPrefix(arg, "--output="), "", true, false
	case strings.HasPrefix(arg, "--"), !strings.HasPrefix(arg, "-"), arg == "-":
		return "", "", false, false
	}
	cluster := arg[1:]
	letters := cluster
	if eq := strings.IndexByte(cluster, '='); eq >= 0 {
		letters = cluster[:eq] // what follows an "=" is a flag's value, not more flags
	}
	idx := strings.IndexByte(letters, 'o')
	if idx < 0 {
		return "", "", false, false
	}
	if idx > 0 {
		prefix = "-" + cluster[:idx]
	}
	rest := cluster[idx+1:]
	if len(rest) > 1 && rest[0] == '=' {
		rest = rest[1:]
	}
	if rest == "" {
		return "", prefix, true, true
	}
	return rest, prefix, true, false
}

// resolveDockerComposeOutputPath resolves a `config -o` target inside the
// output root, following symlinks of existing parents, and rejects directories,
// non-regular files and AuraGo's protected files. The root is the configured
// workspace; without one it is the process working directory, as for the
// compose file. It returns the resolved root, the target relative to it and the
// absolute target.
func resolveDockerComposeOutputPath(cfg DockerConfig, value string) (root, rel, target string, err error) {
	deny := func(format string, args ...any) (string, string, string, error) {
		return "", "", "", dockerComposeDenied(dockerComposeOutputDeniedCode, format, args...)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return deny("compose --output needs a file path")
	}
	workspace := strings.TrimSpace(cfg.WorkspaceDir)
	jail := "the configured workspace"
	if workspace == "" {
		workdir, err := os.Getwd()
		if err != nil {
			return "", "", "", fmt.Errorf("determine working directory for compose --output: %w", err)
		}
		workspace = workdir
		jail = "AuraGo's working directory (no workspace is configured)"
	}
	candidate := value
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspace, candidate)
	}
	absTarget, err := filepath.Abs(candidate)
	if err != nil {
		return deny("invalid compose --output target %q: %v", value, err)
	}
	if dockerComposeOutputHasStreamSyntax(absTarget) {
		return deny("compose --output target %q must not contain ':' after the drive (NTFS alternate data streams are not allowed)", value)
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid workspace path: %w", err)
	}
	resolvedTarget, err := secureResolveFinalPath(filepath.Clean(absTarget))
	if err != nil {
		return deny("cannot resolve compose --output target %q: %v", value, err)
	}
	resolvedWorkspace, err := secureResolveFinalPath(filepath.Clean(absWorkspace))
	if err != nil {
		return "", "", "", fmt.Errorf("resolve workspace directory: %w", err)
	}
	cleanTarget := cleanDockerHostPath(resolvedTarget)
	cleanRoot := cleanDockerHostPath(resolvedWorkspace)
	if cleanTarget == cleanRoot || !dockerPathEqualOrWithin(cleanTarget, cleanRoot) || dockerComposeOutputHasStreamSyntax(resolvedTarget) {
		return deny("compose --output target %q must stay within %s", value, jail)
	}
	if info, statErr := os.Stat(resolvedTarget); statErr == nil {
		if info.IsDir() {
			return deny("compose --output target %q is a directory", value)
		}
		if !info.Mode().IsRegular() {
			return deny("compose --output target %q is not a regular file", value)
		}
	}
	if err := requireUnprotectedNotesPath(resolvedTarget, true); err != nil {
		return deny("%v", err)
	}
	if err := requireUnprotectedSystemPath(resolvedTarget, value); err != nil {
		return deny("%v", err)
	}
	root = filepath.Clean(resolvedWorkspace)
	target = filepath.Clean(resolvedTarget)
	rel, err = filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return deny("compose --output target %q must stay within %s", value, jail)
	}
	if !filepath.IsLocal(rel) {
		// Windows reserved names (CON, COM1, NUL) name devices, not files.
		return deny("compose --output target %q is not a valid file name in %s", value, jail)
	}
	return root, rel, target, nil
}

// dockerComposeOutputAnchor returns the deepest existing folder of rel below
// root and its identity, read through an open handle so os.SameFile also works
// on Windows.
func dockerComposeOutputAnchor(root, rel string) (string, os.FileInfo, error) {
	dir := filepath.Dir(rel)
	for {
		info, err := dockerComposeDirIdentity(filepath.Join(root, dir))
		if err == nil {
			return dir, info, nil
		}
		if !errors.Is(err, fs.ErrNotExist) || dir == "." {
			return "", nil, err
		}
		dir = filepath.Dir(dir)
	}
}

func dockerComposeDirIdentity(path string) (os.FileInfo, error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	info, err := handle.Stat()
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", path)
	}
	return info, nil
}

// dockerComposeOutputHasStreamSyntax reports, on Windows, a ':' anywhere after
// the drive or UNC prefix: NTFS alternate data streams and index streams
// (`file::$DATA`, `dir::$INDEX_ALLOCATION\x`) name an existing protected file
// or directory through a spelling no name comparison recognises. Other
// platforms allow ':' in names.
func dockerComposeOutputHasStreamSyntax(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return strings.Contains(strings.TrimPrefix(path, filepath.VolumeName(path)), ":")
}

// publish moves the staged output into the workspace at the validated relative
// path: an atomic replace through an os.Root bound to the jail root (the same
// writer docker cp uses), so an existing hardlinked or symlinked target is
// replaced and never written through, and a symlink swapped in after validation
// cannot leave the root. Missing parent directories below the root are created.
// It reports false, without error, when Compose wrote no file (`-q`, `--services`
// and the other list flags print to stdout instead).
func (p dockerComposeOutputPlan) publish(stagedPath string) (bool, error) {
	info, err := os.Lstat(stagedPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat the rendered file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("the rendered file is not a regular file")
	}
	source, err := os.Open(stagedPath)
	if err != nil {
		return false, fmt.Errorf("open the rendered file: %w", err)
	}
	defer source.Close()
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return false, fmt.Errorf("open the workspace: %w", err)
	}
	defer root.Close()
	dir, rest := root, p.rel
	if p.anchor != nil {
		// The deepest folder that existed at planning must still be the same
		// one: a folder swapped for a symlink to another folder of the root
		// would otherwise receive the checked file name.
		if p.anchorRel != "." {
			sub, err := root.OpenRoot(p.anchorRel)
			if err != nil {
				return false, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: its folder changed after it was checked: %v", p.target, err)
			}
			defer sub.Close()
			dir = sub
			if rest, err = filepath.Rel(p.anchorRel, p.rel); err != nil {
				return false, err
			}
		}
		handle, err := dir.Open(".")
		if err != nil {
			return false, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: its folder changed after it was checked: %v", p.target, err)
		}
		info, statErr := handle.Stat()
		handle.Close()
		if statErr != nil || !os.SameFile(info, p.anchor) {
			return false, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: its folder changed after it was checked, so the rendered file was not saved", p.target)
		}
	}
	// Walk the folders below the checked one: none may be a symlink (a
	// relative one stays inside the root, so os.Root would follow it), missing
	// ones are created one by one, and each is confirmed to be the folder that
	// is then opened.
	if parent := filepath.Dir(rest); parent != "." {
		for _, name := range strings.Split(parent, string(filepath.Separator)) {
			sub, err := dockerComposeOutputChildFolder(dir, name, p.target)
			if err != nil {
				return false, err
			}
			defer sub.Close()
			dir = sub
		}
		rest = filepath.Base(rest)
	}
	existing, err := dir.Lstat(rest)
	switch {
	case err == nil && !existing.Mode().IsRegular():
		return false, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q is not a regular file", p.target)
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return false, fmt.Errorf("compose --output target %q: %w", p.target, err)
	}
	// rest is a plain file name now, so nothing is created on the way.
	if err := writeRootFromReaderAtomic(dir, rest, source, 0o644, true); err != nil {
		return false, err
	}
	return true, nil
}

// dockerComposeOutputChildFolder opens the folder name below dir for
// publishing target, creating it when it is missing. A symlink or a
// non-folder is refused, and the opened folder must be the one Lstat saw.
func dockerComposeOutputChildFolder(dir *os.Root, name, target string) (*os.Root, error) {
	info, err := dir.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		if err := dir.Mkdir(name, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("compose --output target %q: create folder %s: %w", target, name, err)
		}
		info, err = dir.Lstat(name)
	}
	if err != nil {
		return nil, fmt.Errorf("compose --output target %q: %w", target, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return nil, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: the folder %s is a symlink or not a folder, so the rendered file was not saved", target, name)
	}
	sub, err := dir.OpenRoot(name)
	if err != nil {
		return nil, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: its folder %s changed while it was opened: %v", target, name, err)
	}
	handle, err := sub.Open(".")
	if err != nil {
		sub.Close()
		return nil, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: its folder %s changed while it was opened: %v", target, name, err)
	}
	opened, statErr := handle.Stat()
	handle.Close()
	if statErr != nil || !os.SameFile(info, opened) {
		sub.Close()
		return nil, dockerComposeDenied(dockerComposeOutputDeniedCode, "compose --output target %q: its folder %s changed while it was opened, so the rendered file was not saved", target, name)
	}
	return sub, nil
}

// dockerResultWithField adds one string field to a runDockerCLIHelper result
// that is a JSON object; any other result is returned unchanged.
func dockerResultWithField(result, key, value string) string {
	var payload map[string]any
	if json.Unmarshal([]byte(result), &payload) != nil {
		return result
	}
	payload[key] = value
	encoded, err := json.Marshal(payload)
	if err != nil {
		return result
	}
	return string(encoded)
}

// dockerResultOK reports a runDockerCLIHelper result with status "ok".
func dockerResultOK(result string) bool {
	var payload struct {
		Status string `json:"status"`
	}
	return json.Unmarshal([]byte(result), &payload) == nil && payload.Status == "ok"
}
