package shellpolicy

import "testing"

func TestExecutableEnvironmentAssignments(t *testing.T) {
	for _, tc := range []struct {
		name    string
		escape  byte
		blocked []string
		allowed []string
	}{
		{
			name: "posix", escape: '\\',
			blocked: []string{
				`export GIT_EDITOR=classification-only; git commit --allow-empty`,
				`export 'GIT_PAGER=classification-only'; git log`,
				`export GIT_ED\ITOR=classification-only`,
				`readonly EDITOR=classification-only`,
				`declare -x GIT_SEQUENCE_EDITOR=classification-only`,
				`typeset -- GIT_SSH_COMMAND=classification-only`,
				`command export VISUAL=classification-only`,
				`nohup timeout 1s env LANG=C export GIT_CONFIG_COUNT=1`,
			},
			allowed: []string{
				`export LANG=C; git status`, `export -p`, `export GIT_EDITOR`,
				`readonly NOTE=GIT_EDITOR=literal`, `echo 'export GIT_EDITOR=literal'`,
			},
		},
		{
			name: "cmd", escape: '^',
			blocked: []string{
				`set GIT_EDITOR=classification-only & git commit --allow-empty`,
				`set "GIT_PAGER=classification-only"`, `@set EDITOR=classification-only`,
				`se^t GIT_ED^ITOR=classification-only`, `set /p GIT_EDITOR=Prompt`,
				`set /a GIT_EDITOR=1`, `set /a x=1,GIT_EDITOR=2`, `set /a x=GIT_EDITOR=2`,
			},
			allowed: []string{
				`set LANG=C & git status`, `set GIT_EDITOR`, `set NOTE=GIT_EDITOR=literal`,
				`set /a counter=1`, `echo "set GIT_EDITOR=literal"`,
			},
		},
		{
			name: "powershell", escape: '`',
			blocked: []string{
				`$env:GIT_EDITOR='classification-only'; git commit --allow-empty`,
				`${env:GIT_EDITOR} = 'classification-only'`, `$env:GIT_EDITOR = 'C:/editor.exe'`,
				`$env:GIT_EDITOR += 'classification-only'`, "$env:GIT_ED`ITOR='classification-only'",
				`Set-Item Env:GIT_PAGER classification-only`, `si -Path Env:EDITOR -Value classification-only`,
				`New-Item -Path:Env:GIT_EDITOR -Value classification-only`,
				`Set-Item -LiteralPath Env:\GIT_SEQUENCE_EDITOR -Value classification-only`,
			},
			allowed: []string{
				`$env:LANG='C'; git status`, `$env:GIT_EDITOR`, `$env:GIT_EDITOR -eq 'literal'`,
				`Get-Item Env:GIT_EDITOR`, `Set-Item Env:LANG C`, `Write-Output '$env:GIT_EDITOR=literal'`,
				`Set-Item Env:LANG Env:GIT_EDITOR`, `Set-Item -Value Env:GIT_EDITOR -Path Env:LANG`,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, command := range tc.blocked {
				if err := ValidateCommandWithEscape(command, tc.escape); err == nil {
					t.Errorf("admitted environment mutation %q", command)
				}
			}
			for _, command := range tc.allowed {
				if err := ValidateCommandWithEscape(command, tc.escape); err != nil {
					t.Errorf("rejected benign command %q: %v", command, err)
				}
			}
		})
	}
}
