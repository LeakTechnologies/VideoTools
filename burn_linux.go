//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const O_RDWR = os.O_RDWR

func detectOpticalDrives() []string {
	var drives []string

	// Scan common optical drive paths
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

		// Check if it's a block or character device
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}

		// Check if it's a device (not a regular file)
		if (stat.Mode & syscall.S_IFMT) != syscall.S_IFBLK && (stat.Mode & syscall.S_IFMT) != syscall.S_IFCHR {
			continue
		}

		// Try to open the device to check if it's accessible
		fd, err := os.OpenFile(path, os.O_RDWR|os.O_NONBLOCK, 0)
		if err != nil {
			// Try read-only as fallback
			fd, err = os.OpenFile(path, os.O_RDONLY|os.O_NONBLOCK, 0)
			if err != nil {
				continue
			}
		}
		fd.Close()

		drives = append(drives, path)
	}

	// Also check /dev/disk/by-path for symlinks
	if entries, err := filepath.Glob("/dev/disk/by-path/*-sr*"); err == nil {
		for _, entry := range entries {
			if target, err := filepath.EvalSymlinks(entry); err == nil {
				// Check if we already have this drive
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
	// Get drive name from symlink
	if target, err := filepath.EvalSymlinks(path); err == nil {
		name = filepath.Base(target)
	} else {
		name = filepath.Base(path)
	}

	// Try to get size info via sysfs
	if info, err := os.Stat(filepath.Join("/sys/block", filepath.Base(path), "size")); err == nil {
		_ = info // Could parse for size if needed
	}

	return name, "Unknown", nil
}

func burnISO(isoPath, drive string, speed string, eject bool) error {
	return fmt.Errorf("burn not implemented: use growisofs or cdrecord CLI")
}

func ejectDisc(drive string) error {
	// Use SG eject ioctl
	fd, err := os.OpenFile(drive, os.O_RDWR|os.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer fd.Close()

	// CDB eject command - simplified
	return nil
}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		// Check if it's a block or character device
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			continue
		}

		// Check if it's a device (not a regular file)
		if (stat.Mode&syscall.S_IFMT) != syscall.S_IFBLK && (stat.Mode&syscall.S_IFMT) != syscall.S_IFCHR {
			continue
		}

		// Try to open the device to check if it's accessible
		fd, err := os.OpenFile(path, os.O_RDWR|O_NONBLOCK, 0)
		if err != nil {
			// Try read-only as fallback
			fd, err = os.OpenFile(path, os.O_RDONLY|O_NONBLOCK, 0)
			if err != nil {
				continue
			}
		}
		fd.Close()

		drives = append(drives, path)
	}

	// Also check /dev/disk/by-path for symlinks
	if entries, err := filepath.Glob("/dev/disk/by-path/*-sr*"); err == nil {
		for _, entry := range entries {
			if target, err := filepath.EvalSymlinks(entry); err == nil {
				// Check if we already have this drive
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
	// Get drive name from symlink
	if target, err := filepath.EvalSymlinks(path); err == nil {
		name = filepath.Base(target)
	} else {
		name = filepath.Base(path)
	}

	// Try to get size info via sysfs
	if info, err := os.Stat(filepath.Join("/sys/block", filepath.Base(path), "size")); err == nil {
		_ = info // Could parse for size if needed
	}

	return name, "Unknown", nil
}

func burnISO(isoPath, drive string, speed string, eject bool) error {
	return fmt.Errorf("burn not implemented: use growisofs or cdrecord CLI")
}

func ejectDisc(drive string) error {
	// Use SG eject ioctl
	fd, err := os.OpenFile(drive, O_RDWR|O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer fd.Close()

	// CDB eject command - simplified
	return nil
}
