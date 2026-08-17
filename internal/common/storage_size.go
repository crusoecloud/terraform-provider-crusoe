package common

import (
	"strconv"
	"strings"
)

const (
	gibInTib = 1024
	unitLen  = 3 // "GiB" / "TiB"
)

// StorageSizeInGiB parses "100GiB" / "1TiB" into GiB. ok=false if unrecognized.
func StorageSizeInGiB(size string) (gib int, ok bool) {
	if len(size) <= unitLen {
		return 0, false
	}

	n, err := strconv.Atoi(size[:len(size)-unitLen])
	if err != nil {
		return 0, false
	}

	switch strings.ToLower(size[len(size)-unitLen:]) {
	case "gib":
		return n, true
	case "tib":
		return n * gibInTib, true
	default:
		return 0, false
	}
}

// PreserveSizeFormat renders apiSize in userFormat's unit when equivalent.
func PreserveSizeFormat(userFormat, apiSize string) string {
	if len(userFormat) <= unitLen || len(apiSize) <= unitLen {
		return apiSize
	}

	userUnit := strings.ToLower(userFormat[len(userFormat)-unitLen:])
	apiUnit := strings.ToLower(apiSize[len(apiSize)-unitLen:])

	// Already same unit
	if userUnit == apiUnit {
		return apiSize
	}

	// User wants TiB, API returned GiB → convert if evenly divisible
	if userUnit == "tib" && apiUnit == "gib" {
		if gib, err := strconv.Atoi(apiSize[:len(apiSize)-unitLen]); err == nil &&
			gib >= gibInTib && gib%gibInTib == 0 {

			return strconv.Itoa(gib/gibInTib) + "TiB"
		}
	}

	// User wants GiB, API returned TiB → convert
	if userUnit == "gib" && apiUnit == "tib" {
		if tib, err := strconv.Atoi(apiSize[:len(apiSize)-unitLen]); err == nil {
			return strconv.Itoa(tib*gibInTib) + "GiB"
		}
	}

	return apiSize
}
