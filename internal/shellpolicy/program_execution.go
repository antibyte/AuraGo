package shellpolicy

import (
	"regexp"
	"strings"
)

var arithmeticEnvironmentAssignment = regexp.MustCompile(`(?i)\b([a-z_][a-z0-9_]*)\s*(?:<<|>>|[+\-*/%&^|])?=`)

// Inspect literal arguments only; executable configuration is a separate interpreter.
func programExecutionReason(command string, escape byte) string {
	var words []string
	check := func() string {
		if len(words) == 0 {
			return ""
		}
		if executableEnvironmentMutation(words, escape) {
			return "executable Git/editor environment"
		}
		head := strings.TrimSuffix(strings.ToLower(words[0][strings.LastIndexAny(words[0], "/\\")+1:]), ".exe")
		args := words[1:]
		switch head {
		case "xargs", "ex", "ed":
			if len(args) == 1 && (args[0] == "--help" || args[0] == "--version") {
				return ""
			}
			return "program-controlled command execution"
		case "vi", "vim", "nvim", "gvim", "view":
			for _, arg := range args {
				if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "--cmd") || strings.HasPrefix(arg, "--remote") ||
					strings.HasPrefix(arg, "-c") || strings.HasPrefix(arg, "-S") || strings.HasPrefix(arg, "-s") ||
					strings.HasPrefix(arg, "-u") || strings.HasPrefix(arg, "-U") || strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.ContainsAny(arg[1:], "cSsuUeE") {
					return "editor command execution"
				}
			}
		case "git":
			var configArgs []string
			for i := 0; i < len(args); i++ {
				arg := args[i]
				if arg == "-c" || arg == "-C" || arg == "--git-dir" || arg == "--work-tree" || arg == "--namespace" {
					i++
					continue
				}
				if strings.HasPrefix(arg, "-") {
					continue
				}
				if arg == "config" {
					configArgs = args[i+1:]
				}
				break
			}
			readConfig := false
			for i := 0; i < len(configArgs); i++ {
				arg := configArgs[i]
				if arg == "--" {
					break
				}
				if arg == "--edit" || arg == "-e" {
					return "Git editor execution"
				}
				if arg == "--file" || arg == "-f" || arg == "--blob" || arg == "--type" || arg == "--default" {
					i++
					continue
				}
				if arg == "--get" || arg == "--get-all" || arg == "--get-regexp" || arg == "--get-urlmatch" || arg == "--list" || arg == "-l" {
					readConfig = true
				}
				if !strings.HasPrefix(arg, "-") {
					readConfig = readConfig || arg == "get" || arg == "list"
					break
				}
			}
			if !readConfig {
				for i, arg := range configArgs {
					if executableGitSetting(strings.ToLower(strings.SplitN(arg, "=", 2)[0])) && (i+1 < len(configArgs) || strings.Contains(arg, "=")) {
						return "executable Git configuration"
					}
				}
			}
			for i, arg := range args {
				value := ""
				if (arg == "-c" || arg == "--config-env") && i+1 < len(args) {
					value = args[i+1]
				} else if strings.HasPrefix(arg, "-c") {
					value = arg[2:]
				}
				if strings.HasPrefix(arg, "--config-env=") {
					value = strings.TrimPrefix(arg, "--config-env=")
				}
				key := strings.ToLower(strings.SplitN(value, "=", 2)[0])
				if executableGitSetting(key) {
					return "executable Git configuration"
				}
			}
		}
		return ""
	}
	for i := 0; i < len(command); {
		c := command[i]
		if strings.ContainsRune(";&|\r\n", rune(c)) {
			if reason := check(); reason != "" {
				return reason
			}
			words = nil
			i++
			continue
		}
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		if c == '#' && escape != '^' {
			for i < len(command) && command[i] != '\n' {
				i++
			}
			continue
		}
		word, end := literalCommandWord(command, i, escape, " \t;&|\r\n")
		i = end
		if len(words) == 0 && commandAssignment.MatchString(word) {
			if executableEnvironment(word) {
				return "executable Git/editor environment"
			}
			continue
		}
		words = append(words, word)
	}
	return check()
}

