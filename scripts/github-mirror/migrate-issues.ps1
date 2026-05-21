<#
.SYNOPSIS
    Migrate all issues from a Forgejo repository to GitHub.

.DESCRIPTION
    Fetches all issues (open + closed) from a Forgejo repo via its API,
    creates matching issues on GitHub via the gh CLI, and outputs a
    mapping file of old -> new issue numbers.

    Labels, milestones, and assignees are NOT migrated (this repo has none).
    Issue comments are NOT migrated (this repo has none).

.PARAMETER GitHubOwner
    GitHub owner (user or org). Required.

.PARAMETER ForgejoBaseUrl
    Forgejo instance base URL. Default: https://git.leaktechnologies.dev

.PARAMETER ForgejoOwner
    Forgejo repository owner. Default: leak_technologies

.PARAMETER ForgejoRepo
    Forgejo repository name. Default: VideoTools

.PARAMETER GitHubRepo
    GitHub repository name. Default: VideoTools

.PARAMETER MappingFile
    Path to write old->new issue number mapping JSON. Default: issue-mapping.json

.PARAMETER DryRun
    If set, fetches issues from Forgejo and prints what would be created,
    but does not call gh issue create.

.PARAMETER ForgejoToken
    Forgejo API token. If not set, reads from FORGEJO_TOKEN env var.

.EXAMPLE
    $env:FORGEJO_TOKEN = "your_token_here"
    .\migrate-issues.ps1 -GitHubOwner yourname -DryRun
#>

param(
    [Parameter(Mandatory = $true)]
    [string]$GitHubOwner,

    [string]$ForgejoBaseUrl = "https://git.leaktechnologies.dev",
    [string]$ForgejoOwner = "leak_technologies",
    [string]$ForgejoRepo = "VideoTools",
    [string]$GitHubRepo = "VideoTools",
    [string]$MappingFile = "issue-mapping.json",
    [switch]$DryRun,
    [string]$ForgejoToken = $env:FORGEJO_TOKEN
)

$ErrorActionPreference = "Stop"

# ---- Prerequisites ----

if (-not $ForgejoToken) {
    Write-Error "ForgejoToken required. Set FORGEJO_TOKEN env var or pass -ForgejoToken."
    exit 1
}

try {
    $ghVersion = gh --version
    Write-Host "gh CLI found: $ghVersion"
} catch {
    Write-Error "GitHub CLI (gh) not found. Install from https://cli.github.com/"
    exit 1
}

# Verify gh is authenticated
try {
    gh auth status 2>&1 | Out-Null
} catch {
    Write-Error "gh not authenticated. Run: gh auth login"
    exit 1
}

# Verify gh can see the target repo (or it will be created by the first issue)
try {
    $null = gh repo view "$GitHubOwner/$GitHubRepo" --json name 2>&1
    Write-Host "GitHub repo $GitHubOwner/$GitHubRepo exists."
} catch {
    Write-Warning "GitHub repo $GitHubOwner/$GitHubRepo not found. Create it first, then re-run."
    exit 1
}

# ---- Fetch all issues from Forgejo ----

$page = 1
$allIssues = @()

Write-Host "Fetching issues from $ForgejoBaseUrl/$ForgejoOwner/$ForgejoRepo..."

do {
    $url = "$ForgejoBaseUrl/api/v1/repos/$ForgejoOwner/$ForgejoRepo/issues?state=all&page=$page&limit=50"

    try {
        $response = curl.exe -s -H "Authorization: token $ForgejoToken" $url
        $issues = $response | ConvertFrom-Json
    } catch {
        Write-Error "Failed to fetch page $page : $_"
        exit 1
    }

    if ($issues.Count -eq 0) { break }

    # Filter out pull requests (they have a pull_request field)
    $realIssues = $issues | Where-Object { -not $_.pull_request }
    $allIssues += $realIssues

    Write-Host "  Page $page : $($realIssues.Count) issues ($($issues.Count) total items)"
    $page++
} while ($issues.Count -eq 50)

Write-Host "Total issues fetched: $($allIssues.Count)"

if ($allIssues.Count -eq 0) {
    Write-Host "No issues found. Nothing to migrate."
    exit 0
}

# Sort by creation date ascending so issue order is preserved
$allIssues = $allIssues | Sort-Object created_at

# ---- Migrate each issue ----

$mapping = @{}
$created = 0
$skipped = 0
$errors = @()

