package audit

import (
	"strings"
	"testing"
)

// Distributed AuraGo binaries are GPL-3.0 combined works (the WhatsApp stack links go.mau.fi/libsignal).
// Every release path must therefore check the linked licences before it cleans or builds anything, put
// the licence texts and notices into resources.dat and the Docker image, and link the tagged source.
func TestReleasePackagingCarriesTheGPLNotices(t *testing.T) {
	t.Parallel()
	text := func(path string) string {
		return strings.ReplaceAll(readRepoFile(t, path), "\r\n", "\n")
	}

	builders := []struct {
		path string
		// gate is the licence check line; before lists the first cleanup and build steps it must precede.
		gate    string
		before  []string
		staging []string
	}{
		{
			path: "make_deploy.sh",
			gate: "go test ./internal/audit -run 'TestLinkedGoModulesHaveGPL3CompatibleLicenses' -count=1",
			before: []string{
				"[0/5] Building UI assets",
				`rm -f "$DEPLOY_DIR/$asset"`,
				"tar -czf",
			},
			staging: []string{
				`cp LICENSE THIRD_PARTY_NOTICES.md "$TMPDIR_RES/"`,
				`cp -r licences/texts "$TMPDIR_RES/licences/"`,
				`cp licences/linked-go-modules.tsv "$TMPDIR_RES/licences/"`,
				"./licences/texts/GPL-3.0-only.txt",
			},
		},
		{
			path: "make_release.ps1",
			gate: "go test ./internal/audit -run 'TestLinkedGoModulesHaveGPL3CompatibleLicenses' -count=1",
			before: []string{
				"Get-ChildItem -LiteralPath 'deploy'",
				"tar -czf",
			},
			staging: []string{
				`Copy-Item -LiteralPath 'LICENSE', 'THIRD_PARTY_NOTICES.md' -Destination $tmpStage -Force`,
				`Copy-Item -LiteralPath 'licences\texts' -Destination (Join-Path $tmpStage 'licences') -Recurse -Force`,
				`Copy-Item -LiteralPath 'licences\linked-go-modules.tsv' -Destination (Join-Path $tmpStage 'licences') -Force`,
				"./licences/texts/GPL-3.0-only.txt",
			},
		},
		{
			path: "make_release.bat",
			gate: "go test ./internal/audit -run TestLinkedGoModulesHaveGPL3CompatibleLicenses -count=1",
			before: []string{
				"Get-ChildItem -LiteralPath 'deploy'",
				"tar -czf",
			},
			staging: []string{
				`copy /y "LICENSE" "%TMPSTAGE%\LICENSE"`,
				`copy /y "THIRD_PARTY_NOTICES.md" "%TMPSTAGE%\THIRD_PARTY_NOTICES.md"`,
				`xcopy /e /i /q "licences\texts" "%TMPSTAGE%\licences\texts"`,
				`copy /y "licences\linked-go-modules.tsv" "%TMPSTAGE%\licences\linked-go-modules.tsv"`,
				"./licences/texts/GPL-3.0-only.txt",
			},
		},
	}
	for _, builder := range builders {
		script := text(builder.path)
		gate := strings.Index(script, builder.gate)
		if gate < 0 {
			t.Errorf("%s lacks the linked-module licence gate %q", builder.path, builder.gate)
		}
		for _, step := range builder.before {
			position := strings.Index(script, step)
			if position < 0 {
				t.Errorf("%s no longer contains %q, which the licence gate must precede", builder.path, step)
			} else if gate >= 0 && gate > position {
				t.Errorf("%s runs the licence gate after %q", builder.path, step)
			}
		}
		requireAll(t, builder.path, script, builder.staging)
	}

	// The release notes of the two Windows builders link the tagged source, and retention keeps tags.
	for _, path := range []string{"make_release.ps1", "make_release.bat"} {
		script := text(path)
		requireAll(t, path, script, []string{
			"https://github.com/antibyte/AuraGo/tree/",
			"archive/refs/tags/",
			"GPL-3.0 combined works",
		})
		for number, line := range strings.Split(script, "\n") {
			if strings.Contains(line, "--cleanup-tag") {
				t.Errorf("%s:%d deletes release tags; GPL-3.0 releases link their tag as corresponding source", path, number+1)
			}
		}
	}

	// Installers and updaters copy the notices out of the verified, extracted resources.dat.
	for _, path := range []string{"install.sh", "update.sh"} {
		requireAll(t, path, text(path), []string{`"$TMPEXT/licences"`, "THIRD_PARTY_NOTICES.md"})
	}

	requireAll(t, "Dockerfile", text("Dockerfile"), []string{
		"COPY LICENSE THIRD_PARTY_NOTICES.md /app/\n",
		"COPY licences/texts /app/licences/texts\n",
		"COPY licences/linked-go-modules.tsv /app/licences/linked-go-modules.tsv\n",
	})

	dockerignore := text(".dockerignore")
	requireAll(t, ".dockerignore", dockerignore, []string{"!**/THIRD_PARTY_NOTICES.md", "!**/LICENSE*"})
	for number, line := range strings.Split(dockerignore, "\n") {
		if strings.Contains(line, "licences") {
			t.Errorf(".dockerignore:%d excludes the licences directory the image copies", number+1)
		}
	}

	requireAll(t, ".github/workflows/security-gates.yml", text(".github/workflows/security-gates.yml"), []string{
		"test -s /app/licences/texts/GPL-3.0-only.txt",
	})
}
