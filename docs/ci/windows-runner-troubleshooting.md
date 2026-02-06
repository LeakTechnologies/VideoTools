# Windows Runner Security and Troubleshooting Guide

This guide covers security best practices and troubleshooting for the VideoTools Windows Forgejo runner.

## Security Best Practices

### Runner Account Security
- **Non-privileged user**: Runner runs as `ci-runner` without admin rights
- **Dedicated account**: Only used for CI, no interactive login
- **Password policy**: Strong, randomly generated password
- **Minimal permissions**: Only access to necessary directories

### Service Security
```powershell
# Verify service runs as correct user
sc qc ForgejoRunner

# Check service permissions
Get-Acl C:\ForgejoRunner | Format-List
```

### Network Security
- **Outbound only**: Runner only needs outbound HTTPS to Forgejo
- **No inbound ports**: No services exposed externally
- **Firewall rules**: Consider restricting to specific domains
```powershell
# Example firewall rule (if needed)
New-NetFirewallRule -DisplayName "Forgejo Runner Outbound" -Direction Outbound -Protocol TCP -RemoteAddress git.leaktechnologies.dev -Action Allow
```

### Secrets Management
- **Never log secrets**: Use environment variables only
- **Secure token storage**: Registration token is single-use
- **Workspace cleanup**: Secrets never persist between jobs
- **No hardcoded credentials**: All sensitive data via Forgejo secrets

### File System Security
```powershell
# Verify correct permissions
icacls C:\ForgejoRunner

# Expected permissions:
# ci-runner: Full control on C:\ForgejoRunner
# System: Read access on runner binary
# Administrators: Full control for maintenance
```

## Troubleshooting Guide

### Runner Registration Issues

#### Problem: Registration token invalid
**Symptoms**: Runner fails to register with "bad credentials" error
**Solutions**:
1. Generate new token in Forgejo admin UI
2. Ensure token hasn't expired (single-use)
3. Check Forgejo URL is correct: `https://git.leaktechnologies.dev`

#### Problem: Network connectivity
**Symptoms**: "connection refused" or timeout errors
**Solutions**:
```powershell
# Test connectivity
Test-NetConnection -ComputerName git.leaktechnologies.dev -Port 443

# Check DNS
Resolve-DnsName git.leaktechnologies.dev

# Verify SSL certificate
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri https://git.leaktechnologies.dev -UseBasicParsing
```

### Service Issues

#### Problem: Service won't start
**Symptoms**: Service start fails or immediately stops
**Solutions**:
```powershell
# Check service status
sc query ForgejoRunner

# Check event log
Get-EventLog -LogName Application -Source "ForgejoRunner" -Newest 10

# Test manual start
C:\ForgejoRunner\bin\forgejo-runner.exe daemon --config C:\ForgejoRunner\config\runner.yaml
```

#### Problem: Service stops unexpectedly
**Symptoms**: Runner works then stops after some time
**Solutions**:
1. Check logs for crash details
2. Verify system resources (memory, disk space)
3. Check Windows Event Viewer for system errors

### Build Issues

#### Problem: Go not found
**Symptoms**: "go: command not found" in builds
**Solutions**:
```powershell
# Check Go installation
go version

# Verify PATH
echo $env:PATH

# Check service user PATH
# Run as ci-runner user and check PATH
```

#### Problem: GCC/MSYS2 issues
**Symptoms**: CGO compilation failures
**Solutions**:
```powershell
# Test GCC
gcc --version

# Test compilation
echo 'int main(){return 0;}' > test.c
gcc -c test.c -o test.o
gcc test.o -o test.exe
./test.exe

# Check MSYS2 environment
C:\msys64\usr\bin\bash.exe -lc "which gcc"
```

#### Problem: PATH issues in service context
**Symptoms**: Tools work interactively but fail in CI
**Solutions**:
1. Check service runs as correct user
2. Verify system PATH includes all required directories
3. Restart service after PATH changes

### Workflow Issues

#### Problem: Jobs stuck queued
**Symptoms**: Jobs never start, remain in queued state
**Solutions**:
1. Check runner labels match job requirements
2. Verify runner is online in Forgejo UI
3. Check runner capacity in config

#### Problem: Label mismatch
**Symptoms**: Runner online but jobs don't execute
**Solutions**:
```yaml
# In workflow file:
runs-on: windows  # Must match runner labels

# In runner config:
labels:
  - "windows:host"
```

### Performance Issues

#### Problem: Slow builds
**Symptoms**: Builds take much longer than expected
**Solutions**:
1. Check system resource usage
2. Verify adequate disk space for workspace
3. Consider increasing runner capacity in config

