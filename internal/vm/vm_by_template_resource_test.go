package vm

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// TestVMByTemplateImportState exercises the ImportState path end-to-end: it
// confirms the parsed identifiers land in the "id" and "project_id" schema
// attributes, that the client project ID is used as a fallback, and that an
// invalid identifier surfaces a diagnostic. The parsing logic itself is covered
// by TestParseResourceIdentifiers in the common package.
func TestVMByTemplateImportState(t *testing.T) {
	const (
		resourceUUID = "11111111-1111-1111-1111-111111111111"
		projectUUID  = "22222222-2222-2222-2222-222222222222"
		fallbackUUID = "33333333-3333-3333-3333-333333333333"
	)

	ctx := context.Background()

	t.Run("explicit project id from suffix", func(t *testing.T) {
		r := &vmByTemplateResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: resourceUUID + "," + projectUUID}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}

		var gotID, gotProject types.String
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &gotID)...)
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("project_id"), &gotProject)...)
		if resp.Diagnostics.HasError() {
			t.Fatalf("failed reading state attributes: %v", resp.Diagnostics)
		}

		if gotID.ValueString() != resourceUUID {
			t.Errorf("id = %q, want %q", gotID.ValueString(), resourceUUID)
		}
		if gotProject.ValueString() != projectUUID {
			t.Errorf("project_id = %q, want %q", gotProject.ValueString(), projectUUID)
		}
	})

	t.Run("falls back to client project id", func(t *testing.T) {
		r := &vmByTemplateResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: resourceUUID}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}

		var gotProject types.String
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("project_id"), &gotProject)...)
		if resp.Diagnostics.HasError() {
			t.Fatalf("failed reading project_id: %v", resp.Diagnostics)
		}
		if gotProject.ValueString() != fallbackUUID {
			t.Errorf("project_id = %q, want fallback %q", gotProject.ValueString(), fallbackUUID)
		}
	})

	t.Run("invalid identifier surfaces diagnostic", func(t *testing.T) {
		r := &vmByTemplateResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: "not-a-uuid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Error("expected diagnostics error for invalid identifier, got none")
		}
	})
}

// TestVMByTemplateDisksToTerraformResourceModel makes sure that Create and Read write the
// same disks: every attached disk, the OS disk included, with the API attachment_type.
func TestVMByTemplateDisksToTerraformResourceModel(t *testing.T) {
	ctx := context.Background()

	t.Run("keeps every disk with its attachment type", func(t *testing.T) {
		disks, diags := vmByTemplateDisksToTerraformResourceModel([]swagger.AttachedDiskV1{
			{Id: "os-disk", AttachmentType: DiskOS, Mode: "read-write"},
			{Id: "data-disk", AttachmentType: "data", Mode: "read-only"},
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}

		var got []vmDiskResourceModel
		if diags := disks.ElementsAs(ctx, &got, false); diags.HasError() {
			t.Fatalf("failed reading disks: %v", diags)
		}

		want := map[string]vmDiskResourceModel{
			"os-disk":   {ID: "os-disk", AttachmentType: DiskOS, Mode: "read-write"},
			"data-disk": {ID: "data-disk", AttachmentType: "data", Mode: "read-only"},
		}
		if len(got) != len(want) {
			t.Fatalf("got %d disks, want %d", len(got), len(want))
		}
		for _, disk := range got {
			if disk != want[disk.ID] {
				t.Errorf("disk %q = %+v, want %+v", disk.ID, disk, want[disk.ID])
			}
		}
	})

	t.Run("no disks gives a null set", func(t *testing.T) {
		disks, diags := vmByTemplateDisksToTerraformResourceModel(nil)
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics: %v", diags)
		}
		if !disks.IsNull() {
			t.Errorf("disks = %v, want null", disks)
		}
	})
}
