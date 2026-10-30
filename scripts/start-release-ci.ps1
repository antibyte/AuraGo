# Dispatch only committed source; signing credentials stay in GitHub Actions.
param([string]$Version)
$ErrorActionPreference = 'Stop'
$null = Get-Command git -ErrorAction Stop
$null = Get-Command gh -ErrorAction Stop
if (-not $Version) {
    $Version = 'v' + (Get-Date -Format 'yyyy.MM.dd')
    $answer = Read-Host "Release tag [$Version]"
    if ($answer) { $Version = $answer }
}
if ($Version -notmatch '^v[0-9][0-9A-Za-z._-]*$') { throw 'Release version must start with v and a digit and contain only letters, digits, dots, underscores or hyphens.' }

Push-Location -LiteralPath (Join-Path $PSScriptRoot '..')
try {
    $branch = (& git branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne 'main') { throw 'Start an official release from the reviewed main branch.' }
    $revision = (& git rev-parse --verify HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $revision -notmatch '^[0-9a-f]{40}$') { throw 'Cannot resolve the committed release source.' }
    $origin = (& git remote get-url origin).Trim()
    if ($LASTEXITCODE -ne 0 -or $origin -notmatch '^(?:https://github\.com/|git@github\.com:)([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:\.git)?$') { throw 'The origin remote must identify a GitHub repository.' }
    $repository = $Matches[1]
    if (& git status --porcelain --untracked-files=normal) {
        Write-Host 'Only committed source will be released; local drafts stay in this checkout.'
    }
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect the release checkout.' }
    & git push origin HEAD:refs/heads/main
    if ($LASTEXITCODE -ne 0) { throw 'Source publication failed; no release workflow was started.' }

    Write-Host "Release source: $repository main@$revision"
    $requestedAt = [DateTimeOffset]::UtcNow.AddSeconds(-5)
    $dispatch = (& gh workflow run release.yml --repo $repository --ref main -f "version=$Version" -f "expected_sha=$revision") -join "`n"
    if ($LASTEXITCODE -ne 0) { throw 'Cannot start the native release workflow.' }
    if ($dispatch) { Write-Host $dispatch }
    $runMatch = [regex]::Match($dispatch, ('https://github\.com/' + [regex]::Escape($repository) + '/actions/runs/([0-9]+)'))
    $runID = if ($runMatch.Success) { $runMatch.Groups[1].Value } else { $null }
    for ($attempt = 0; -not $runID -and $attempt -lt 10; $attempt++) {
        $runs = & gh run list --repo $repository --workflow release.yml --commit $revision --event workflow_dispatch --limit 10 --json databaseId,displayTitle,createdAt
        if ($LASTEXITCODE -ne 0) { throw 'Cannot find the dispatched release workflow.' }
        $run = ($runs | ConvertFrom-Json) | Where-Object { $_.displayTitle -eq "Release $Version" -and [DateTimeOffset]$_.createdAt -ge $requestedAt } | Select-Object -First 1
        if ($run) { $runID = [string]$run.databaseId } else { Start-Sleep -Seconds 2 }
    }
    if (-not $runID) { throw 'Release was dispatched but its run is not visible yet; inspect GitHub Actions before retrying.' }
    $runRevision = (& gh run view $runID --repo $repository --json headSha --jq .headSha) -join ''
    if ($LASTEXITCODE -ne 0 -or $runRevision.Trim() -ne $revision) { throw 'The dispatched workflow does not match the reviewed source; no success is reported.' }
    Write-Host 'GitHub Actions will continue running if you close this terminal.'
    & gh run watch $runID --repo $repository --exit-status --interval 10
    if ($LASTEXITCODE -ne 0) { throw 'Release completion was not confirmed; inspect its GitHub Actions run before retrying.' }
} finally { Pop-Location }
