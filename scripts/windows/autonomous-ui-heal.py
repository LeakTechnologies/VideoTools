#!/usr/bin/env python3
"""VideoTools autonomous UI heal harness (baseline runtime architecture).

Launches VideoTools.exe and watches its main window for UI-thread hangs,
captures diagnostics when the window stops pumping messages, kills the
process tree and relaunches. The correction loop is hard-capped at five
runs (global AGENTS.md loop-engineering rule); every run writes its own
diagnostic bundle so a human operator can inspect the failure class.

Exit codes:
    0   stable — survived the full observation window
    1   heal budget exhausted (run cap reached without a stable run)
    2   setup error (missing exe, app already running, bad arguments)
    3   verify gate failed (dev-verify.ps1 returned non-zero)

Usage:
    python scripts/windows/autonomous-ui-heal.py
    python scripts/windows/autonomous-ui-heal.py --verify --stabilize 90
"""

import argparse
import ctypes
import datetime
import json
import shutil
import subprocess
import sys
import time
from ctypes import wintypes
from pathlib import Path

MAX_RUNS_CAP = 5  # global hard cap — never raise without Human Director approval
STABLE = "stable"
EXITED = "process-exited"
HUNG = "ui-hung"
NOWIN = "no-main-window"

HEAL_ROOT = Path(__file__).resolve().parent / "heal-logs"


def parse_args():
    root = Path(__file__).resolve().parents[1]
    p = argparse.ArgumentParser(description="VideoTools autonomous UI heal harness")
    p.add_argument("--exe", type=Path, default=root / "VideoTools.exe",
                   help="path to the built executable (default: repo root)")
    p.add_argument("--max-runs", type=int, default=MAX_RUNS_CAP,
                   help="heal attempts before yielding (hard-capped at %d)" % MAX_RUNS_CAP)
    p.add_argument("--warmup", type=int, default=15,
                   help="seconds to wait for the main window after launch")
    p.add_argument("--stabilize", type=int, default=60,
                   help="seconds the app must stay responsive to count as stable")
    p.add_argument("--poll", type=float, default=1.0,
                   help="seconds between responsiveness polls")
    p.add_argument("--verify", action="store_true",
                   help="run scripts/windows/dev-verify.ps1 before each launch")
    args = p.parse_args()
    if args.max_runs > MAX_RUNS_CAP:
        print("max-runs %d exceeds global cap %d; clamping"
              % (args.max_runs, MAX_RUNS_CAP))
        args.max_runs = MAX_RUNS_CAP
    if args.max_runs < 1:
        args.max_runs = 1
    return args


def find_main_hwnd(pid):
    """Return the visible main-window handle for a PID, or None."""
    user32 = ctypes.WinDLL("user32", use_last_error=True)
    found = []
    enum_proc = ctypes.WINFUNCTYPE(wintypes.BOOL, wintypes.HWND, wintypes.LPARAM)
    get_pid = user32.GetWindowThreadProcessId

    def cb(hwnd, _lparam):
        out = wintypes.DWORD()
        get_pid(hwnd, ctypes.byref(out))
        if out.value == pid and user32.IsWindowVisible(hwnd):
            found.append(hwnd)
        return True

    user32.EnumWindows(enum_proc(cb), 0)
    if not found:
        return None
    user32.GetWindowTextLengthW.restype = ctypes.c_int
    for hwnd in found:  # prefer a titled window over hidden helper surfaces
        if user32.GetWindowTextLengthW(hwnd) > 0:
            return hwnd
    return found[0]


def is_hung(hwnd):
    """True when the window thread has stopped pumping messages."""
    user32 = ctypes.WinDLL("user32", use_last_error=True)
    return bool(user32.IsHungAppWindow(hwnd))


def app_already_running():
    out = subprocess.run(
        ["tasklist", "/FI", "IMAGENAME eq VideoTools.exe"],
        capture_output=True, text=True, creationflags=subprocess.CREATE_NO_WINDOW,
    )
    return "VideoTools.exe" in (out.stdout or "")


def kill_tree(pid):
    subprocess.run(["taskkill", "/PID", str(pid), "/T", "/F"],
                   capture_output=True, creationflags=subprocess.CREATE_NO_WINDOW)


def process_snapshot(pid):
    ps = (
        "Get-Process -Id %d -ErrorAction SilentlyContinue | "
        "Select-Object Id,ProcessName,CPU,WorkingSet64,Handles,"
        "Threads,Responding,StartTime | Format-List | Out-String" % pid
    )
    r = subprocess.run(["powershell", "-NoProfile", "-Command", ps],
                       capture_output=True, text=True)
    return r.stdout.strip() or "(process not found)"


