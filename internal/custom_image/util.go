package custom_image

import (
	"sort"
	"strconv"
	"strings"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

// customImagesToTerraformDataModel takes the API response and applies name/name_prefix filters.
// Images that match the filters are converted to the terraform data model and returned as a slice.
func filterCustomImagesListResponse(resp *swagger.ListImagesResponseV1Alpha5, config customImageDataSourceModel) []customImageModel {
	var filtered []customImageModel

	for _, image := range resp.Items {
		// Apply name filter (exact match)
		if config.Name != nil && *config.Name != "" {
			if image.Name != *config.Name {
				continue
			}
		}

		// Apply name_prefix filter
		if config.NamePrefix != nil && *config.NamePrefix != "" {
			if !strings.HasPrefix(image.Name, *config.NamePrefix) {
				continue
			}
		}

		// Convert API response to terraform data model
		filtered = append(filtered, customImageModel{
			ID:          image.Id,
			Name:        image.Name,
			Description: image.Description,
			Locations:   image.Locations,
			Tags:        image.Tags,
			CreatedAt:   image.CreatedAt,
		})
	}

	return filtered
}

// findNewestImage returns the image with the largest numeric suffix in its name from a list of images.
// (If images do not share a common prefix or their numeric suffixes are not found, then it returns the
// image with the lexicographically "largest" name.)
func findNewestImage(images []customImageModel) *customImageModel {
	switch len(images) {
	case 0:
		return nil
	case 1:
		return &images[0]
	default:
		sortedImages := make([]customImageModel, len(images))
		copy(sortedImages, images)
		sort.Slice(sortedImages, func(i, j int) bool {
			// Use compareImages to sort in _descending_ order
			return compareImages(&sortedImages[i], &sortedImages[j]) > 0
		})

		return &sortedImages[0]
	}
}

func compareImages(image1, image2 *customImageModel) int {
	return compareImageNames(image1.Name, image2.Name)
}

// compareImageNames compares two image names and returns a sort comparator.
//
// If names share a common prefix and their suffixes are numeric, those suffixes are assumed to represent
// the image version and are compared numerically.
//
// If names do not share a common prefix or their suffixes are not numeric, then names are compared
// lexicographically:
//
// -1 if name1 < name2
//
//	0 if name1 == name2
//
// +1 if name1 > name2
func compareImageNames(name1, name2 string) int {
	// Split names into parts
	parts1 := strings.Split(name1, "-")
	parts2 := strings.Split(name2, "-")

	// Find the common prefix
	commonPrefix := ""
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] == parts2[i] {
			commonPrefix += parts1[i]
			if i < len(parts1)-1 {
				commonPrefix += "-"
			}
		} else {
			break
		}
	}

	// If names have the same prefix, compare the numeric suffixes
	if commonPrefix != "" && strings.HasPrefix(name1, commonPrefix) && strings.HasPrefix(name2, commonPrefix) {
		suffix1 := strings.TrimPrefix(name1, commonPrefix)
		suffix2 := strings.TrimPrefix(name2, commonPrefix)

		// Try to parse as integers
		num1, err1 := strconv.Atoi(suffix1)
		num2, err2 := strconv.Atoi(suffix2)

		switch {
		case err1 != nil || err2 != nil:
			// At least one is not a valid number, cannot compare numerically
			break
		case num1 < num2:
			return -1
		case num1 > num2:
			return 1
		default:
			return 0
		}
	}

	// Fall back to string comparison
	switch {
	case name1 < name2:
		return -1
	case name1 > name2:
		return 1
	default:
		return 0
	}
}
