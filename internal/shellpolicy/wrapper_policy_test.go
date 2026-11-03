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
