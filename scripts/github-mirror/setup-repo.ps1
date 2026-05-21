<#
.SYNOPSIS
    Configure GitHub repo metadata (description, website, topics).

.DESCRIPTION
    Updates the LeakTechnologies/VideoTools GitHub repo settings to match
    the project's current state. Requires GitHub CLI (gh).

.PARAMETER Repo
    GitHub repo in owner/name format. Default: LeakTechnologies/VideoTools

.PARAMETER DryRun
    If set, print what would be changed without making changes.

.EXAMPLE
    .\setup-repo.ps1
    .\setup-repo.ps1 -DryRun
#>

param(
    [string]$Repo = "LeakTechnologies/VideoTools",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

# ---- Prerequisites ----

try {
    $ghVersion = gh --version
    Write-Host "gh CLI found: $ghVersion"
} catch {
    Write-Error "GitHub CLI (gh) not found. Install from https://cli.github.com/"
    exit 1
}

try {
    gh auth status 2>&1 | Out-Null
    Write-Host "Authenticated to GitHub."
} catch {
    Write-Error "Not authenticated. Run: gh auth login"
    exit 1
}

# ---- Current state ----

Write-Host "Current repo metadata for $Repo :"
gh repo view $Repo --json name,description,url,homepageUrl,repositoryTopics 2>&1

# ---- Description ----

$description = "Desktop video processing suite with native DVD authoring, disc ripping, media conversion, AI upscaling, and more. Built with Go + FFmpeg."

if ($DryRun) {
    Write-Host "`nWould set description to:"
    Write-Host "  $description"
} else {
    Write-Host "`nUpdating description..."
    gh repo edit $Repo --description $description
}

# ---- Website ----

$homepage = "https://leaktechnologies.dev"

if ($DryRun) {
    Write-Host "Would set homepage to: $homepage"
} else {
    Write-Host "Updating homepage..."
    gh repo edit $Repo --homepage $homepage
}

# ---- Topics ----

$topics = @(
    "go",
    "golang",
    "ffmpeg",
    "video-processing",
    "video-converter",
    "dvd-authoring",
    "dvd-ripping",
    "desktop-app",
    "fyne"
)

if ($DryRun) {
    Write-Host "`nWould set topics to:"
    $topics | ForEach-Object { Write-Host "  - $_" }
} else {
    Write-Host "`nUpdating topics..."
    $topicsStr = $topics -join ","
    gh repo edit $Repo --add-topic $topicsStr
}

# ---- Enable Issues ----

if (-not $DryRun) {
    Write-Host "`nEnabling issues tracker..."
    gh repo edit $Repo --enable-issues=true
}

# ---- Summary ----

Write-Host "`nDone. Verify at: https://github.com/$Repo"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Configure Forgejo push mirror (Forgejo UI -> Settings -> Mirroring -> Add Push Mirror)"
Write-Host "  2. Run migrate-issues.ps1 to port existing issues"
Write-Host "  3. Disable Forgejo issue tracker after migration"
