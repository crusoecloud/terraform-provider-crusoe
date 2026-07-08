package instance_template

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// sampleAPITemplate returns an API instance template with the disk type populated
// and placement_policy plus the nullable fields empty, to exercise the transform's
// normalizations.
func sampleAPITemplate() *swagger.InstanceTemplate {
	return &swagger.InstanceTemplate{
		Id:                  "template-1",
		ProjectId:           "proj-1",
		Name:                "my-template",
		Type_:               "a100.1x",
		Location:            "us-east1-a",
		ImageName:           "ubuntu",
		SshPublicKey:        "ssh-ed25519 AAAA user@host",
		SubnetId:            "subnet-1",
		PublicIpAddressType: "dynamic",
		Disks:               []swagger.DiskTemplate{{Size: "100GiB", Type_: "persistent-ssd"}},
		// Empty values that must be normalized:
		PlacementPolicy: "",
		IbPartitionId:   "",
		StartupScript:   "",
		ShutdownScript:  "",
		NvlinkDomainId:  "",
	}
}

// Test_instanceTemplateToResourceModel covers the transform's field mapping: the
// disk type comes from the API response (not the create request), an empty
// placement_policy falls back to "unspecified", and the nullable fields are
// null-normalized.
func Test_instanceTemplateToResourceModel(t *testing.T) {
	var diags diag.Diagnostics
	model := &instanceTemplateResourceModel{}

	instanceTemplateToResourceModel(context.Background(), sampleAPITemplate(), model, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := model.ID.ValueString(); got != "template-1" {
		t.Errorf("id = %q, want %q", got, "template-1")
	}
	if got := model.Type.ValueString(); got != "a100.1x" {
		t.Errorf("type = %q, want %q (from API)", got, "a100.1x")
	}
	if got := model.PlacementPolicy.ValueString(); got != unspecifiedPlacementPolicy {
		t.Errorf("placement_policy = %q, want %q (empty API value falls back)", got, unspecifiedPlacementPolicy)
	}
	for name, v := range map[string]types.String{
		"ib_partition":     model.IBPartition,
		"startup_script":   model.StartupScript,
		"shutdown_script":  model.ShutdownScript,
		"nvlink_domain_id": model.NvlinkDomainID,
	} {
		if !v.IsNull() {
			t.Errorf("%s = %v, want null (empty API value normalizes to null)", name, v)
		}
	}

	var disks []diskToCreateResourceModel
	if d := model.DisksToCreate.ElementsAs(context.Background(), &disks, false); d.HasError() {
		t.Fatalf("reading disks: %v", d)
	}
	if len(disks) != 1 {
		t.Fatalf("got %d disks, want 1", len(disks))
	}
	if got := disks[0].Type.ValueString(); got != "persistent-ssd" {
		t.Errorf("disk type = %q, want %q (sourced from API, not the empty request value)", got, "persistent-ssd")
	}
}

// Test_instanceTemplateToResourceModel_createReadIdentical checks that, given the
// same API object and disk intent, the Create and Read paths converge on identical
// state.
func Test_instanceTemplateToResourceModel_createReadIdentical(t *testing.T) {
	api := sampleAPITemplate()

	var d1, d2 diag.Diagnostics
	createModel := &instanceTemplateResourceModel{DisksToCreate: types.SetNull(diskToCreateSchema)}
	readModel := &instanceTemplateResourceModel{DisksToCreate: types.SetNull(diskToCreateSchema)}

	instanceTemplateToResourceModel(context.Background(), api, createModel, &d1)
	instanceTemplateToResourceModel(context.Background(), api, readModel, &d2)
	if d1.HasError() || d2.HasError() {
		t.Fatalf("unexpected diagnostics: create=%v read=%v", d1, d2)
	}

	if !reflect.DeepEqual(createModel, readModel) {
		t.Errorf("Create and Read produced different state:\n create = %+v\n read   = %+v", createModel, readModel)
	}
}

// Test_instanceTemplateToResourceModel_disksNullVsEmpty verifies that, when the
// template has no disks, the transform preserves the caller's null-vs-empty
// intent instead of collapsing both to the same representation.
func Test_instanceTemplateToResourceModel_disksNullVsEmpty(t *testing.T) {
	api := sampleAPITemplate()
	api.Disks = nil

	t.Run("null stays null", func(t *testing.T) {
		var diags diag.Diagnostics
		model := &instanceTemplateResourceModel{DisksToCreate: types.SetNull(diskToCreateSchema)}
		instanceTemplateToResourceModel(context.Background(), api, model, &diags)
		if !model.DisksToCreate.IsNull() {
			t.Errorf("disks = %v, want null", model.DisksToCreate)
		}
	})

	t.Run("empty stays empty", func(t *testing.T) {
		var diags diag.Diagnostics
		empty, d := types.SetValueFrom(context.Background(), diskToCreateSchema, []diskToCreateResourceModel{})
		if d.HasError() {
			t.Fatalf("building empty set: %v", d)
		}
		model := &instanceTemplateResourceModel{DisksToCreate: empty}
		instanceTemplateToResourceModel(context.Background(), api, model, &diags)
		if model.DisksToCreate.IsNull() {
			t.Error("disks = null, want empty (non-null) set")
		}
		if n := len(model.DisksToCreate.Elements()); n != 0 {
			t.Errorf("disks has %d elements, want 0", n)
		}
	})
}
