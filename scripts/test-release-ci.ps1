# Offline dispatch fixture: no Git push, workflow execution or signing credentials.
$ErrorActionPreference = 'Stop'
$global:AuraGoReleaseCIFixture = @{
    Calls = [Collections.Generic.List[string]]::new()
    Revision = 'c' * 40
    Branch = 'main'
    PushExit = 0
    WatchExit = 0
    DispatchURL = $true
    RunRevision = 'c' * 40
}
$fixture = $global:AuraGoReleaseCIFixture
function git {
    $fixture = $global:AuraGoReleaseCIFixture
    $fixture.Calls.Add('git ' + ($args -join ' '))
    $global:LASTEXITCODE = 0
    switch ($args[0]) {
        'branch' { return $fixture.Branch }
        'rev-parse' { return $fixture.Revision }
        'remote' { return 'https://github.com/fixture/AuraGo.git' }
        'status' { return ' M local-draft.md' }
        'push' { $global:LASTEXITCODE = $fixture.PushExit; return }
        default { throw 'Unexpected Git operation in release dispatch.' }
    }
}
function gh {
    $fixture = $global:AuraGoReleaseCIFixture
    $fixture.Calls.Add('gh ' + ($args -join ' '))
    $global:LASTEXITCODE = 0
    if ($args[0] -eq 'workflow' -and $args[1] -eq 'run') {
        if ($fixture.DispatchURL) { return 'https://github.com/fixture/AuraGo/actions/runs/123' }
        return 'Created workflow_dispatch event.'
    }
    if ($args[0] -eq 'run' -and $args[1] -eq 'list') {
        return @(@{ databaseId = 999; displayTitle = 'Release v-other'; createdAt = [DateTimeOffset]::UtcNow.ToString('o') }, @{ databaseId = 123; displayTitle = 'Release v2099.01.01'; createdAt = [DateTimeOffset]::UtcNow.ToString('o') }) | ConvertTo-Json -Compress
    }
    if ($args[0] -eq 'run' -and $args[1] -eq 'view') { return $fixture.RunRevision }
    if ($args[0] -eq 'run' -and $args[1] -eq 'watch') { $global:LASTEXITCODE = $fixture.WatchExit; return }
    throw 'Unexpected GitHub operation in release dispatch.'
}
function Assert-Rejected([scriptblock]$Action) {
    $rejected = $false
    try { & $Action } catch { $rejected = $true }
    if (-not $rejected) { throw 'Unsafe release dispatch was accepted.' }
}
$dispatcher = Join-Path $PSScriptRoot 'start-release-ci.ps1'
& $dispatcher -Version v2099.01.01
if (-not ($fixture.Calls -contains 'git push origin HEAD:refs/heads/main')) { throw 'Reviewed source was not pushed.' }
if (-not ($fixture.Calls -contains ('gh workflow run release.yml --repo fixture/AuraGo --ref main -f version=v2099.01.01 -f expected_sha=' + $fixture.Revision))) { throw 'Dispatch lost the version, repository or reviewed source.' }
if (-not ($fixture.Calls -contains 'gh run watch 123 --repo fixture/AuraGo --exit-status --interval 10')) { throw 'Workflow failure status was not awaited.' }
$fixture.DispatchURL = $false
& $dispatcher -Version v2099.01.01
if (-not ($fixture.Calls | Where-Object { $_ -like 'gh run list*' })) { throw 'Dispatch without a run URL did not resolve the matching run.' }
$fixture.DispatchURL = $true

$fixture.Calls.Clear()
Assert-Rejected { & $dispatcher -Version 'v2099.01.01;publish' }
if ($fixture.Calls.Count) { throw 'Invalid version reached external commands.' }
$fixture.Branch = 'unreviewed'
Assert-Rejected { & $dispatcher -Version v2099.01.01 }
if ($fixture.Calls | Where-Object { $_ -like 'git push*' -or $_ -like 'gh *' }) { throw 'Unreviewed branch reached publication.' }
$fixture.Branch = 'main'
$fixture.Calls.Clear()
$fixture.PushExit = 1
Assert-Rejected { & $dispatcher -Version v2099.01.01 }
if ($fixture.Calls | Where-Object { $_ -like 'gh *' }) { throw 'Failed source publication started a workflow.' }
$fixture.PushExit = 0
$fixture.Calls.Clear()
$fixture.RunRevision = 'd' * 40
Assert-Rejected { & $dispatcher -Version v2099.01.01 }
if ($fixture.Calls | Where-Object { $_ -like 'gh run watch*' }) { throw 'A different source revision was accepted.' }
$fixture.RunRevision = $fixture.Revision
$fixture.WatchExit = 1
Assert-Rejected { & $dispatcher -Version v2099.01.01 }
$global:LASTEXITCODE = 0
Remove-Variable -Name AuraGoReleaseCIFixture -Scope Global
Write-Host 'Release dispatch fixture passed: pinned source, safe version, no dirty-file upload and failure propagation.'
