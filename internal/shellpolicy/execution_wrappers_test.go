package shellpolicy

import "testing"

func TestExecutionWrappersCannotHideBlockedCommands(t *testing.T) {
	if err := ValidateCommand(`time -f '%e seconds' printf hello`); err != nil {
		t.Fatalf("literal GNU time format rejected: %v", err)
	}
	for _, escape := range []byte{'\\', '`', '^'} {
		for _, command := range []string{
			"nohup rm -rf ./classification-only", "nohup -- timeout -s TERM -k 2s 10s command rm -rf ./classification-only",
			"timeout --signal=TERM --kill-after=2 1 rm -rf ./classification-only",
			"timeout -- 1 busybox sh -c true", "timeout unknown rm -rf ./classification-only",
			"timeout 1 $COMMAND", "nohup \"rm\" \"-rf\" ./classification-only",
			"nice -n 10 rm -rf ./classification-only", "nice --adjustment=-5 nohup timeout 1 rm -rf ./classification-only",
			"stdbuf -oL -e 0 nice -- rm -rf ./classification-only", "stdbuf --output=1K command sh -c true",
			"time -p nice -10 stdbuf -i0 rm -rf ./classification-only", "/usr/bin/time --format=elapsed nohup xargs rm",
			"nice -n", "stdbuf --output=unknown rm -rf ./classification-only", "time --unknown printf safe",
			"nice timeout 1 git -c core.gitProxy=classification-only status", "git config core.gitProxy classification-only",
			"git --config-env=core.gitProxy=TEST_PROXY status", "env GIT_PROXY_COMMAND=classification-only git status",
			"git --config-env core.fsmonitor=TEST_MONITOR status", "git -c core.fsmonitor=classification-only status",
			"git config gpg.program classification-only", "git config gpg.ssh.program classification-only",
			"git config diff.fixture.textconv classification-only", "git config include.path classification-only",
			"git config includeIf.gitdir:fixture.path classification-only", "git -c protocol.ext.allow=always fetch",
			"env GIT_ASKPASS=classification-only git fetch", "GIT_EXEC_PATH=classification-only git status",
			"setx GIT_ASKPASS classification-only", "setx.exe /M SSH_ASKPASS classification-only",
			"git config core.pager classification-only", "git config core.pager get", "git config --global core.editor list", "git -C . config set core.editor classification-only", "git config --edit", "vim -ncclassification-only",
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
		if escape == '^' {
			for _, command := range []string{"setx GIT_ASKPASS classification-only", "setx.exe /M SSH_ASKPASS classification-only", "setx GIT_EXEC_PATH classification-only"} {
				if err := ValidateCommandWithEscape(command, escape); err == nil {
					t.Errorf("Cmd admitted %q", command)
				}
			}
		}
		escaped := "no" + string(escape) + "hup r" + string(escape) + "m -rf ./classification-only"
		if err := ValidateCommandWithEscape(escaped, escape); err == nil {
			t.Errorf("escape %q admitted escaped wrapper %q", escape, escaped)
		}
		for _, command := range []string{
			"nohup printf hello", "timeout 1s command printf hello", "timeout --foreground -s TERM 1s printf hello",
			"nohup --help", "timeout --version", "xargs --help", "xargs --version && command -v sudo",
			"nice -n 10 stdbuf -oL time -p printf hello", "nice --adjustment=-5 printf hello", "nice -10 printf hello",
			"time -f elapsed -o result.txt printf hello", "stdbuf --input=0 --output=L printf hello", "git config --get core.gitProxy",
			"git --no-pager log", "git log -- core.pager", "git show config core.pager", "git config --get core.pager", "git -C . config get core.editor", "git config core.pager", "git -c color.ui=never status", "vi classification-only.txt",
			"echo 'xargs rm -rf classification-only'", "echo 'git -c core.pager=classification-only'",
		} {
			if err := ValidateCommandWithEscape(command, escape); err != nil {
				t.Errorf("escape %q rejected %q: %v", escape, command, err)
			}
		}
	}
}
