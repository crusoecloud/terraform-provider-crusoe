package disk

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_diskToTerraformResourceModel covers the shared transform that all CRUD
// paths use: VIPs are sorted deterministically (the CCX-4394 ordering guarantee
// the Update path previously bypassed), size is rendered in the user's unit,
// serial_number/dns_name are sourced from the API response, and the deprecated
// block_size is intentionally left unset so callers can preserve it (CCX-3067).
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
	if !state.BlockSize.IsNull() {
		t.Errorf("block_size = %v, want null (deprecated field must not be sourced from the API)", state.BlockSize)
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

// Test_preserveDeprecatedBlockSize verifies the deprecated block_size value is
// echoed back from the plan/state when the user set it (so Terraform's Computed
// consistency holds and the disk is never spuriously replaced), and falls back to
// the API value only when unset (CCX-3067).
func Test_preserveDeprecatedBlockSize(t *testing.T) {
	const apiBlockSize int64 = 512

	tests := []struct {
		name    string
		planned types.Int64
		want    int64
	}{
		{
			name:    "user-configured value is preserved over the API value",
			planned: types.Int64Value(4096),
			want:    4096,
		},
		{
			name:    "user-configured value equal to API value is preserved",
			planned: types.Int64Value(512),
			want:    512,
		},
		{
			name:    "null (unset) falls back to the API value",
			planned: types.Int64Null(),
			want:    apiBlockSize,
		},
		{
			name:    "unknown (computed on create) falls back to the API value",
			planned: types.Int64Unknown(),
			want:    apiBlockSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preserveDeprecatedBlockSize(tt.planned, apiBlockSize)
			if got.IsNull() || got.IsUnknown() {
				t.Fatalf("preserveDeprecatedBlockSize returned a null/unknown value: %v", got)
			}
			if got.ValueInt64() != tt.want {
				t.Errorf("preserveDeprecatedBlockSize(%v, %d) = %d, want %d",
					tt.planned, apiBlockSize, got.ValueInt64(), tt.want)
			}
		})
	}
}
