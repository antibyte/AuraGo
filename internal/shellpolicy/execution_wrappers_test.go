package shellpolicy

import "testing"

func TestExecutionWrappersCannotHideBlockedCommands(t *testing.T) {
	for _, escape := range []byte{'\\', '`', '^'} {
		for _, command := range []string{
			"nohup rm -rf ./classification-only", "nohup -- timeout -s TERM -k 2s 10s command rm -rf ./classification-only",
			"timeout --signal=TERM --kill-after=2 1 rm -rf ./classification-only",
			"timeout -- 1 busybox sh -c true", "timeout unknown rm -rf ./classification-only",
			"timeout 1 $COMMAND", "nohup \"rm\" \"-rf\" ./classification-only",
			"git config core.pager classification-only", "vim -ncclassification-only",
			"xargs rm", "xargs -I{} {}", "command xargs cat", "xargs --help && \"sudo\" id",
			"vi -c ':!id'", "vim --cmd=classification-only", "nvim +classification-only", "ex classification-only",
			"git -c core.pager=classification-only --paginate log", "git -ccore.editor=classification-only commit",
			"git --config-env=core.pager=TEST_PAGER log", "git -c alias.test=!classification-only test",
			"env GIT_PAGER=classification-only git log", "PAGER=classification-only git log",
		} {
			if err := ValidateCommandWithEscape(command, escape); err == nil {
				t.Errorf("escape %q admitted %q", escape, command)
			}
		}
		for _, command := range []string{
			"nohup printf hello", "timeout 1s command printf hello", "timeout --foreground -s TERM 1s printf hello",
			"nohup --help", "timeout --version", "xargs --help", "xargs --version && command -v sudo",
			"git --no-pager log", "git -c color.ui=never status", "vi classification-only.txt",
			"echo 'xargs rm -rf classification-only'", "echo 'git -c core.pager=classification-only'",
		} {
			if err := ValidateCommandWithEscape(command, escape); err != nil {
				t.Errorf("escape %q rejected %q: %v", escape, command, err)
			}
		}
	}
}
