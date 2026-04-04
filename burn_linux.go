//go:build linux

package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type BurnProgress struct {
	Written int64
	Total   int64
	Speed   float64
	ETA     time.Duration
	Status  string
}

func detectOpticalDrives() []string {
	var drives []string

	candidates := []string{
		"/dev/sr0", "/dev/sr1", "/dev/sr2", "/dev/sr3",
		"/dev/dvd", "/dev/dvd1", "/dev/cdrom", "/dev/cdrom1",
	}

	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}
		mode := stat.Mode & syscall.S_IFMT
		if mode != syscall.S_IFBLK && mode != syscall.S_IFCHR {
			continue
		}
		fd, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			continue
		}
		fd.Close()
		drives = append(drives, path)
	}

	// Add any symlink targets under /dev/disk/by-path that point to sr* devices.
	if entries, err := filepath.Glob("/dev/disk/by-path/*-sr*"); err == nil {
		for _, entry := range entries {
			target, err := filepath.EvalSymlinks(entry)
			if err != nil {
				continue
			}
			exists := false
			for _, d := range drives {
				if d == target {
					exists = true
					break
				}
			}
			if !exists {
				drives = append(drives, target)
			}
		}
	}

	return drives
}

func getDriveInfo(path string) (name, capacity string, err error) {
	devName := filepath.Base(path)
	if target, err2 := filepath.EvalSymlinks(path); err2 == nil {
		devName = filepath.Base(target)
		name = devName
	} else {
		name = devName
	}

	sizePath := filepath.Join("/sys/block", devName, "size")
	data, err2 := os.ReadFile(sizePath)
	if err2 != nil {
		return name, "No disc", nil
	}

	var sectors int64
	if _, err2 := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &sectors); err2 != nil || sectors == 0 {
		return name, "No disc", nil
	}

	// sysfs reports size in 512-byte logical blocks
	bytes := sectors * 512
	gb := float64(bytes) / (1024 * 1024 * 1024)
	switch {
	case gb > 22:
		capacity = fmt.Sprintf("BD-50 (%.1f GB)", gb)
	case gb > 20:
		capacity = fmt.Sprintf("BD-25 (%.1f GB)", gb)
	case gb > 7:
		capacity = fmt.Sprintf("DVD-9 (%.1f GB)", gb)
	case gb > 3:
		capacity = fmt.Sprintf("DVD-5 (%.1f GB)", gb)
	default:
		capacity = fmt.Sprintf("%.1f GB", gb)
	}

	return name, capacity, nil
}

// burnISO burns an ISO image using growisofs from the dvd+rw-tools package.
// Install on Debian/Ubuntu: sudo apt install dvd+rw-tools
// Install on Fedora/RHEL:   sudo dnf install dvd+rw-tools
// Install on Arch:          sudo pacman -S dvd+rw-tools
//
// growisofs line format on stderr:
//
//	/dev/sr0: 512/4693048 (0.0%), 55.6*1667KiB/s, ETA 00:22
func burnISO(isoPath, drive string, speed string, eject bool, verify bool, progress func(BurnProgress)) error {
	growisofs, err := exec.LookPath("growisofs")
	if err != nil {
		return fmt.Errorf("growisofs not found — install dvd+rw-tools (e.g. sudo apt install dvd+rw-tools)")
	}

	fileSize, err := getISOSize(isoPath)
	if err != nil {
		return fmt.Errorf("failed to read ISO: %w", err)
	}

	// Strip capacity label if the drive entry includes it, e.g. "/dev/sr0 (DVD-5)"
	driveDevice := drive
	if idx := strings.Index(driveDevice, " ("); idx != -1 {
		driveDevice = strings.TrimSpace(driveDevice[:idx])
	}

	// Build growisofs arguments.
	// -dvd-compat: finalises the disc so it plays on standalone players.
	// -Z device=isoPath: write ISO image to device (bypass track layout).
	args := []string{"-dvd-compat"}

	speedNum := parseSpeedArg(speed)
	if speedNum > 0 {
		args = append(args, fmt.Sprintf("-speed=%d", speedNum))
	}

	args = append(args, "-Z", fmt.Sprintf("%s=%s", driveDevice, isoPath))

	progress(BurnProgress{Status: "Preparing...", Total: fileSize})

	cmd := exec.Command(growisofs, args...)
	// growisofs writes progress to stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start growisofs: %w", err)
	}

	// Parse progress lines from stderr in a goroutine.
	done := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if p, ok := parseGrowisofsLine(line, fileSize); ok {
				progress(p)
			}
		}
	}()
	go func() { done <- cmd.Wait() }()

	if err := <-done; err != nil {
		return fmt.Errorf("burn failed: %w", err)
	}

	progress(BurnProgress{Status: "Finalizing...", Total: fileSize, Written: fileSize})

	if verify {
		progress(BurnProgress{Status: "Verifying...", Total: fileSize, Written: fileSize})
		if err := verifyBurn(isoPath, driveDevice, fileSize, progress); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
	}

	progress(BurnProgress{Status: "Complete", Total: fileSize, Written: fileSize})

	if eject {
		if err := ejectDisc(driveDevice); err != nil {
			return fmt.Errorf("eject failed: %w", err)
		}
	}

	return nil
}

