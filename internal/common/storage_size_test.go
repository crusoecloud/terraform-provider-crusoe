package common

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
			result := PreserveSizeFormat(tt.userFormat, tt.apiSize)
			if result != tt.expected {
				t.Errorf("PreserveSizeFormat(%q, %q) = %q, want %q",
					tt.userFormat, tt.apiSize, result, tt.expected)
			}
		})
	}
}

func TestStorageSizeInGiB(t *testing.T) {
	tests := []struct {
		size string
		want int
		ok   bool
	}{
		{size: "100GiB", want: 100, ok: true},
		{size: "20TiB", want: 20480, ok: true},
		{size: "100MiB", ok: false},
		{size: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			got, ok := StorageSizeInGiB(tt.size)
			if ok != tt.ok {
				t.Fatalf("StorageSizeInGiB(%q) ok = %v, want %v", tt.size, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("StorageSizeInGiB(%q) = %d, want %d", tt.size, got, tt.want)
			}
		})
	}
}
