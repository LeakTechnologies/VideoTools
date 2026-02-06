# Manual Forgejo Runner Setup for Windows

Since automated downloads are failing due to network restrictions, please follow these manual steps:

## Option 1: Download Windows Build from Fork

1. **Visit the Windows fork**: https://github.com/Crown0815/forgejo-runner-windows
2. **Download the latest Windows build**:
   - Go to: https://github.com/Crown0815/forgejo-runner-windows/releases/latest
   - Download: `forgejo-runner-6.3.1-windows-amd64.exe`
3. **Save the file** to: `C:\ForgejoRunner\bin\forgejo-runner.exe`

## Option 2: Build from Source (if downloads fail completely)

If the fork doesn't work, we can build from source:

1. **Install Go** (if not already installed):
   ```powershell
   winget install --id GoLang.Go -e --source winget
   ```

2. **Clone the official runner repository**:
   ```powershell
   git clone https://code.forgejo.org/forgejo/runner.git C:\ForgejoRunner\src
   ```

3. **Build for Windows**:
   ```powershell
   cd C:\ForgejoRunner\src
   go build -ldflags="-s -w" -o ..\bin\forgejo-runner.exe
   ```

## Registration Steps (after you have the binary)

Once you have `forgejo-runner.exe` in `C:\ForgejoRunner\bin\`:

1. **Register with Forgejo**:
   ```powershell
   cd C:\ForgejoRunner
   .\bin\forgejo-runner.exe register --config config\runner.yaml
   ```
   
   **When prompted, enter:**
   - **Forgejo instance URL**: `https://git.leaktechnologies.dev`
   - **Registration token**: `0Q1zcb-rICQepSoRiWl4tsxbQNdgOe3k_pGrZMaM1lS`
   - **Runner name**: `win-runner-01`
   - **Runner group**: [default]
   - **Labels**: `windows,x64,videotools,ucrt64`

2. **Generate configuration**:
   ```powershell
   .\bin\forgejo-runner.exe generate-config > config\runner.yaml
   ```

3. **Test the runner**:
   ```powershell
   .\bin\forgejo-runner.exe daemon --config config\runner.yaml
   ```

## Alternative: Use Docker on Windows

If all else fails, you can run the runner in Docker:

1. **Install Docker Desktop** for Windows
2. **Create a docker-compose.yml** in `C:\ForgejoRunner\`:
   ```yaml
   version: '3.8'
   services:
     runner:
       image: code.forgejo.org/forgejo/runner:3.5.1
       volumes:
         - ./config:/data
         - /var/run/docker.sock:/var/run/docker.sock
       environment:
         - DOCKER_HOST=unix:///var/run/docker.sock
       command: daemon --config /data/config.yaml
   ```

3. **Register using Docker**:
   ```powershell
   docker-compose run --rm runner forgejo-runner register --no-interactive --token 0Q1zcb-rICQepSoRiWl4tsxbQNdgOe3k_pGrZMaM1lS --name win-runner-01 --instance https://git.leaktechnologies.dev
   ```

## Next Steps After Registration

Once you successfully register the runner (via any method above), let me know and I can help with:

1. **Installing as a Windows service**
2. **Configuring the runner for VideoTools builds**
3. **Testing the workflow**

Please let me know which approach works for you!