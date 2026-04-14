package outputcompress

import "strings"

// compressShellOutput analyses the command string and routes to the
// appropriate domain-specific filter.
func compressShellOutput(command, output string) (string, string) {
	sig := commandSignature(command)
	parts := strings.Fields(sig)
	if len(parts) == 0 {
		return compressGeneric(output), "generic"
	}

	bin := parts[0]
	sub := ""
	if len(parts) >= 2 {
		sub = parts[1]
	}

	switch {
	// ─── Composite commands (must come before simple bin matches) ─────
	case bin == "docker" && sub == "compose":
		// commandSignature() truncates to 2 tokens, so extract 3rd from raw command
		composeSub := ""
		rawParts := strings.Fields(command)
		for i, p := range rawParts {
			if p == "compose" && i+1 < len(rawParts) {
				composeSub = rawParts[i+1]
				break
			}
		}
		return compressDockerCompose(composeSub, output)
	case bin == "docker-compose" || bin == "docker_compose":
		return compressDockerCompose(sub, output)

	// ─── V1–V5 routes (unchanged) ────────────────────────────────────
	case bin == "git":
		return compressGit(sub, output)
	case bin == "docker" || bin == "podman":
		return compressContainer(sub, output)
	case bin == "kubectl" || bin == "k3s" || bin == "k9s":
		return compressK8s(sub, output)
	case bin == "go" && sub == "test":
		return compressGoTest(output), "go-test"
	case bin == "python" && (sub == "-m" || sub == "pytest"):
		return compressPytest(output), "pytest"
	case bin == "cargo" && sub == "test":
		return compressCargoTest(output), "cargo-test"
	case bin == "npm" && (sub == "test" || sub == "run"):
		return compressJsTest(output), "npm-test"
	case bin == "npx" && (sub == "vitest" || sub == "jest"):
		return compressJsTest(output), "js-test"
	case bin == "yarn" && (sub == "test" || sub == "jest"):
		return compressJsTest(output), "yarn-test"
	case bin == "pnpm" && (sub == "test" || sub == "run"):
		return compressJsTest(output), "pnpm-test"
	case bin == "eslint" || bin == "tsc" || bin == "ruff" || bin == "golangci-lint" || bin == "flake8" || bin == "pylint":
		return compressLint(output), "lint"
	case bin == "ls" || bin == "dir" || bin == "tree":
		return compressLsTree(output), "ls-tree"
	case bin == "find":
		return compressFind(output), "find"
	case bin == "grep" || bin == "rg" || bin == "ag" || bin == "ack":
		return compressGrep(output), "grep"
	case bin == "curl" || bin == "wget":
		return compressCurl(output), "curl"
	case bin == "systemctl":
		return compressSystemctl(sub, output)
	case bin == "journalctl" || bin == "logcli" || strings.HasSuffix(bin, "log"):
		return compressLogs(output), "logs"
	case bin == "aws":
		return compressAws(sub, output)
	case bin == "ansible" || bin == "ansible-playbook":
		return compressAnsible(output), "ansible"

	// ─── V6: Home-Lab / Infra routes ─────────────────────────────────
	case bin == "helm":
		return compressHelm(sub, output)
	case bin == "terraform" || bin == "tf":
		return compressTerraform(sub, output)
	case bin == "df":
		return compressDiskFree(output), "df"
	case bin == "du":
		return compressDiskUsage(output), "du"
	case bin == "ps":
		return compressProcessList(output), "ps"
	case bin == "ss" || bin == "netstat":
		return compressNetworkConnections(output), "netstat"
	case bin == "ip" && (sub == "addr" || sub == "a" || sub == "address"):
		return compressIpAddr(output), "ip-addr"
	case bin == "ip" && (sub == "route" || sub == "r"):
		return compressIpRoute(output), "ip-route"
	case bin == "ip":
		return compressGeneric(output), "ip-generic"
	case bin == "free":
		return compressGeneric(output), "free"
	case bin == "uptime":
		return compressGeneric(output), "uptime"

	// ─── V7: File / Log viewing routes ───────────────────────────────
	case bin == "cat" || bin == "less" || bin == "more":
		return compressCatFile(output), "cat"
	case bin == "tail":
		return compressTailHead(output), "tail"
	case bin == "head":
		return compressTailHead(output), "head"
	case bin == "stat":
		return compressStat(output), "stat"
	case bin == "file":
		return compressGeneric(output), "file"
	case bin == "wc":
		return compressGeneric(output), "wc"

	// ─── V8: Network diagnostics routes ──────────────────────────────
	case bin == "ping" || bin == "ping6":
		return compressPing(output), "ping"
	case bin == "dig":
		return compressDig(output), "dig"
	case bin == "nslookup":
		return compressDNS(output), "nslookup"
	case bin == "host":
		return compressDNS(output), "host"

	// ─── V9A: Archive / Backup routes ───────────────────────────────
	case bin == "tar":
		return compressTar(output), "tar"
	case bin == "zip":
		return compressZip(output), "zip"
	case bin == "unzip":
		return compressZip(output), "unzip"
	case bin == "rsync":
		return compressRsync(output), "rsync"

	// ─── V9B: Text pipeline routes ─────────────────────────────────
	case bin == "sort":
		return compressSort(output), "sort"
	case bin == "uniq":
		return compressUniq(output), "uniq"
	case bin == "cut":
		return compressCut(output), "cut"
	case bin == "sed":
		return compressSed(output), "sed"
	case bin == "awk" || bin == "gawk" || bin == "mawk":
		return compressAwk(output), "awk"
	case bin == "xargs":
		return compressXargs(output), "xargs"
	case bin == "jq":
		return compressJq(output), "jq"
	case bin == "tr":
		return compressTr(output), "tr"
	case bin == "column":
		return compressColumn(output), "column"
	case bin == "diff":
		return compressDiff(output), "diff"
	case bin == "comm":
		return compressComm(output), "comm"
	case bin == "paste":
		return compressPaste(output), "paste"

	default:
		return compressGeneric(output), "generic"
	}
}
