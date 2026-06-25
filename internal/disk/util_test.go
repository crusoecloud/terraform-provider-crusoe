package disk

import (
	"context"
	"reflect"
	"testing"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_diskToTerraformResourceModel covers the shared transform that all CRUD
// paths use: VIPs are sorted deterministically (the CCX-4394 ordering guarantee
// the Update path previously bypassed), size is rendered in the user's unit, and
// serial_number/dns_name/block_size are sourced from the API response.
func Test_diskToTerraformResourceModel(t *testing.T) {
	state := &diskResourceModel{}
	disk := &swagger.DiskV1{
		Id:           "disk-1",
		Name:         "my-disk",
		Location:     "us-east1-a",
		Type_:        "shared-volume",
		Size:         "1024GiB",
		SerialNumber: "SN123",
		BlockSize:    4096,
		DnsName:      "my-disk.dns",
		Vips:         []string{"10.0.0.3", "10.0.0.1", "10.0.0.2"},
	}

	diskToTerraformResourceModel(disk, state, "1TiB")

	if got := state.ID.ValueString(); got != "disk-1" {
		t.Errorf("id = %q, want %q", got, "disk-1")
	}
	if got := state.SerialNumber.ValueString(); got != "SN123" {
		t.Errorf("serial_number = %q, want %q (from API)", got, "SN123")
	}
	if got := state.DNSName.ValueString(); got != "my-disk.dns" {
		t.Errorf("dns_name = %q, want %q (from API)", got, "my-disk.dns")
	}
	if got := state.BlockSize.ValueInt64(); got != 4096 {
		t.Errorf("block_size = %d, want %d (from API)", got, 4096)
	}
	if got := state.Size.ValueString(); got != "1TiB" {
		t.Errorf("size = %q, want %q (preserved in user's unit)", got, "1TiB")
	}

	var gotVips []string
	if diags := state.Vips.ElementsAs(context.Background(), &gotVips, false); diags.HasError() {
		t.Fatalf("reading vips: %v", diags)
	}
	wantVips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}
	if !reflect.DeepEqual(gotVips, wantVips) {
		t.Errorf("vips = %v, want %v (sorted)", gotVips, wantVips)
	}
}

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