#### Problem: Disk space exhaustion
**Symptoms**: Builds fail with "no space left" errors
**Solutions**:
```powershell
# Clean workspace
Remove-Item C:\ForgejoRunner\work\* -Recurse -Force

# Clean Go cache
go clean -modcache

# Configure workspace cleanup in runner.yaml
```

## Diagnostic Commands

### System Information
```powershell
# OS version
[Environment]::OSVersion

# System resources
Get-CimInstance Win32_ComputerSystem | Select-Object TotalPhysicalMemory
Get-CimInstance Win32_LogicalDisk | Select-Object DeviceID, Size, FreeSpace

# Environment variables
Get-ChildItem Env:
```

### Service Diagnostics
```powershell
# Service configuration
sc qc ForgejoRunner

# Service dependencies
sc enumdepend ForgejoRunner

# Service status
sc query ForgejoRunner

# Service logs
Get-Content C:\ForgejoRunner\logs\runner.log -Tail 50
```

### Network Diagnostics
```powershell
# Test connectivity
Test-NetConnection -ComputerName git.leaktechnologies.dev -Port 443

# Trace route
Test-NetConnection -ComputerName git.leaktechnologies.dev -TraceRoute

# SSL certificate check
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
$cert = Invoke-WebRequest -Uri https://git.leaktechnologies.dev -UseBasicParsing
$cert.BaseResponse.ResponseUri
```

### Toolchain Diagnostics
```powershell
# Go diagnostics
go version
go env

# Git diagnostics
git --version
git config --list

# MSYS2 diagnostics
C:\msys64\usr\bin\bash.exe -lc "pacman -Q"
```

## Maintenance Tasks

### Weekly Maintenance
```powershell
# Clean workspace
Remove-Item C:\ForgejoRunner\work\* -Recurse -Force -ErrorAction SilentlyContinue

# Clean logs (keep last 7 days)
Get-ChildItem C:\ForgejoRunner\logs\*.log | Where-Object CreationTime -lt (Get-Date).AddDays(-7) | Remove-Item

# Check disk space
Get-CimInstance Win32_LogicalDisk | Select-Object DeviceID, @{Name="FreeGB";Expression={[math]::Round($_.FreeSpace / 1GB, 2)}}
```

### Monthly Maintenance
```powershell
# Update MSYS2 packages
C:\msys64\usr\bin\bash.exe -lc "pacman -Syu --noconfirm"

# Check for Go updates
go version

# Review runner logs for issues
Get-Content C:\ForgejoRunner\logs\runner.log | Select-String -Pattern "ERROR|WARN"
```

### Security Audits
```powershell
# Check user accounts
Get-LocalUser | Where-Object Enabled -eq $true

# Check running services
Get-Service | Where-Object Status -eq "Running"

# Check network connections
Get-NetTCPConnection | Where-Object State -eq "Established"
```

## Emergency Procedures

### Runner Compromise
If runner is suspected compromised:
1. **Stop service**: `nssm stop ForgejoRunner`
2. **Revoke token**: Generate new registration token in Forgejo
3. **Change passwords**: Update ci-runner user password
4. **Rebuild**: Rebuild system from known good state
5. **Re-register**: Register runner with new token

### Service Recovery
```powershell
# Emergency restart
nssm restart ForgejoRunner

# Force stop
nssm stop ForgejoRunner
nssm start ForgejoRunner

# Reinstall service (if needed)
nssm remove ForgejoRunner
# Then reinstall using setup documentation
```

### Data Recovery
```powershell
# Backup configuration
Copy-Item C:\ForgejoRunner\config\runner.yaml C:\ForgejoRunner\backup\

# Backup logs
Copy-Item C:\ForgejoRunner\logs\* C:\ForgejoRunner\backup\logs\

# Restore from backup
Copy-Item C:\ForgejoRunner\backup\runner.yaml C:\ForgejoRunner\config\
```

## Contact and Support

### Internal Support
- **Administrator**: Stu Leak <leaktechnologies@proton.me>
- **Documentation**: `docs/ci/windows-runner.md`
- **Bootstrap script**: `scripts/ci/bootstrap-windows.ps1`

### External Resources
- **Forgejo Documentation**: https://forgejo.org/docs/
- **Forgejo Runner Issues**: https://code.forgejo.org/forgejo/runner/issues
- **MSYS2 Documentation**: https://www.msys2.org/docs/

### Log Collection
For support requests, collect:
```powershell
# System information
systeminfo > systeminfo.txt

# Service configuration
sc qc ForgejoRunner > service-config.txt

# Recent logs
Get-Content C:\ForgejoRunner\logs\runner.log -Tail 100 > runner-log.txt

# Environment
Get-ChildItem Env: > environment.txt
```

This guide should help resolve most common issues with the VideoTools Windows Forgejo runner.