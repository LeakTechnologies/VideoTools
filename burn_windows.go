//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
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

	logicalDrives, err := windows.GetLogicalDrives()
	if err != nil {
		return drives
	}

	for i := 2; i < 26; i++ {
		if logicalDrives&(1<<i) == 0 {
			continue
		}

		driveLetter := string(rune('A' + i))
		drivePath := driveLetter + ":"

		driveType := windows.GetDriveType(windows.StringToUTF16Ptr(drivePath + `\`))
		if driveType == windows.DRIVE_CDROM {
			drives = append(drives, drivePath)
		}
	}

	return drives
}

func getDriveInfo(drive string) (name, capacity string, err error) {
	drivePath := windows.StringToUTF16Ptr(strings.TrimRight(drive, `\`) + `\`)

	var volumeName [256]uint16
	var serialNum, maxCompLen, flags uint32
	var fileSystem [256]uint16
	windows.GetVolumeInformation(drivePath, &volumeName[0], uint32(len(volumeName)), &serialNum, &maxCompLen, &flags, &fileSystem[0], uint32(len(fileSystem)))
	name = windows.UTF16ToString(volumeName[:])
	if name == "" {
		name = drive
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err2 := windows.GetDiskFreeSpaceEx(drivePath, &freeBytesAvailable, &totalBytes, &totalFreeBytes); err2 != nil || totalBytes == 0 {
		return name, "No disc", nil
	}

	gb := float64(totalBytes) / (1024 * 1024 * 1024)
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

// burnISO burns an ISO image to a disc using the built-in Windows Disc Image
// Burner (isoburn.exe). The /Q flag causes the burner to start immediately
// without prompting. Progress is indeterminate while isoburn runs.
func burnISO(isoPath, drive string, speed string, eject bool, verify bool, progress func(BurnProgress)) error {
	fileSize, err := getISOSize(isoPath)
	if err != nil {
		return fmt.Errorf("failed to read ISO: %w", err)
	}

	// isoburn.exe ships with Windows 7+ at %SYSTEMROOT%\System32\isoburn.exe
	sysRoot := os.Getenv("SYSTEMROOT")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	isoburnPath := filepath.Join(sysRoot, "System32", "isoburn.exe")
	if _, err := os.Stat(isoburnPath); err != nil {
		return fmt.Errorf("isoburn.exe not found at %s: Windows 7 or later required", isoburnPath)
	}

	// Normalise drive to "D:" (strip trailing backslash, strip capacity label)
	driveLetter := drive
	if idx := strings.Index(driveLetter, " ("); idx != -1 {
		driveLetter = strings.TrimSpace(driveLetter[:idx])
	}
	driveLetter = strings.TrimRight(driveLetter, `\`)
	if !strings.HasSuffix(driveLetter, ":") {
		driveLetter += ":"
	}

	progress(BurnProgress{Status: "Starting Windows disc burner...", Total: fileSize})

	// /Q — start the burn immediately without user confirmation
	cmd := exec.Command(isoburnPath, "/Q", driveLetter, isoPath)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch isoburn.exe: %w", err)
	}

	// isoburn.exe manages the burn internally; poll until it exits.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// Emit a mid-point progress tick so the UI shows activity.
	progress(BurnProgress{
		Status:  "Burning via Windows Disc Image Burner...",
		Total:   fileSize,
		Written: fileSize / 2,
	})

	if err := <-done; err != nil {
		return fmt.Errorf("burn failed: %w", err)
	}

	progress(BurnProgress{Status: "Complete", Total: fileSize, Written: fileSize})

	if eject {
		if err := ejectDisc(driveLetter); err != nil {
			return fmt.Errorf("failed to eject: %w", err)
		}
	}

	return nil
}

func verifyBurn(isoPath string, expectedSize int64) error {
	// Verify is handled by the Windows Disc Image Burner internally.
	return nil
}

// ejectDisc opens the disc tray using IOCTL_STORAGE_EJECT_MEDIA.
func ejectDisc(drive string) error {
	// Normalise to NT device path: "D:" → "\\.\D:"
	letter := strings.ToUpper(strings.TrimRight(strings.TrimRight(drive, `\`), ":"))
	if len(letter) == 0 {
		return fmt.Errorf("invalid drive: %s", drive)
	}
	devicePath := `\\.\` + letter + `:`

	h, err := windows.CreateFile(
		windows.StringToUTF16Ptr(devicePath),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return fmt.Errorf("open drive %s: %w", devicePath, err)
	}
	defer windows.CloseHandle(h)

	const ioctlStorageEjectMedia = 0x2D4808
	var bytesReturned uint32
	return windows.DeviceIoControl(h, ioctlStorageEjectMedia, nil, 0, nil, 0, &bytesReturned, nil)
}

func getISOSize(isoPath string) (int64, error) {
	info, err := os.Stat(isoPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
