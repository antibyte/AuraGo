package shellpolicy

import "testing"

func TestValidateCommandChecksLiteralExecutionWrappers(t *testing.T) {
	for _, escape := range []byte{'\\', '`', '^'} {
		for _, command := range []string{
			"command sudo -n id",
			"command -p /usr/bin/sudo -n id",
			"command -- /bin/rm -rf ./classification-only",
			"command command /bin/sh -c true",
			"command -v sudo && command busybox sh -c true",
			"command /bin/busybox sh -c true",
			"env LANG=C command busybox rm -rf ./classification-only",
			"busybox find . -exec echo {} +",
			"find . -exec echo {} +",
			"find . -execdir echo {} +",
			"find . -ok echo {} +",
			"find . -okdir echo {} +",
			`"/path with spaces/busybox.exe" sh -c true`,
			`command "sudo.exe" id`,
			`find . "-exec" echo {} +`,
		} {
			if err := ValidateCommandWithEscape(command, escape); err == nil {
				t.Errorf("escape %q: wrapper command %q was allowed", escape, command)
			}
		}
		for _, command := range []string{
			"command -v sudo", "command -V sh", "command -pv sudo", "command -pV sh", "command -p -v sudo",
			"command command -v sudo", "command -- printf hello", "busybox ls .", "busybox --list",
			"find . -type f -name '*.go' -print", "command busybox find . -type f -print",
			`echo "command sudo id"`, `echo "busybox sh -c true"`,
		} {
			if err := ValidateCommandWithEscape(command, escape); err != nil {
				t.Errorf("escape %q: benign command %q was blocked: %v", escape, command, err)
			}
		}
	}
}

func TestValidateCommandChecksEscapedLiteralWords(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		escape  byte
		command string
		blocked bool
	}{
		{"POSIX command head", '\\', `s\udo id`, true},
		{"POSIX forward-slash path", '\\', `//usr/bin/s\udo id`, true},
		{"POSIX wrapped head", '\\', `command su\do id`, true},
		{"POSIX applet", '\\', `b\usybox s\h -c true`, true},
		{"POSIX find action", '\\', `find . -e\xec echo {} +`, true},
		{"POSIX literal backslash in double quotes", '\\', `"s\udo" id`, false},
		{"POSIX literal backslash in single quotes", '\\', `'s\udo' id`, false},
		{"Windows drive path in generic policy", '\\', `C:\Windows\System32\cmd.exe /c echo classification-only`, true},
		{"Windows UNC path in generic policy", '\\', `\\server\share\cmd.exe /c echo classification-only`, true},
		{"cmd command head", '^', `ru^nas /user:classification-only echo`, true},
		{"cmd interpreter", '^', `c^md /c echo classification-only`, true},
		{"cmd literal caret in quotes", '^', `"ru^nas" id`, false},
		{"cmd quoted executable path", '^', `"C:\Program Files\Example\program.exe" --version`, false},
		{"cmd UNC executable path", '^', `"\\server\share\Example\program.exe" --version`, false},
		{"PowerShell command head", '`', "sud`o id", true},
		{"PowerShell command dash", '`', "Start`-Process classification-only -Verb RunAs", true},
		{"PowerShell quoted command", '`', "& \"sud`o\" id", true},
		{"PowerShell literal backtick in single quotes", '`', "'sud`o' id", false},
		{"PowerShell control escape", '`', "sud`no id", false},
		{"PowerShell control escape in command name", '`', "fo`rmat classification-only", false},
		{"PowerShell formfeed escape", '`', "`format classification-only", false},
		{"Windows PowerShell literal e", '`', "p`erl -e classification-only", true},
		{"PowerShell quoted executable path", '`', `& "C:\Program Files\Example\program.exe" --version`, false},
		{"PowerShell UNC executable path", '`', `& "\\server\share\Example\program.exe" --version`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCommandWithEscape(tt.command, tt.escape)
			if (err != nil) != tt.blocked {
				t.Fatalf("ValidateCommandWithEscape(%q, %q) = %v, blocked=%v", tt.command, tt.escape, err, tt.blocked)
			}
		})
	}
}
