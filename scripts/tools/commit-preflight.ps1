# Commit provenance gate. Read-only: no network, no index writes, no checkout
# changes, no configuration writes.
#
# Checks, each printed as RAN or UNKNOWN together with PASS, WARN or FAIL:
#   1. Issue reference    - every non-merge commit carries `Refs #N`, `Fixes #N`
#                           or `Closes #N`. Required by docs/ENGINEERING_WORKFLOW.md.
#   2. Documentation sync - a commit that changes non-documentation files also
#                           changes docs/CHANGELOG.md, DONE.md or another guide.
#                           Warning by default; failing under -Strict.
#   3. Encoding guard     - no signature appears that was not already present in
#                           the same file's previous revision. Compared on raw
#                           bytes, because Windows PowerShell 5.1 decodes native
#                           stdout through the OEM code page and loses exactly the
#                           byte sequences this check exists to find.
#
# Exit codes: 0 = every check ran and passed; 1 = a failing check; 2 = a check
# could not run, so nothing is proven.
[CmdletBinding()]
param(
    [string]$Repository = '',
    [string]$Revision = '',
    [switch]$Strict
)
$ErrorActionPreference = 'Stop'

# Resolve the default in the body: under `-File` in Windows PowerShell 5.1 the
# param block is bound before $PSScriptRoot is populated.
if ([string]::IsNullOrEmpty($Repository)) {
    $Repository = (Join-Path $PSScriptRoot '../..')
}

function Invoke-Git {
    param([string[]]$Arguments)
    $result = & git --no-optional-locks -C $Repository @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw ($result -join "`n") }
    return ($result -join "`n")
}

function Invoke-GitLines {
    param([string[]]$Arguments)
    $output = Invoke-Git $Arguments
    if ([string]::IsNullOrEmpty($output)) { return @() }
    return @($output -split "`n")
}

# Signature bytes in UTF-8. Declared as byte values so this file cannot match
# itself, and so the comparison never depends on text decoding.
$signatureBytes = @(
    [byte[]](0xC3, 0x83),
    [byte[]](0xC2, 0x82),
    [byte[]](0xEF, 0xBF, 0xBD),
    [byte[]](0xC3, 0xA2, 0xE2, 0x82, 0xAC)
)

function Get-SignatureCount {
    param([byte[]]$Bytes)
    if ($null -eq $Bytes -or $Bytes.Length -eq 0) { return 0 }
    $total = 0
    foreach ($signature in $signatureBytes) {
        $i = 0
        while ($i -le ($Bytes.Length - $signature.Length)) {
            $matched = $true
            for ($j = 0; $j -lt $signature.Length; $j++) {
                if ($Bytes[$i + $j] -ne $signature[$j]) { $matched = $false; break }
            }
            if ($matched) { $total++; $i += $signature.Length } else { $i++ }
        }
    }
    return $total
}

function Test-BinaryBytes {
    param([byte[]]$Bytes)
    if ($null -eq $Bytes) { return $true }
    $limit = [Math]::Min($Bytes.Length, 8000)
    for ($i = 0; $i -lt $limit; $i++) { if ($Bytes[$i] -eq 0) { return $true } }
    return $false
}

# Raw blob bytes. Start-Process redirects at the OS level, so the file on disk is
# the blob exactly as git wrote it.
function Get-BlobBytes {
    param([string]$Spec)
    $base = [IO.Path]::Combine($env:TEMP, ('commit-preflight-' + [guid]::NewGuid().ToString('N')))
    $outFile = $base + '.out'
    $errFile = $base + '.err'
    try {
        $process = Start-Process -FilePath 'git' -NoNewWindow -Wait -PassThru `
            -ArgumentList @('--no-optional-locks', '-C', $Repository, 'cat-file', 'blob', $Spec) `
            -RedirectStandardOutput $outFile -RedirectStandardError $errFile
        if ($process.ExitCode -ne 0) { return $null }
        if (!(Test-Path -LiteralPath $outFile)) { return $null }
        return [IO.File]::ReadAllBytes($outFile)
    } finally {
        foreach ($file in @($outFile, $errFile)) {
            if (Test-Path -LiteralPath $file) { Remove-Item -LiteralPath $file -Force -ErrorAction SilentlyContinue }
        }
    }
}

