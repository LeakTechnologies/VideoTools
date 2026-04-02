package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func ValidateCRF(input string) error {
	if input == "" {
		return nil
	}
	val, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("CRF must be a number")
	}
	if val < 0 || val > 51 {
		return fmt.Errorf("CRF must be between 0 and 51")
	}
	return nil
}

func ValidateBitrate(input string, unit string) error {
	if input == "" {
		return nil
	}
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return fmt.Errorf("Bitrate must be a number")
	}
	if val <= 0 {
		return fmt.Errorf("Bitrate must be positive")
	}
	_ = unit // Reserved for future unit validation (Kbps/Mbps/Gbps)
	return nil
}

func ValidateFileSize(input string) error {
	if input == "" {
		return nil
	}
	val, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return fmt.Errorf("File size must be a number")
	}
	if val <= 0 {
		return fmt.Errorf("File size must be positive")
	}
	return nil
}

func IsStandardAspect(val string) bool {
	switch strings.TrimSpace(strings.ToLower(val)) {
	case "16:9", "4:3", "1:1", "9:16", "21:9":
		return true
	default:
		return false
	}
}

func NormalizeBitrateMode(mode string) string {
	switch {
	case strings.HasPrefix(mode, "CRF"):
		return "CRF"
	case strings.HasPrefix(mode, "CBR"):
		return "CBR"
	case strings.HasPrefix(mode, "VBR"):
		return "VBR"
	case strings.HasPrefix(mode, "Target Size"):
		return "Target Size"
	default:
		return mode
	}
}
