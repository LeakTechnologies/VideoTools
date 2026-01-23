# VideoTools Agent Workflow Rules

These rules apply to any automation or agent working in this repo.

## Commit Discipline

- After every change: `git add` then `git commit -m "..."`.
- Do not leave unstaged changes in the worktree.
- Commit only files related to the current task.

## Documentation Discipline

- If behavior changes, update:
  - `docs/INSTALLATION.md`
  - the relevant platform guide (`docs/INSTALL_WINDOWS.md`, `docs/INSTALL_LINUX.md`)
- Always update `DONE.md` and `TODO.md` for completed or planned work.
- Avoid personal names in documentation; use `user report` or `dev report` only.

## Windows Install Flow

- Use `scripts\install.ps1` or `scripts\install.bat` from PowerShell/CMD.
- `scripts/install.sh` is for bash shells only; do not run it from PowerShell.

## Coordination

- Ask before changing workflow entrypoints or automation behavior.
- If a change affects installs/builds, add a short note in docs.
