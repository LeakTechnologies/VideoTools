# Forgejo Runner Service Setup Script
# Run this script as Administrator in PowerShell

Write-Host "Setting up Forgejo Runner as Windows Service..." -ForegroundColor Cyan

# Define paths (using repository relative paths)
$runnerPath = "C:\ForgejoRunner"
$runnerBinary = "$runnerPath\bin\forgejo-runner.exe"
$configFile = "$runnerPath\config\runner.yaml"
$nssmPath = "C:\Program Files\NSSM\nssm.exe"

Write-Host "Checking files..." -ForegroundColor Yellow
if (-not (Test-Path $runnerBinary)) {
    Write-Host "ERROR: Runner binary not found at $runnerBinary" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $configFile)) {
    Write-Host "ERROR: Config file not found at $configFile" -ForegroundColor Red
    exit 1
}

Write-Host "Files verified" -ForegroundColor Green

# Install service
Write-Host "Installing service with NSSM..." -ForegroundColor Yellow
& $nssmPath install ForgejoRunner $runnerBinary

# Configure service
Write-Host "Configuring service..." -ForegroundColor Yellow
& $nssmPath set ForgejoRunner Arguments "daemon --config $configFile"
& $nssmPath set ForgejoRunner DisplayName "Forgejo CI Runner"
& $nssmPath set ForgejoRunner Description "Forgejo Actions runner for VideoTools Windows builds"

# Set log files
& $nssmPath set ForgejoRunner AppStdout "$runnerPath\logs\runner.log"
& $nssmPath set ForgejoRunner AppStderr "$runnerPath\logs\runner-error.log"
& $nssmPath set ForgejoRunner AppRotateFiles 1

# Set service user (important!)
& $nssmPath set ForgejoRunner ObjectName ".\ci-runner"

# Start service
Write-Host "Starting service..." -ForegroundColor Yellow
& $nssmPath start ForgejoRunner

# Verify service
Write-Host "Verifying service status..." -ForegroundColor Yellow
Start-Sleep -Seconds 3
$serviceStatus = & $nssmPath status ForgejoRunner

Write-Host "Service status: $serviceStatus" -ForegroundColor Cyan

if ($serviceStatus -like "RUNNING") {
    Write-Host "SUCCESS: Forgejo Runner service is running!" -ForegroundColor Green
    Write-Host ""
    Write-Host "The runner will automatically start on boot and connect to Forgejo." -ForegroundColor White
    Write-Host ""
    Write-Host "You can check.service status anytime with:" -ForegroundColor Cyan
    Write-Host "  nssm status ForgejoRunner" -ForegroundColor White
    Write-Host ""
    Write-Host "To stop service: nssm stop ForgejoRunner" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "To restart: nssm restart ForgejoRunner" -ForegroundColor White
} else {
    Write-Host "ERROR: Service failed to start" -ForegroundColor Red
    Write-Host "Status: $serviceStatus" -ForegroundColor Red
    exit 1
}