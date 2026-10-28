package security

import (
	"reflect"
	"testing"

	"aurago/internal/config"
)

func routineCheck(operation, command string) GuardianCheck {
	return GuardianCheck{Operation: operation, Parameters: map[string]string{"command": command}, RegexLevel: ThreatNone}
}

// An allowlisted first word is not a read-only command: these calls mutate,
// execute or write, and must reach the judge at the default level.
func TestShouldCheckRejectsMutatingFlagsOnAllowlistedPrefixes(t *testing.T) {
	g := &LLMGuardian{cfg: &config.Config{}}
	g.cfg.LLMGuardian.DefaultLevel = "medium"

	mutating := map[string][]string{
		"execute_shell": {
			"find / -name x -delete",
			"find /home -newer x -fprint /root/.ssh/authorized_keys",
			"find . -name x -fprintf /tmp/out %p",
			"find . -fls /tmp/out",
			"find . -name x -exec unlink {} +",
			"find . -name x -execdir unlink {} +",
			"find . -ok unlink {} ;",
			"find . '-delete'",
			"find . -DELETE",
			"find . -name *",
			"rg --pre sh /tmp/payload x",
			"rg --pre=sh x",
			"rg --pre-glob '*.x' foo",
			"rg --hostname-bin=sh foo",
			"rg foo *",
			"ls>/etc/passwd",
			"ls $(whoami)",
			"ls ${HOME}",
			"cat <secrets",
			"ls\nrm -rf /",
			"lsof -i",
			"FOO=bar ls",
			"docker rm -f web",
			"docker network create x",
			"docker -H tcp://10.0.0.5:2375 ps",
			"docker ps; id",
		},
		"execute_sudo": {
			"ip route add default via 10.0.0.1",
			"ip addr flush dev eth0",
			"ip link set eth0 down",
			"ip -b /tmp/batch route",
			"ip r a default via 10.0.0.1",
			"hostname attacker",
			"hostnamectl set-hostname x",
			"iptables -N newchain",
			"iptables -s 10.0.0.1 -A INPUT -j DROP",
			"iptables -A INPUT -j DROP",
			"iptables -F",
			"iptables -L -Z",
			"iptables -n",
			"ss -K",
			"ss -D /tmp/out",
			"cat /etc/shadow",
			"cat /proc/cpuinfo /etc/shadow",
			"cat /proc/../etc/shadow",
			"cat /proc/self/root/etc/shadow",
			"cat /proc/1/environ",
			"cat /proc/*/status",
			"cat /sys/class/net/eth0/address > /tmp/x",
			"idle --kill", // "id" was matched as a prefix
		},
	}
	for operation, commands := range mutating {
		for _, command := range commands {
			if !g.ShouldCheck(routineCheck(operation, command)) {
				t.Errorf("%s %q took the fast path, want the judge", operation, command)
			}
		}
	}

	readOnly := map[string][]string{
		"execute_shell": {
			"find /tmp -name x",
			"find . -name '*.go' -type f",
			"find . -newer ref -print",
			"rg foo file",
			"rg -n 'a b' dir",
			"ls -la",
			"pwd",
			"cat notes.txt",
			"head -n 5 file",
			"tail -n 20 app.log",
			"wc -l file",
			"stat file",
			"grep -rn \"x y\" dir",
			"docker ps -a",
			"docker logs aurago-homepage-web --tail 50",
			"docker inspect web",
			"docker images",
			"docker network ls",
			"docker ps --format '{{.Names}}'",
		},
		"execute_sudo": {
			"id",
			"whoami",
			"uptime",
			"uname -a",
			"hostname",
			"ip addr",
			"ip addr show",
			"ip link",
			"ip link show dev eth0",
			"ip route show",
			"ip route",
			"ip -4 route get 1.1.1.1",
			"ip -s -s link",
			"iptables -L -n",
			"iptables -nvL",
			"iptables -t nat -L -n -v",
			"iptables -S",
			"ss -tulpn",
			"ss -a state established",
			"netstat -tulpn",
			"cat /proc/cpuinfo",
			"cat -n /sys/class/net/eth0/address",
			"ls /root",
			"stat /etc/hosts",
		},
	}
	for operation, commands := range readOnly {
		for _, command := range commands {
			if g.ShouldCheck(routineCheck(operation, command)) {
				t.Errorf("%s %q goes to the judge, want the read-only fast path", operation, command)
			}
		}
	}
}

func TestSplitRoutineArgv(t *testing.T) {
	cases := []struct {
		command string
		want    []string
		globbed bool
		ok      bool
	}{
		{"ls -la /tmp", []string{"ls", "-la", "/tmp"}, false, true},
		{"  find   .\t-name  x ", []string{"find", ".", "-name", "x"}, false, true},
		{"grep 'a b' \"c d\"", []string{"grep", "a b", "c d"}, false, true},
		{"find . '-de'lete", []string{"find", ".", "-delete"}, false, true},
		{"find . -name *.go", []string{"find", ".", "-name", "*.go"}, true, true},
		{"find . -name '*.go'", []string{"find", ".", "-name", "*.go"}, false, true},
		{"echo 'a;b'", []string{"echo", "a;b"}, false, true},
		{"ls; id", nil, false, false},
		{"ls && id", nil, false, false},
		{"ls | wc", nil, false, false},
		{"ls > out", nil, false, false},
		{"ls >out", nil, false, false},
		{"ls <in", nil, false, false},
		{"ls $HOME", nil, false, false},
		{"ls \"$HOME\"", nil, false, false},
		{"ls `id`", nil, false, false},
		{"ls \"`id`\"", nil, false, false},
		{"ls \\-la", nil, false, false},
		{"ls {a,b}", nil, false, false},
		{"(ls)", nil, false, false},
		{"ls #comment", nil, false, false},
		{"ls a#b", []string{"ls", "a#b"}, false, true},
		{"ls 'unterminated", nil, false, false},
		{"ls\nid", nil, false, false},
		{"ls\x00", nil, false, false},
		{"ls \xff", nil, false, false},
		{"", nil, false, false},
		{"   ", nil, false, false},
	}
	for _, tc := range cases {
		got, globbed, ok := splitRoutineArgv(tc.command)
		if ok != tc.ok || globbed != tc.globbed || (ok && !reflect.DeepEqual(got, tc.want)) {
			t.Errorf("splitRoutineArgv(%q) = %q, %v, %v; want %q, %v, %v", tc.command, got, globbed, ok, tc.want, tc.globbed, tc.ok)
		}
	}
}

// The check is made on the command as the model wrote it, not on the lowercased
// copy the marker filter reads.
func TestRoutineShapesAreCaseSensitiveWhereTheToolIs(t *testing.T) {
	for _, command := range []string{"LS -la", "Find . -name x"} {
		if isLowRiskShellCommand(command) {
			t.Errorf("%q is not a command the tool runs, but took the fast path", command)
		}
	}
	// iptables -N (new chain) and -n (numeric) differ only in case.
	if isLowRiskSudoCommand("iptables -N chain") || !isLowRiskSudoCommand("iptables -L -n") {
		t.Error("iptables -N must be judged, iptables -L -n must not")
	}
}
