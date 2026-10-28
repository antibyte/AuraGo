package security

import (
	"strings"
	"unicode/utf8"
)

// The tool guardian skips the judge for a few read-only shell commands (see
// isLowRiskRoutine). A prefix test is not enough for that: `find / -delete`,
// `rg --pre sh x` and `ip route add ...` start with allowed words. So the
// command is split into words without a shell and must match one exact
// read-only shape; anything else goes to the judge.

// maxRoutineCommandBytes bounds what the fast path looks at; longer commands
// are judged.
const maxRoutineCommandBytes = 2048

// splitRoutineArgv splits command into words the way a POSIX shell would, for
// lines that need nothing else. Any shell feature makes it report ok=false:
// command separators, pipes, redirections, groups and braces, expansions
// ($, backticks, backslashes), comments and control characters (a newline
// included). Single quotes are literal; double quotes may not hold an
// expansion. globbed reports an unquoted * ? or [, whose expansion could turn
// file names into options.
func splitRoutineArgv(command string) (words []string, globbed, ok bool) {
	if len(command) > maxRoutineCommandBytes || !utf8.ValidString(command) {
		return nil, false, false
	}
	var word strings.Builder
	inWord := false
	flush := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	for i := 0; i < len(command); i++ {
		c := command[i]
		switch {
		case c == ' ' || c == '\t':
			flush()
		case c == '\'' || c == '"':
			end := strings.IndexByte(command[i+1:], c)
			if end < 0 {
				return nil, false, false
			}
			quoted := command[i+1 : i+1+end]
			if hasRoutineControlByte(quoted) || (c == '"' && strings.ContainsAny(quoted, "$`\\")) {
				return nil, false, false
			}
			word.WriteString(quoted)
			inWord = true
			i += end + 1
		case c < 0x20 || c == 0x7f || strings.IndexByte(";&|<>(){}$`\\", c) >= 0 || (c == '#' && !inWord):
			return nil, false, false
		default:
			if c == '*' || c == '?' || c == '[' {
				globbed = true
			}
			word.WriteByte(c)
			inWord = true
		}
	}
	flush()
	return words, globbed, len(words) > 0
}

func hasRoutineControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < 0x20 && s[i] != '\t') || s[i] == 0x7f {
			return true
		}
	}
	return false
}

// routineShellShape reports whether argv is a read-only execute_shell command.
func routineShellShape(argv []string, globbed bool) bool {
	switch argv[0] {
	case "ls", "pwd", "cat", "head", "tail", "wc", "stat", "grep":
		return true
	case "find":
		return !globbed && routineFindReadOnly(argv[1:])
	case "rg":
		return !globbed && routineRipgrepReadOnly(argv[1:])
	case "docker":
		return routineDockerReadOnly(argv[1:])
	}
	return false
}

// routineSudoShape reports whether argv is a read-only execute_sudo command.
func routineSudoShape(argv []string, globbed bool) bool {
	switch argv[0] {
	case "id", "whoami", "uptime", "uname", "ls", "stat", "netstat":
		return true
	case "hostname":
		// With an argument it sets the host name.
		return len(argv) == 1
	case "cat":
		return !globbed && routineSudoCatReadOnly(argv[1:])
	case "ip":
		return routineIPReadOnly(argv[1:])
	case "iptables":
		return routineIptablesListing(argv[1:])
	case "ss":
		return routineSSReadOnly(argv[1:])
	}
	return false
}

// routineFindReadOnly rejects the primaries that delete, run a command or write
// a file.
func routineFindReadOnly(args []string) bool {
	for _, arg := range args {
		arg = strings.ToLower(arg)
		if arg == "-delete" || strings.HasPrefix(arg, "-exec") || strings.HasPrefix(arg, "-ok") ||
			strings.HasPrefix(arg, "-fprint") || strings.HasPrefix(arg, "-fls") {
			return false
		}
	}
	return true
}

// routineRipgrepReadOnly rejects the flags that run a program: --pre,
// --pre-glob and --hostname-bin.
func routineRipgrepReadOnly(args []string) bool {
	for _, arg := range args {
		arg = strings.ToLower(arg)
		if strings.HasPrefix(arg, "--pre") || strings.HasPrefix(arg, "--hostname") {
			return false
		}
	}
	return true
}

func routineDockerReadOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "ps", "logs", "inspect", "images":
		return true
	case "network":
		return len(args) > 1 && args[1] == "ls"
	}
	return false
}

// routineProcSysUnsafeSegments are /proc entries that reach beyond the process
// information they look like: symlinks to the root or working directory, the
// environment and memory of a process.
var routineProcSysUnsafeSegments = map[string]bool{
	"root": true, "cwd": true, "exe": true, "fd": true, "fdinfo": true, "map_files": true,
	"environ": true, "mem": true, "kcore": true,
}

// routineSudoCatReadOnly accepts options and files below /proc or /sys. Dot-dot
// and the segments above stop a path from leaving them.
func routineSudoCatReadOnly(args []string) bool {
	files := 0
	for _, arg := range args {
		if arg != "-" && strings.HasPrefix(arg, "-") {
			continue
		}
		if !strings.HasPrefix(arg, "/proc/") && !strings.HasPrefix(arg, "/sys/") {
			return false
		}
		for _, segment := range strings.Split(arg, "/") {
			if segment == ".." || routineProcSysUnsafeSegments[segment] {
				return false
			}
		}
		files++
	}
	return files > 0
}

// routineIPGlobalOptions are the iproute2 options that only change the output.
var routineIPGlobalOptions = map[string]bool{
	"-4": true, "-6": true, "-s": true, "-stats": true, "-statistics": true, "-d": true, "-details": true,
	"-j": true, "-json": true, "-p": true, "-pretty": true, "-br": true, "-brief": true, "-o": true,
	"-oneline": true, "-h": true, "-human": true, "-r": true, "-resolve": true,
}

// routineIPReadOnly accepts `ip [output options] route|link|addr`, optionally
// followed by show, list or get. iproute2 abbreviates its words (`ip r a` adds a
// route), so only the full words count.
func routineIPReadOnly(args []string) bool {
	i := 0
	for i < len(args) && routineIPGlobalOptions[args[i]] {
		i++
	}
	if i >= len(args) {
		return false
	}
	switch args[i] {
	case "route", "link", "addr":
	default:
		return false
	}
	i++
	if i == len(args) {
		return true
	}
	switch args[i] {
	case "show", "list", "get":
		return true
	}
	return false
}

var routineIptablesListOptions = map[string]bool{
	"--list": true, "--list-rules": true, "--numeric": true, "--verbose": true, "--line-numbers": true, "--exact": true,
}

// routineIptablesListing accepts listing a table: -L or -S (alone or in a
// cluster such as -nvL) with the counters and name options, and -t <table>.
func routineIptablesListing(args []string) bool {
	listing := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--"):
			if !routineIptablesListOptions[arg] {
				return false
			}
			listing = listing || arg == "--list" || arg == "--list-rules"
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			for j, r := range arg[1:] {
				switch r {
				case 'L', 'S':
					listing = true
				case 'n', 'v', 'x':
				case 't':
					// The table name is the next word; any other letter after t would
					// be a value glued to it.
					if j != len(arg)-2 || i+1 >= len(args) {
						return false
					}
					i++
				default:
					return false
				}
			}
		}
	}
	return listing
}

var routineSSLongOptions = map[string]bool{
	"--all": true, "--listening": true, "--tcp": true, "--udp": true, "--raw": true, "--unix": true,
	"--numeric": true, "--processes": true, "--summary": true, "--extended": true, "--info": true,
	"--memory": true, "--options": true, "--ipv4": true, "--ipv6": true, "--resolve": true,
	"--no-header": true, "--oneline": true,
}

// routineSSReadOnly accepts listing sockets. ss -K closes sockets and -D, -F and
// -N touch files or namespaces; they and every option not listed go to the
// judge. Filter words (state established, dport = :22) are not options.
func routineSSReadOnly(args []string) bool {
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--"):
			if !routineSSLongOptions[arg] {
				return false
			}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			for _, r := range arg[1:] {
				if !strings.ContainsRune("altuwxnpseimor46HO", r) {
					return false
				}
			}
		}
	}
	return true
}