// parseGrowisofsLine parses a growisofs progress line:
//
//	/dev/sr0: 1024/4693048 (0.0%), 55.6*1667KiB/s, ETA 00:22
func parseGrowisofsLine(line string, fileSize int64) (BurnProgress, bool) {
	// Find the colon separating device from stats
	colon := strings.Index(line, ": ")
	if colon < 0 {
		return BurnProgress{}, false
	}
	rest := strings.TrimSpace(line[colon+2:])

	// Expect: "written/total (pct%), speed*1667KiB/s, ETA hh:mm"
	// or just "written/total (pct%)"
	slashIdx := strings.Index(rest, "/")
	if slashIdx < 0 {
		return BurnProgress{}, false
	}
	writtenSectors, err := strconv.ParseInt(strings.TrimSpace(rest[:slashIdx]), 10, 64)
	if err != nil {
		return BurnProgress{}, false
	}

	rest = rest[slashIdx+1:]
	spaceIdx := strings.IndexAny(rest, " (")
	if spaceIdx < 0 {
		return BurnProgress{}, false
	}
	totalSectors, err := strconv.ParseInt(strings.TrimSpace(rest[:spaceIdx]), 10, 64)
	if err != nil || totalSectors == 0 {
		return BurnProgress{}, false
	}

	writtenBytes := writtenSectors * 2048
	totalBytes := totalSectors * 2048
	if fileSize > 0 {
		totalBytes = fileSize
	}

	// Parse speed (e.g. "55.6*1667KiB/s")
	var speedMBs float64
	if starIdx := strings.Index(rest, "*"); starIdx > 0 {
		speedStr := strings.TrimSpace(rest[spaceIdx+1:])
		if parenIdx := strings.Index(speedStr, "("); parenIdx >= 0 {
			speedStr = strings.TrimSpace(speedStr[parenIdx+1:])
			if closeIdx := strings.Index(speedStr, ")"); closeIdx > 0 {
				speedStr = speedStr[:closeIdx]
			}
		}
		if starIdx2 := strings.Index(speedStr, "*"); starIdx2 > 0 {
			multiplierStr := strings.TrimSpace(speedStr[:starIdx2])
			multiplier, _ := strconv.ParseFloat(multiplierStr, 64)
			// 1x DVD = 1385 KB/s; the factor after * indicates the base unit
			// growisofs uses *1667KiB/s (CD speed) or *1385KiB/s depending on disc
			unitStr := strings.TrimSpace(speedStr[starIdx2+1:])
			var baseKBs float64
			if strings.HasPrefix(unitStr, "1667") {
				baseKBs = 1667
			} else if strings.HasPrefix(unitStr, "1385") {
				baseKBs = 1385
			} else {
				baseKBs = 1385
			}
			speedMBs = (multiplier * baseKBs) / 1024
		}
	}

	// Parse ETA (e.g. "ETA 00:22" or "ETA 00:00:22")
	var eta time.Duration
	if etaIdx := strings.Index(rest, "ETA "); etaIdx >= 0 {
		etaStr := strings.TrimSpace(rest[etaIdx+4:])
		// Take up to first space/comma
		if end := strings.IndexAny(etaStr, " ,\t"); end > 0 {
			etaStr = etaStr[:end]
		}
		parts := strings.Split(etaStr, ":")
		switch len(parts) {
		case 2:
			min, _ := strconv.Atoi(parts[0])
			sec, _ := strconv.Atoi(parts[1])
			eta = time.Duration(min)*time.Minute + time.Duration(sec)*time.Second
		case 3:
			hr, _ := strconv.Atoi(parts[0])
			min, _ := strconv.Atoi(parts[1])
			sec, _ := strconv.Atoi(parts[2])
			eta = time.Duration(hr)*time.Hour + time.Duration(min)*time.Minute + time.Duration(sec)*time.Second
		}
	}

	return BurnProgress{
		Written: writtenBytes,
		Total:   totalBytes,
		Speed:   speedMBs,
		ETA:     eta,
		Status:  "Burning...",
	}, true
}

