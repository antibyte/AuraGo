package audit

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleasePowerShellPassesPinnedNativeBuildArguments(t *testing.T) {
	ps, err := exec.LookPath("pwsh")
	if err != nil {
		ps, err = exec.LookPath("powershell")
		if err != nil {
			t.Skip("PowerShell is unavailable")
		}
	}
	dir := t.TempDir()
	stub := filepath.Join(dir, "go")
	if runtime.GOOS == "windows" {
		stub += ".exe"
	}
	stubSource := filepath.Join(dir, "capture.go")
	if err := os.WriteFile(stubSource, []byte(`package main
import ("encoding/json"; "os")
func main() {
 data, err := json.Marshal(os.Args[1:]); if err != nil { panic(err) }
 f, err := os.OpenFile(os.Getenv("AURAGO_FIXTURE_GO_ARGS"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); if err != nil { panic(err) }
 defer f.Close(); if _, err := f.Write(append(data, '\n')); err != nil { panic(err) }
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("go", "build", "-o", stub, stubSource).CombinedOutput(); err != nil {
		t.Fatalf("build native argv fixture: %v %s", err, output)
	}
	source := strings.ReplaceAll(readRepoFile(t, "make_release.ps1"), "\r\n", "\n")
	start := strings.Index(source, "function Build-Binary {")
	if start < 0 {
		t.Fatal("PowerShell build helper missing")
	}
	end := strings.Index(source[start:], "\n# ── Linux amd64")
	if start < 0 || end < 0 {
		t.Fatal("PowerShell build helper missing")
	}
	var webAssetsDeclaration, releaseArguments string
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(line, "$webReleaseAssets = ") {
			webAssetsDeclaration = line
		}
		if strings.HasPrefix(strings.TrimSpace(line), "$ghArgs = ") {
			releaseArguments = strings.TrimSpace(line)
		}
	}
	if webAssetsDeclaration == "" {
		t.Fatal("web release asset list is missing")
	}
	validationStart := strings.Index(source, "$requiredPortableAssets = @(")
	validationEnd := strings.Index(source, `Write-Host "  Generating SHA256SUMS`)
	if validationStart < 0 || validationEnd <= validationStart {
		t.Fatal("required portable asset validation missing")
	}
	script := `$ErrorActionPreference = 'Stop'
$scriptDir = $PSScriptRoot
$Version = 'v-fixture'
$mainLdFlags = '-s -w -X aurago/internal/buildinfo.BuildID=reviewed-source'
$assetLdFlags = '-X aurago/internal/webassets.BuildAssetSetID=reviewed-assets'
$assetArchive = 'fixture.tar.gz'
` + webAssetsDeclaration + `
if ($webReleaseAssets.Count -ne 2 -or $webReleaseAssets[0] -ne 'deploy\fixture.tar.gz' -or $webReleaseAssets[1] -ne 'deploy\web-assets.json') { throw 'Web release assets must be two separate paths.' }
New-Item -ItemType Directory -Path (Join-Path $scriptDir 'deploy') -Force | Out-Null
foreach ($asset in @('resources.dat','install.sh','update.sh','fixture.tar.gz')) {
    [IO.File]::WriteAllBytes((Join-Path $scriptDir ('deploy/' + $asset)), [byte[]]::new(2048))
}
[IO.File]::WriteAllText((Join-Path $scriptDir 'deploy/web-assets.json'), ('{"asset_set_id":"' + ('a' * 64) + '"}'))
` + source[validationStart:validationEnd] + source[start:start+end] + `
Build-Binary -Output 'main.bin' -Target './cmd/aurago/'
Build-Binary -Output 'remote.bin' -Target './cmd/remote/'
Build-Binary -Output 'merger.bin' -Target './cmd/config-merger/'
$buildID = 'reviewed-source'
$assets = @('first.bin', 'two files.bin')
$notesFile = 'fixture notes.md'
Set-Alias gh -Value $env:AURAGO_FIXTURE_GO_EXE
` + releaseArguments + "\ngh @ghArgs\n"
	path := filepath.Join(dir, "fixture.ps1")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	argsFile := filepath.Join(dir, "args.jsonl")
	cmd := exec.Command(ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "AURAGO_FIXTURE_GO_ARGS="+argsFile, "AURAGO_FIXTURE_GO_EXE="+stub)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run native argv fixture: %v %s", err, output)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected three native builds and one publication, got %s", data)
	}
	var releaseArgs []string
	if err := json.Unmarshal([]byte(lines[3]), &releaseArgs); err != nil {
		t.Fatal(err)
	}
	if len(releaseArgs) < 10 || releaseArgs[3] != "first.bin" || releaseArgs[4] != "two files.bin" ||
		!strings.Contains(strings.Join(releaseArgs, "|"), "--target|reviewed-source") ||
		!strings.Contains(strings.Join(releaseArgs, "|"), "--notes-file|fixture notes.md") {
		t.Fatalf("release arguments lost reviewed source, asset boundaries or notes file: %q", releaseArgs)
	}
	for i, expected := range []string{"-X aurago/internal/webassets.BuildAssetSetID=reviewed-assets", "-X main.BuildVersion=v-fixture", "-s -w"} {
		var args []string
		if err := json.Unmarshal([]byte(lines[i]), &args); err != nil {
			t.Fatal(err)
		}
		var ldflags string
		for _, arg := range args {
			if strings.HasPrefix(arg, "-ldflags=") {
				ldflags = arg
			}
		}
		if !strings.Contains(ldflags, expected) || strings.Contains(ldflags, "$binaryFlags") {
			t.Fatalf("build %d lost linker flags: %q", i, args)
		}
	}
	cleanup := strings.Index(source, "Get-ChildItem -LiteralPath 'deploy'")
	for _, required := range []string{"git status --porcelain --untracked-files=normal", "BuildVCSModified=false"} {
		if at := strings.Index(source, required); at < 0 || at > cleanup {
			t.Fatalf("%s must be checked before cleanup", required)
		}
	}
	for _, required := range []string{"go run ./cmd/assetpack", "$checksumFiles += $webReleaseAssets", "$assetPaths += $webReleaseAssets"} {
		if !strings.Contains(source, required) {
			t.Fatalf("missing version-bound UI release contract: %s", required)
		}
	}
}