def run_verify(repo_root):
    script = repo_root / "scripts" / "windows" / "dev-verify.ps1"
    print("[verify] running dev-verify.ps1 ...")
    r = subprocess.run(["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
                        "-File", str(script)], cwd=str(repo_root))
    if r.returncode != 0:
        print("[verify] FAILED (exit %d) — build gate closed, not launching"
              % r.returncode)
        return False
    print("[verify] OK")
    return True


def monitor(proc, hwnd, warmup, stabilize, poll):
    """Observe one launch. Returns (outcome, detail)."""
    # Phase 1: warmup — wait for the main window to materialise.
    deadline = time.time() + warmup
    while time.time() < deadline:
        if proc.poll() is not None:
            return EXITED, "exit code %s during warmup" % proc.returncode
        hwnd = find_main_hwnd(proc.pid)
        if hwnd:
            break
        time.sleep(poll)
    if not hwnd:
        if proc.poll() is not None:
            return EXITED, "exit code %s during warmup" % proc.returncode
        # Window may still be late; keep watching across the stabilize window.

    # Phase 2: observation — two consecutive hung polls = UI freeze.
    hung_streak = 0
    deadline = time.time() + stabilize
    while time.time() < deadline:
        if proc.poll() is not None:
            return EXITED, "exit code %s during observation" % proc.returncode
        if hwnd and is_hung(hwnd):
            hung_streak += 1
            if hung_streak >= 2:
                return HUNG, "main window stopped responding (IsHungAppWindow)"
        else:
            hung_streak = 0
            if not hwnd:
                hwnd = find_main_hwnd(proc.pid)
        time.sleep(poll)
    if not hwnd:
        return NOWIN, "no visible main window after warmup+stabilize"
    return STABLE, "responsive for %ds" % stabilize


def capture(run_dir, outcome, detail, proc):
    (run_dir / "meta.json").write_text(json.dumps({
        "timestamp": datetime.datetime.now().isoformat(timespec="seconds"),
        "outcome": outcome,
        "detail": detail,
        "pid": proc.pid,
        "exit_code": proc.poll(),
    }, indent=2), encoding="utf-8")
    if outcome != EXITED:  # still alive → grab a live process state
        (run_dir / "process-state.txt").write_text(
            process_snapshot(proc.pid), encoding="utf-8")


def main():
    args = parse_args()
    repo_root = Path(__file__).resolve().parents[1]

    if not args.exe.is_file():
        print("ERROR: executable not found: %s" % args.exe)
        return 2
    if app_already_running():
        print("ERROR: VideoTools.exe is already running — close it first "
              "so the harness owns a single process tree.")
        return 2
    if args.verify and not run_verify(repo_root):
        return 3

    HEAL_ROOT.mkdir(exist_ok=True)
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    print("heal budget: %d run(s) | warmup %ds | stabilize %ds"
          % (args.max_runs, args.warmup, args.stabilize))

    for run in range(1, args.max_runs + 1):
        run_dir = HEAL_ROOT / ("run-%03d-%s" % (run, stamp))
        run_dir.mkdir(parents=True, exist_ok=True)
        out = open(run_dir / "stdout.log", "wb")
        err = open(run_dir / "stderr.log", "wb")
        print("[run %d/%d] launching %s" % (run, args.max_runs, args.exe.name))
        try:
            proc = subprocess.Popen(
                [str(args.exe)], cwd=str(repo_root),
                stdout=out, stderr=err,
                creationflags=subprocess.CREATE_NEW_PROCESS_GROUP,
            )
            outcome, detail = monitor(proc, None, args.warmup,
                                      args.stabilize, args.poll)
        except OSError as exc:
            out.close()
            err.close()
            print("ERROR: launch failed: %s" % exc)
            return 2
        finally:
            out.close()
            err.close()

        print("[run %d/%d] %s — %s" % (run, args.max_runs, outcome, detail))
        if outcome == STABLE:
            print("STABLE: app left running (pid %d), diagnostics in %s"
                  % (proc.pid, run_dir))
            return 0

        capture(run_dir, outcome, detail, proc)
        if proc.poll() is None:
            kill_tree(proc.pid)
        print("[run %d/%d] diagnostics -> %s" % (run, args.max_runs, run_dir))

    print("EXHAUSTED: heal budget spent after %d run(s); yielding to operator. "
          "Diagnostics: %s" % (args.max_runs, HEAL_ROOT))
    return 1


if __name__ == "__main__":
    sys.exit(main())