func executableGitSetting(key string) bool {
	switch key {
	case "core.pager", "core.editor", "sequence.editor", "core.sshcommand", "core.gitproxy", "core.hookspath", "core.fsmonitor", "diff.external", "gpg.program", "include.path", "protocol.ext.allow":
		return true
	}
	return strings.HasPrefix(key, "pager.") || strings.HasPrefix(key, "alias.") ||
		strings.HasPrefix(key, "filter.") || strings.HasPrefix(key, "credential.") ||
		strings.HasPrefix(key, "diff.") && (strings.HasSuffix(key, ".command") || strings.HasSuffix(key, ".textconv")) ||
		strings.HasPrefix(key, "gpg.") && strings.HasSuffix(key, ".program") ||
		strings.HasPrefix(key, "includeif.") && strings.HasSuffix(key, ".path")
}

func executableEnvironment(word string) bool {
	key := strings.ToUpper(strings.SplitN(word, "=", 2)[0])
	switch key {
	case "PAGER", "GIT_PAGER", "EDITOR", "VISUAL", "GIT_EDITOR", "GIT_SEQUENCE_EDITOR", "GIT_SSH_COMMAND", "GIT_PROXY_COMMAND", "GIT_SSH", "GIT_ASKPASS", "SSH_ASKPASS", "GIT_EXEC_PATH":
		return true
	}
	return strings.HasPrefix(key, "GIT_CONFIG_")
}

func executableEnvironmentMutation(words []string, escape byte) bool {
	head := strings.ToLower(words[0])
	args := words[1:]
	if strings.TrimSuffix(strings.TrimLeft(head[strings.LastIndexAny(head, "/\\")+1:], "@"), ".exe") == "setx" {
		for i := 0; i < len(args); i++ {
			if strings.HasPrefix(args[i], "/") {
				switch strings.ToLower(args[i]) {
				case "/s", "/u", "/p":
					i++
				}
				continue
			}
			return i+1 < len(args) && executableEnvironment(args[i])
		}
	}
	switch escape {
	case '\\':
		assignment := strings.Replace(words[0], "+=", "=", 1)
		if commandAssignment.MatchString(assignment) && executableEnvironment(assignment) {
			return true
		}
		switch head {
		case "export", "readonly", "declare", "typeset":
			for _, arg := range args {
				arg = strings.Replace(arg, "+=", "=", 1)
				if commandAssignment.MatchString(arg) && executableEnvironment(arg) {
					return true
				}
			}
		}
	case '^':
		head = strings.TrimLeft(head, "@")

		assignment := strings.TrimSpace(strings.Join(args, " "))
		if strings.HasPrefix(head, "set/") {
			assignment, head = head[3:]+" "+assignment, "set"
		}
		if head == "set" {
			if strings.HasPrefix(strings.ToLower(assignment), "/a") {
				for _, match := range arithmeticEnvironmentAssignment.FindAllStringSubmatch(assignment[2:], -1) {
					if executableEnvironment(match[1]) {
						return true
					}
				}
				return false
			}
			if strings.HasPrefix(strings.ToLower(assignment), "/p") {
				assignment = strings.TrimSpace(assignment[2:])
			}
			key, _, found := strings.Cut(assignment, "=")
			return found && executableEnvironment(strings.TrimSpace(key))
		}
	case '`':
		variable := strings.TrimPrefix(strings.TrimPrefix(head, "${"), "$")
		if key, ok := strings.CutPrefix(variable, "env:"); ok {
			end := strings.IndexAny(key, "}=+-*/%?")
			assigned := false
			if end >= 0 {
				assigned, key = strings.Contains(key[end:], "="), key[:end]
			}
			if len(args) > 0 {
				assigned = assigned || strings.HasPrefix(strings.TrimLeft(args[0], "+-*/%?"), "=")
			}
			return assigned && executableEnvironment(key)
		}
		switch head {
		case "set-item", "new-item", "si", "ni":
			pathSeen := false
			for i := 0; i < len(args); i++ {
				path := strings.ToLower(args[i])
				switch path {
				case "-value", "-filter", "-include", "-exclude", "-credential", "-name", "-itemtype":
					i++
					continue
				case "-path", "-literalpath":
					i++
					if i >= len(args) {
						return false
					}
					path = strings.ToLower(args[i])
				default:
					if strings.HasPrefix(path, "-path:") || strings.HasPrefix(path, "-literalpath:") {
						path = strings.TrimPrefix(strings.TrimPrefix(path, "-literalpath:"), "-path:")
					} else if strings.HasPrefix(path, "-") || pathSeen {
						continue
					}
				}
				pathSeen = true
				if key, ok := strings.CutPrefix(path, "env:"); ok && executableEnvironment(strings.TrimLeft(key, "/\\")) {
					return true
				}
			}
		}
	}
	return false
}
