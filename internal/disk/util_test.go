package disk

import "testing"

func TestPreserveSizeFormat(t *testing.T) {
	tests := []struct {
		name       string
		userFormat string
		apiSize    string
		expected   string
	}{
		{
			name:       "user wants TiB, API returns GiB (divisible)",
			userFormat: "1TiB",
			apiSize:    "1024GiB",
			expected:   "1TiB",
		},
		{
			name:       "user wants TiB, API returns GiB (2TiB)",
			userFormat: "2TiB",
			apiSize:    "2048GiB",
			expected:   "2TiB",
		},
		{
			name:       "user wants GiB, API returns TiB",
			userFormat: "1024GiB",
			apiSize:    "1TiB",
			expected:   "1024GiB",
		},
		{
			name:       "user wants GiB, API returns TiB (2TiB)",
			userFormat: "2048GiB",
			apiSize:    "2TiB",
			expected:   "2048GiB",
		},
		{
			name:       "same unit GiB",
			userFormat: "500GiB",
			apiSize:    "500GiB",
			expected:   "500GiB",
		},
		{
			name:       "same unit TiB",
			userFormat: "1TiB",
			apiSize:    "1TiB",
			expected:   "1TiB",
		},
		{
			name:       "user wants TiB, API returns GiB (not divisible)",
			userFormat: "1TiB",
			apiSize:    "500GiB",
			expected:   "500GiB",
		},
		{
			name:       "user wants TiB, API returns GiB (less than 1TiB)",
			userFormat: "1TiB",
			apiSize:    "512GiB",
			expected:   "512GiB",
		},
		{
			name:       "empty user format returns API size",
			userFormat: "",
			apiSize:    "1024GiB",
			expected:   "1024GiB",
		},
		{
			name:       "case insensitive user format (tib)",
			userFormat: "1tib",
			apiSize:    "1024GiB",
			expected:   "1TiB",
		},
		{
			name:       "case insensitive user format (TIB)",
			userFormat: "1TIB",
			apiSize:    "1024gib",
			expected:   "1TiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preserveSizeFormat(tt.userFormat, tt.apiSize)
			if result != tt.expected {
				t.Errorf("preserveSizeFormat(%q, %q) = %q, want %q",
					tt.userFormat, tt.apiSize, result, tt.expected)
			}
		})
	}
}
