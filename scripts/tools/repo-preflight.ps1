# Local inspection only: no network, configuration, checkout or index writes.
[CmdletBinding()]
param([string]$Repository = (Join-Path $PSScriptRoot '../..'))
$ErrorActionPreference = 'Stop'
function Read-Git {
    param([string[]]$Arguments)
    $result = & git --no-optional-locks -C $Repository @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw ($result -join "`n") }
    return ($result -join "`n")
}
try {
    $root = Read-Git @('rev-parse', '--show-toplevel')
    $origin = Read-Git @('remote', 'get-url', 'origin')
    Write-Output "Root: $root"
    # Never print credential-bearing remote URLs.
    $canonical = $origin -match '^(https://github\.com/|git@github\.com:)LeakTechnologies/VideoTools(?:\.git)?/?$'
    Write-Output "Canonical origin: $canonical"
    if (!$canonical) { throw 'Origin does not match LeakTechnologies/VideoTools; inspect repository identity.' }
    Write-Output "HEAD: $(Read-Git @('rev-parse', 'HEAD'))"
    Write-Output "Branch: $(Read-Git @('rev-parse', '--abbrev-ref', 'HEAD'))"
    $versionPath = Join-Path $root 'VERSION'
    if (!(Test-Path -LiteralPath $versionPath)) { throw 'VERSION is missing.' }
    Write-Output "Application VERSION: $([IO.File]::ReadAllText($versionPath).Trim())"
    Write-Output 'Upstream counts use LOCAL refs only; no fetch performed, freshness unknown.'
    try {
        $upstream = Read-Git @('rev-parse', '--abbrev-ref', '--symbolic-full-name', '@{upstream}')
        $counts = Read-Git @('rev-list', '--left-right', '--count', 'HEAD...@{upstream}')
        Write-Output "Upstream: $upstream; ahead/behind: $counts"
    } catch { Write-Output "Upstream: UNKNOWN ($($_.Exception.Message))" }
    Write-Output "Worktrees:`n$(Read-Git @('worktree', 'list', '--porcelain'))"
    # Keep tracked/untracked output even if recursive submodule status is unreadable.
    $status = Read-Git @('status', '--short', '--untracked-files=normal', '--ignore-submodules=all')
    if (!$status) { $status = 'clean (submodules excluded here)' }
    Write-Output "Main worktree:`n$status"
    try { Write-Output "Submodule revisions:`n$(Read-Git @('submodule', 'status', '--recursive'))" } catch { Write-Output "Submodule revisions: UNKNOWN ($($_.Exception.Message))"; exit 2 }
    try {
        $full = Read-Git @('status', '--short', '--untracked-files=normal', '--ignore-submodules=none')
        if (!$full) { $full = 'clean including submodule inspection' }
        Write-Output "Full worktree:`n$full"
    } catch {
        Write-Output "Full worktree: UNKNOWN; submodule inspection failed: $($_.Exception.Message)"
        exit 2
    }
    Write-Output 'No CI, release, approval or runtime acceptance inferred. Read AGENTS.md and the active issue.'
} catch {
    Write-Error "Preflight incomplete: $($_.Exception.Message)"
    exit 1
}