# First, check for existing migration references on GitHub
Write-Host "Checking for already-migrated issues on GitHub..."
$existingRefs = @()
try {
    $ghIssues = gh issue list --repo "$GitHubOwner/$GitHubRepo" --state all --limit 1000 --json number,title,body 2>&1 | ConvertFrom-Json
    foreach ($gi in $ghIssues) {
        if ($gi.body -match "Migrated from Forgejo issue #(\d+)") {
            $existingRefs += [PSCustomObject]@{ ForgejoNum = [int]$Matches[1]; GitHubNum = $gi.number }
        }
    }
} catch {
    Write-Warning "Could not check existing issues: $_"
}

foreach ($existing in $existingRefs) {
    $mapping[$existing.ForgejoNum.ToString()] = $existing.GitHubNum
}

Write-Host "Found $($existingRefs.Count) already-migrated issues."

foreach ($issue in $allIssues) {
    $forgejoNum = $issue.number
    $forgejoUrl = "$ForgejoBaseUrl/$ForgejoOwner/$ForgejoRepo/issues/$forgejoNum"

    if ($mapping.ContainsKey($forgejoNum.ToString())) {
        Write-Host "  #$forgejoNum -> #$($mapping[$forgejoNum.ToString()]) (already migrated)"
        $skipped++
        continue
    }

    $title = $issue.title

    # Build body with migration banner + original author
    $author = $issue.user.login
    $createdAt = $issue.created_at
    $bodyLines = @(
        "> **Migrated from [Forgejo issue #${forgejoNum}](${forgejoUrl})**"
        "> **Original author: @${author}**"
        "> **Created: ${createdAt}**"
        ""
        $issue.body
    )
    $body = $bodyLines -join "`n"

    if ($DryRun) {
        Write-Host "  Would create: #$forgejoNum - $title"
        $mapping[$forgejoNum.ToString()] = "<dry-run>"
        $created++
        continue
    }

    # Create issue on GitHub
    Write-Host "  Creating #$forgejoNum - $title ..."

    try {
        $env:GH_TITLE = $title
        $env:GH_BODY = $body
        $result = gh issue create `
            --repo "$GitHubOwner/$GitHubRepo" `
            --title "$title" `
            --body "$body" 2>&1

        if (-not $result -or $result -match "^https://github.com") {
            # Extract new issue number from URL
            if ($result -match "issues/(\d+)$") {
                $newNum = [int]$Matches[1]
                $mapping[$forgejoNum.ToString()] = $newNum
                $created++

                # If issue was closed on Forgejo, close it on GitHub
                if ($issue.state -eq "closed") {
                    Write-Host "    Closing as #$newNum (was closed on Forgejo)..."
                    gh api "repos/$GitHubOwner/$GitHubRepo/issues/$newNum" `
                        --method PATCH `
                        -f state=closed 2>&1 | Out-Null
                }
            } else {
                Write-Warning "    Could not parse new issue number from: $result"
                $errors += "#$forgejoNum: could not parse result"
            }
        } else {
            Write-Warning "    Failed to create issue: $result"
            $errors += "#$forgejoNum: $result"
        }
    } catch {
        Write-Warning "    Error creating issue #$forgejoNum : $_"
        $errors += "#$forgejoNum: $_"
    }
}

# ---- Write mapping file ----

$mappingObj = [PSCustomObject]@{
    ForgejoUrl = "$ForgejoBaseUrl/$ForgejoOwner/$ForgejoRepo"
    GitHubUrl  = "https://github.com/$GitHubOwner/$GitHubRepo"
    MigratedAt = (Get-Date -Format "o")
    Count      = $mapping.Count
    Issues     = $mapping
}

$mappingJson = $mappingObj | ConvertTo-Json -Depth 10
$mappingJson | Out-File -FilePath $MappingFile -Encoding utf8

# ---- Summary ----

Write-Host ""
Write-Host "=============================="
Write-Host "  Migration Complete"
Write-Host "=============================="
Write-Host "  Total Forgejo issues: $($allIssues.Count)"
Write-Host "  Created on GitHub:    $created"
Write-Host "  Already migrated:     $skipped"
if ($errors.Count -gt 0) {
    Write-Host "  Errors:               $($errors.Count)"
    foreach ($err in $errors) {
        Write-Host "    $err"
    }
}
Write-Host "  Mapping file:         $MappingFile"
Write-Host ""
Write-Host "Add this to Forgejo repo description:"
Write-Host "  Issues tracked on GitHub — https://github.com/$GitHubOwner/$GitHubRepo/issues"
