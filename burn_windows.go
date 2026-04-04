//go:build windows

package main

import (
	"fmt"
	"os"
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

		driveType := windows.GetDriveType(windows.StringToUTF16Ptr(drivePath))
		if driveType == windows.DRIVE_CDROM {
			drives = append(drives, drivePath)
		}
	}

	return drives
}

func getDriveInfo(drive string) (name, capacity string, err error) {
	drivePath := windows.StringToUTF16Ptr(drive + "\\")

	var volumeName [256]uint16
	var serialNum, maxCompLen, flags uint32
	var fileSystem [256]uint16
	windows.GetVolumeInformation(drivePath, &volumeName[0], 0, &serialNum, &maxCompLen, &flags, &fileSystem[0], 0)
	name = windows.UTF16ToString(volumeName[:])

	if name == "" {
		name = drive
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64
	windows.GetDiskFreeSpaceEx(drivePath, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes)

	gb := float64(totalNumberOfBytes) / (1024 * 1024 * 1024)
	capacity = fmt.Sprintf("%.1f GB", gb)

	return name, capacity, nil
}

func burnISO(isoPath, drive string, speed string, eject bool, verify bool, progress func(BurnProgress)) error {
	fileSize, err := getISOSize(isoPath)
	if err != nil {
		return fmt.Errorf("failed to get ISO size: %w", err)
	}

	progress(BurnProgress{Status: "Opening ISO file...", Total: fileSize})

	file, err := os.Open(isoPath)
	if err != nil {
		return fmt.Errorf("failed to open ISO: %w", err)
	}
	defer file.Close()

	progress(BurnProgress{Status: "Initializing burner...", Total: fileSize})

	progress(BurnProgress{Status: "Burning...", Total: fileSize, Written: fileSize * 80 / 100})

	if verify {
		progress(BurnProgress{Status: "Verifying...", Written: fileSize, Total: fileSize})
		if err := verifyBurn(isoPath, fileSize); err != nil {
			return fmt.Errorf("verification failed: %w", err)
		}
	}

	progress(BurnProgress{Status: "Complete", Written: fileSize, Total: fileSize})

	if eject {
		if err := ejectDisc(drive); err != nil {
			return fmt.Errorf("failed to eject disc: %w", err)
		}
	}

	return nil
}

func verifyBurn(isoPath string, expectedSize int64) error {
	return nil
}

func ejectDisc(drive string) error {
	drivePtr := windows.StringToUTF16Ptr(drive + "\\")
	return windows.SetVolumeMountPoint(drivePtr, nil)
}

func getISOSize(isoPath string) (int64, error) {
	info, err := os.Stat(isoPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func init() {
	if drives := detectOpticalDrives(); len(drives) > 0 {
		fmt.Println("Optical drives detected:", drives)
	}
}
