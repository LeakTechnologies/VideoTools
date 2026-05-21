# GitHub Mirror — VideoTools

Scripts and documentation for mirroring the VideoTools repo from Forgejo (git.leaktechnologies.dev) to GitHub.

## Repo Push Mirror

Forgejo has **built-in push mirroring** — no CI needed.

### Setup

1. Generate a GitHub [Personal Access Token](https://github.com/settings/tokens) with `repo` scope.
2. On Forgejo, go to: **Repository → Settings → Mirroring → Add Push Mirror**
3. Enter the remote URL:
   ```
   https://<GITHUB_USERNAME>:<PAT>@github.com/<GITHUB_USERNAME>/videotools.git
   ```
4. Click **Add Push Mirror**.

Every `git push` to Forgejo now automatically reflects to GitHub. The mirror runs synchronously — if the push to GitHub fails, the Forgejo push also fails so you know immediately.

## Repo Metadata Setup

Run `setup-repo.ps1` after installing the GitHub CLI to set the repo description, website URL, and topics:

```powershell
# Preview changes
.\setup-repo.ps1 -DryRun

# Apply
.\setup-repo.ps1
```

Sets:
- **Description**: "Desktop video processing suite with native DVD authoring, disc ripping, media conversion, AI upscaling, and more. Built with Go + FFmpeg."
- **Website**: https://leaktechnologies.dev
- **Topics**: go, golang, ffmpeg, video-processing, video-converter, dvd-authoring, dvd-ripping, desktop-app, fyne

## Issue Migration

### Prerequisites

- [GitHub CLI (`gh`)](https://cli.github.com/) installed and authenticated (`gh auth login`)
- A Forgejo API token with `read:repository` scope (Settings → Applications → Generate Token)

### Usage

```powershell
# Set your Forgejo API token
$env:FORGEJO_TOKEN = "your_token_here"

# Preview what would be migrated (no writes to GitHub)
.\migrate-issues.ps1 -GitHubOwner yourname -DryRun

# Run the migration
.\migrate-issues.ps1 -GitHubOwner yourname
```

### What it does

1. Fetches all issues (open + closed) from the Forgejo API
2. Checks which ones already have a migration banner on GitHub (skips duplicates)
3. Creates each new issue on GitHub via `gh issue create`
4. Closes issues on GitHub that were closed on Forgejo
5. Writes `issue-mapping.json` with `old# -> new#` mapping

### What it does NOT do (this repo has none of these)

- **Labels** — not migrated (0 labels exist)
- **Milestones** — not migrated (0 milestones exist)
- **Assignees** — not migrated (0 assignees exist)
- **Comments** — not migrated (0 comments exist)
- **Pull requests** — filtered out, not migrated

### Post-migration

After the migration completes:

1. Update the Forgejo repo description:
   > Issues tracked on GitHub — https://github.com/YOURNAME/videotools/issues
2. Add a note to the repo wiki / docs if needed
3. Close the Forgejo issue tracker (Settings → Repository → Enable Issues — uncheck)
