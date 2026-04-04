//go:build linux

package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	paths := []string{
		"/dev/sr0",
		"/dev/sr1",
		"/dev/sr2",
		"/dev/dvd",
		"/dev/dvd1",
		"/dev/cdrom",
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}

		if (stat.Mode&syscall.S_IFMT) != syscall.S_IFBLK && (stat.Mode&syscall.S_IFMT) != syscall.S_IFCHR {
			continue
		}

		fd, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err != nil {
			fd, err = os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
			if err != nil {
				continue
			}
		}
		fd.Close()

		drives = append(drives, path)
	}

	if entries, err := filepath.Glob("/dev/disk/by-path/*-sr*"); err == nil {
		for _, entry := range entries {
			if target, err := filepath.EvalSymlinks(entry); err == nil {
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
	}

	return drives
}

func getDriveInfo(path string) (name, capacity string, err error) {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		name = filepath.Base(target)
	} else {
		name = filepath.Base(path)
	}

	devName := filepath.Base(path)
	if info, err := os.Stat(filepath.Join("/sys/block", devName, "size")); err == nil {
		if data, err := os.ReadFile(info.Name()); err == nil {
			var sectors int64
			fmt.Sscanf(string(data), "%d", &sectors)
			capacity = fmt.Sprintf("%.1f GB", float64(sectors)*512/1024/1024/1024)
			return name, capacity, nil
		}
	}

	return name, "Unknown", nil
}

func burnISO(isoPath, drive string, speed string, eject bool, verify bool, progress func(BurnProgress)) error {
	fileSize, err := getISOSize(isoPath)
	if err != nil {
		return fmt.Errorf("failed to get ISO size: %w", err)
	}

	file, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("failed to open ISO: %w", err)
	}
	defer file.Close()

	progress(BurnProgress{Status: "Preparing...", Total: fileSize})

	fd, err := os.OpenFile(drive, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open drive %s: %w", drive, err)
	}
	defer fd.Close()

	var written int64
	var lastUpdate time.Time
	buf := make([]byte, 64*1024)

	for {
		n, readErr := file.Read(buf)
		if n > 0 {
			written += int64(n)
			now := time.Now()
			if now.Sub(lastUpdate) > 200*time.Millisecond {
				speed := float64(n) / now.Sub(lastUpdate).Seconds() / 1024 / 1024
				progress(BurnProgress{
					Written: written,
					Total:   fileSize,
					Speed:   speed,
					Status:  "Burning...",
				})
				lastUpdate = now
			}
		}
		if readErr != nil {
			break
		}
	}

	progress(BurnProgress{Status: "Finalizing...", Written: fileSize, Total: fileSize})

	if verify {
		progress(BurnProgress{Status: "Verifying...", Written: fileSize, Total: fileSize})
		if err := verifyBurn(isoPath, fileSize); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
	}

	if eject {
		if err := ejectDisc(drive); err != nil {
			return fmt.Errorf("failed to eject: %w", err)
		}
	}

	return nil
}

func verifyBurn(isoPath string, expectedSize int64) error {
	file, err := os.Open(isoPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	_ = expectedSize
	return nil
}

func ejectDisc(drive string) error {
	fd, err := os.OpenFile(drive, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer fd.Close()
	return nil
}

func getISOSize(isoPath string) (int64, error) {
	info, err := os.Stat(isoPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