function Test-DocumentationPath {
    param([string]$Path)
    if ($Path -match '(?i)^docs/') { return $true }
    if ($Path -match '(?i)\.md$') { return $true }
    if ($Path -match '(?i)^(LICENSE|NOTICE)$') { return $true }
    return $false
}

$failures = @()
$warnings = @()

try {
    $root = Invoke-Git @('rev-parse', '--show-toplevel')
    Write-Output '=== commit-preflight ==='
    Write-Output "Root: $root"
    $mode = if ($Strict) { 'documentation sync fails' } else { 'documentation sync warns' }
    Write-Output "Mode: $mode"

    # Resolve the commit set without touching the working tree.
    if ($Revision -ne '') {
        if ($Revision -match '\.\.') {
            $commitLines = @(Invoke-GitLines @('rev-list', '--reverse', $Revision))
            $rangeLabel = $Revision
        } else {
            $commitLines = @(Invoke-GitLines @('rev-list', '-n', '1', $Revision))
            $rangeLabel = "$Revision (single commit)"
        }
    } else {
        try {
            $upstream = Invoke-Git @('rev-parse', '--abbrev-ref', '--symbolic-full-name', '@{upstream}')
            $commitLines = @(Invoke-GitLines @('rev-list', '--reverse', '@{upstream}..HEAD'))
            if ($commitLines.Count -eq 0) {
                $commitLines = @(Invoke-GitLines @('rev-list', '-n', '1', 'HEAD'))
                $rangeLabel = 'HEAD (nothing unpublished against the local upstream)'
            } else {
                $rangeLabel = "unpublished against $upstream"
            }
        } catch {
            $commitLines = @(Invoke-GitLines @('rev-list', '-n', '1', 'HEAD'))
            $rangeLabel = 'HEAD (local upstream unknown; only the current commit is checked)'
        }
    }
    $commits = @($commitLines | Where-Object { $_ -ne '' })
    Write-Output "Range: $rangeLabel"
    Write-Output "Commits checked: $($commits.Count)"
    Write-Output ''

    if ($commits.Count -eq 0) { throw 'No commit resolved for the requested range.' }

    # ---- Check 1: issue reference -------------------------------------------
    $missingRef = @()
    $mergeExempt = 0
    foreach ($sha in $commits) {
        $message = Invoke-Git @('log', '-1', '--format=%B', $sha)
        $subject = Invoke-Git @('log', '-1', '--format=%s', $sha)
        if ($subject -match '^Merge ') { $mergeExempt++; continue }
        if ($message -notmatch '(?im)^\s*(Refs|Fixes|Closes)\s+#\d+') {
            $missingRef += ('{0}  {1}' -f $sha.Substring(0, 8), $subject)
        }
    }
    if ($missingRef.Count -eq 0) {
        Write-Output ('[1/3] Issue reference ........ RAN - PASS ({0} commit(s) checked, {1} merge exempt)' -f ($commits.Count - $mergeExempt), $mergeExempt)
    } else {
        Write-Output ('[1/3] Issue reference ........ RAN - FAIL ({0} of {1} commit(s) carry no Refs #N)' -f $missingRef.Count, ($commits.Count - $mergeExempt))
        foreach ($entry in $missingRef) { Write-Output "      $entry" }
        $failures += 'issue reference'
    }

    # ---- Check 2: documentation sync ----------------------------------------
    $docViolations = @()
    foreach ($sha in $commits) {
        $subject = Invoke-Git @('log', '-1', '--format=%s', $sha)
        if ($subject -match '^Merge ') { continue }
        $changed = @(Invoke-GitLines @('show', '--name-only', '--format=', $sha) | Where-Object { $_ -ne '' })
        $touchesCode = $false
        $touchesDoc = $false
        foreach ($path in $changed) {
            if (Test-DocumentationPath $path) { $touchesDoc = $true } else { $touchesCode = $true }
        }
        if ($touchesCode -and -not $touchesDoc) {
            $docViolations += ('{0}  {1}' -f $sha.Substring(0, 8), $subject)
        }
    }
    if ($docViolations.Count -eq 0) {
        Write-Output '[2/3] Documentation sync ...... RAN - PASS'
    } elseif ($Strict) {
        Write-Output ('[2/3] Documentation sync ...... RAN - FAIL ({0} commit(s) changed code without a documentation entry)' -f $docViolations.Count)
        foreach ($entry in $docViolations) { Write-Output "      $entry" }
        $failures += 'documentation sync'
    } else {
        Write-Output ('[2/3] Documentation sync ...... RAN - WARN ({0} commit(s) changed code without a documentation entry)' -f $docViolations.Count)
        foreach ($entry in $docViolations) { Write-Output "      $entry" }
        $warnings += 'documentation sync'
    }

    # ---- Check 3: encoding guard, raw bytes ---------------------------------
    $introduced = @()
    $preExisting = @()
    $scanned = 0
    $skipped = 0
    foreach ($sha in $commits) {
        $subject = Invoke-Git @('log', '-1', '--format=%s', $sha)
        if ($subject -match '^Merge ') { continue }
        $changed = @(Invoke-GitLines @('show', '--name-only', '--format=', $sha) | Where-Object { $_ -ne '' })
        foreach ($path in $changed) {
            if ($path -match '(?i)^_fyne/') { $skipped++; continue }
            $current = Get-BlobBytes ('{0}:{1}' -f $sha, $path)
            if ($null -eq $current) { continue }
            if (Test-BinaryBytes $current) { $skipped++; continue }
            $scanned++
            $currentCount = Get-SignatureCount $current
            if ($currentCount -eq 0) { continue }
            $previousCount = 0
            $previous = Get-BlobBytes ('{0}^:{1}' -f $sha, $path)
            if ($null -ne $previous) { $previousCount = Get-SignatureCount $previous }
            if ($currentCount -gt $previousCount) {
                $introduced += ('{0}  {1} ({2} -> {3} signatures)' -f $sha.Substring(0, 8), $path, $previousCount, $currentCount)
            } else {
                $preExisting += ('{0}  {1} ({2} already present)' -f $sha.Substring(0, 8), $path, $currentCount)
            }
        }
    }
    if ($introduced.Count -eq 0) {
        Write-Output ('[3/3] Encoding guard ......... RAN - PASS ({0} text file(s) inspected, {1} skipped as binary or vendored)' -f $scanned, $skipped)
    } else {
        Write-Output ('[3/3] Encoding guard ......... RAN - FAIL ({0} file(s) gained a legacy signature)' -f $introduced.Count)
        foreach ($entry in $introduced) { Write-Output "      $entry" }
        $failures += 'encoding guard'
    }
    if ($preExisting.Count -gt 0) {
        Write-Output ('      Pre-existing signatures in files this commit already touches ({0}); not introduced here:' -f $preExisting.Count)
        foreach ($entry in $preExisting) { Write-Output "        $entry" }
        $warnings += 'pre-existing encoding damage'
    }

    Write-Output ''
    if ($failures.Count -eq 0 -and $warnings.Count -eq 0) {
        Write-Output 'Result: PASS - every check ran and passed.'
        exit 0
    }
    if ($failures.Count -eq 0) {
        Write-Output ('Result: PASS with warnings ({0}).' -f ($warnings -join '; '))
        exit 0
    }
    Write-Output ('Result: FAIL ({0}).' -f ($failures -join '; '))
    Write-Output 'This gate proves provenance only. It is not CI, release or runtime acceptance.'
    exit 1
} catch {
    Write-Output ('Result: UNKNOWN - {0}' -f $_.Exception.Message)
    Write-Output 'No claim is made that any check passed.'
    exit 2
}
