# VideoTools primary workspace launcher for opencode.
# Anchors every agent session to the canonical workspace root. Never boot from
# C:\Users\User\Videos\VideoTools or VideoTools-Versions — those are redundant
# capture/archive trees and do not hold the active codebase.
#
# Usage:  .\scripts\windows\opencode-workspace.ps1 [-Model google/gemini-2.5-flash]

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

param(
    [string]$Model = "google/gemini-2.5-flash"
)

$projectRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
Set-Location $projectRoot

Write-Host "Workspace: $projectRoot" -ForegroundColor Cyan
Write-Host "Model:     $Model" -ForegroundColor Cyan
Write-Host ""

opencode --model $Model
exit $LASTEXITCODE