// parseSpeedArg converts a UI speed string to a growisofs integer speed factor.
// "Auto" or "" → 0 (omit flag, use default), "1x" → 1, "4x" → 4, etc.
func parseSpeedArg(speed string) int {
	speed = strings.TrimSpace(strings.ToLower(speed))
	if speed == "" || speed == "auto" {
		return 0
	}
	speed = strings.TrimSuffix(speed, "x")
	n, err := strconv.Atoi(speed)
	if err != nil {
		return 0
	}
	return n
}

// verifyBurn reads the disc back and compares its SHA-256 hash against the ISO.
func verifyBurn(isoPath, device string, fileSize int64, progress func(BurnProgress)) error {
	isoFile, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("open ISO: %w", err)
	}
	defer isoFile.Close()

	discFile, err := os.Open(device)
	if err != nil {
		return fmt.Errorf("open disc: %w", err)
	}
	defer discFile.Close()

	isoHash := sha256.New()
	discHash := sha256.New()

	buf := make([]byte, 256*1024)
	var read int64

	tee := io.TeeReader(io.LimitReader(discFile, fileSize), discHash)
	for {
		n, err := tee.Read(buf)
		if n > 0 {
			isoFile.Read(buf[:n]) // advance ISO reader in sync
			isoHash.Write(buf[:n])
			read += int64(n)
			progress(BurnProgress{
				Status:  "Verifying...",
				Written: read,
				Total:   fileSize,
			})
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read disc: %w", err)
		}
	}

	// Recompute ISO hash properly
	isoFile.Seek(0, io.SeekStart)
	isoHash2 := sha256.New()
	if _, err := io.Copy(isoHash2, isoFile); err != nil {
		return fmt.Errorf("hash ISO: %w", err)
	}

	if fmt.Sprintf("%x", discHash.Sum(nil)) != fmt.Sprintf("%x", isoHash2.Sum(nil)) {
		return fmt.Errorf("disc content does not match ISO (data may be corrupt)")
	}

	return nil
}

// ejectDisc sends CDROMEJECT ioctl to open the disc tray.
func ejectDisc(drive string) error {
	const CDROMEJECT = 0x5309

	fd, err := os.OpenFile(drive, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open drive: %w", err)
	}
	defer fd.Close()

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), CDROMEJECT, 0)
	if errno != 0 {
		return fmt.Errorf("eject ioctl: %w", errno)
	}
	return nil
}

func getISOSize(isoPath string) (int64, error) {
	info, err := os.Stat(isoPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
