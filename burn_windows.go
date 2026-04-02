//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func detectOpticalDrives() []string {
	var drives []string

	// Get logical drives using GetLogicalDrives
	logicalDrives, err := windows.GetLogicalDrives()
	if err != nil {
		return drives
	}

	// Check each drive letter (D-Z) for CD-ROM
	for i := 2; i < 26; i++ { // Start from D:
		if logicalDrives&(1<<i) == 0 {
			continue
		}

		driveLetter := string(rune('A' + i))
		drivePath := driveLetter + ":"

		// Check drive type
		driveType := windows.GetDriveType(windows.StringToUTF16Ptr(drivePath))
		if driveType == windows.DRIVE_CDROM {
			drives = append(drives, drivePath)
		}
	}

	return drives
}

func getDriveInfo(drive string) (name, capacity string, err error) {
	drivePath := windows.StringToUTF16Ptr(drive + "\\")

	// Get volume info
	var volumeName [256]uint16
	windows.GetVolumeInformation(drivePath, &volumeName, nil, nil, nil)
	name = windows.UTF16ToString(volumeName[:])

	if name == "" {
		name = drive
	}

	// Get disk free space
	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes int64
	windows.GetDiskFreeSpaceEx(drivePath, &freeBytesAvailable, &totalNumberOfBytes, &totalNumberOfFreeBytes)

	gb := float64(totalNumberOfBytes) / (1024 * 1024 * 1024)
	capacity = fmt.Sprintf("%.1f GB", gb)

	return name, capacity, nil
}

func burnISO(isoPath, drive string, speed string, eject bool) error {
	// Use IMAPI2 COM interface for burning
	// This requires proper COM initialization and interfaces
	return fmt.Errorf("burn not implemented: use IMAPI2 COM")
}

func ejectDisc(drive string) error {
	// Use SetVolumeMountPoint to eject
	drivePtr := windows.StringToUTF16Ptr(drive + "\\")
	return windows.SetVolumeMountPoint(drivePtr, nil)
}

type IMAPIDiscMaster interface {
	QueryInterface(riid *windows.GUID, ppv *unsafe.Pointer) error
	AddRef() uint32
	Release() uint32
}

type IDiscRecorder2 interface {
	QueryInterface(riid *windows.GUID, ppv *unsafe.Pointer) error
	AddRef() uint32
	Release() uint32
	Open(driveLetter string) error
	Close() error
}

type IStream interface {
	QueryInterface(riid *windows.GUID, ppv *unsafe.Pointer) error
	AddRef() uint32
	Release() uint32
	Read(pv *byte, cb int32, pcbRead *int32) error
	Write(pv *byte, cb int32, pcbWritten *int32) error
}

// writeToDisc uses IMAPI2 to write an ISO to disc
func writeToDisc(isoPath, drive string) error {
	// This is a placeholder for the actual IMAPI2 implementation
	// Would require:
	// 1. CoInitializeEx(nil, COINIT_APARTMENTTHREADED)
	// 2. CoCreateInstance(CLSID_DiscMaster, ..., IID_IMAPI_Disc_Recorder, ...)
	// 3. Open the recorder with drive letter
	// 4. Create an IStream from the ISO file
	// 5. Write using IDiscRecorder2::Write
	// 6. Close and release interfaces

	return fmt.Errorf("IMAPI2 implementation not yet complete")
}

func init() {
	// Register burn as available if we can detect drives
	if drives := detectOpticalDrives(); len(drives) > 0 {
		fmt.Println("Optical drives detected:", drives)
	}
}

func getISOSize(isoPath string) (int64, error) {
	info, err := os.Stat(isoPath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func createISO(isoPath, sourcePath string) error {
	// Create ISO from source path using ISO9660/UDF
	return fmt.Errorf("ISO creation not implemented")
}
