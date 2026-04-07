// Package region provides DVD region code constants for authoring.
// Region codes control geographic playback restrictions on DVD-Video discs.
package region

// DVD Region codes as defined by DVD-Video specification.
// Multi-region discs OR these values together (e.g., Region1|Region2 = regions 1 & 2).
const (
	RegionFree = 0x00FFFFFE // Plays everywhere (default for archival)
	Region1    = 0x00000001 // USA, Canada, US Territories
	Region2    = 0x00000002 // Europe, Japan, South Africa, Middle East
	Region3    = 0x00000004 // Southeast Asia, East Asia
	Region4    = 0x00000008 // Latin America, Australia, New Zealand
	Region5    = 0x00000010 // Africa, Russia, North Korea, South Asia
	Region6    = 0x00000020 // China
	Region7    = 0x00000040 // Reserved (special use)
	Region8    = 0x00000080 // International venues (airlines, cruise ships)
)

// Common multi-region presets.
const (
	Region1And2     = Region1 | Region2           // North America + Europe
	Region2And4     = Region2 | Region4           // Europe + Australia
	Region1And2And4 = Region1 | Region2 | Region4 // Common distribution
	RegionAll       = RegionFree                  // Region-free (recommended)
)

// Category returns the VMG_Category value for a given region mask.
// The upper byte contains additional flags (0x00 for standard discs).
func Category(regionMask uint32) uint32 {
	// Upper byte is category flags (0x00 = standard), lower 24 bits are region
	return regionMask & 0x00FFFFFF
}

// RegionFreeCategory returns the region-free category value.
func RegionFreeCategory() uint32 {
	return Category(RegionFree)
}

// Regions returns a list of region numbers for a given mask.
func Regions(mask uint32) []int {
	regions := []int{}
	for i := 1; i <= 8; i++ {
		if mask&(1<<(i-1)) != 0 {
			regions = append(regions, i)
		}
	}
	return regions
}

// String returns a human-readable region description.
func String(mask uint32) string {
	if mask == RegionFree || mask == 0 {
		return "Region 0 (Region-free)"
	}

	regions := Regions(mask)
	if len(regions) == 8 {
		return "Region 0 (Region-free)"
	}

	result := "Regions "
	for i, r := range regions {
		if i > 0 {
			result += ", "
		}
		result += string(rune('0' + r))
	}
	return result
}
