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
	// ib_partition and transport_partition_id are deliberately absent from this loop.
	// They are plan-owned, so the transform does not write them at all; their behavior is
	// covered by Test_instanceTemplateToResourceModel_leavesAliasPairUntouched.
	for name, v := range map[string]types.String{
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

// Test_instanceTemplateToResourceModel_leavesAliasPairUntouched pins the ownership of the
// partition alias pair. The transform must not write either half, even when the API
// reports values for both.
//
// This matters on Create. Both attributes are Optional and not Computed, so their planned
// value is a known null, and writing an API value over that null fails Terraform's
// post-apply consistency check.
func Test_instanceTemplateToResourceModel_leavesAliasPairUntouched(t *testing.T) {
	api := sampleAPITemplate()
	api.IbPartitionId = "p1"
	api.TransportPartitionId = "p1"

	tests := []struct {
		name            string
		ibPartition     types.String
		transportID     types.String
		wantIBPartition types.String
		wantTransportID types.String
	}{
		{
			// A configuration still on the deprecated name keeps it, and does not gain the
			// replacement name behind the user's back.
			name:            "configuration uses the deprecated name",
			ibPartition:     types.StringValue("p1"),
			transportID:     types.StringNull(),
			wantIBPartition: types.StringValue("p1"),
			wantTransportID: types.StringNull(),
		},
		{
			// A configuration on the replacement name keeps a null deprecated half, even
			// though the API echoes ib_partition_id.
			name:            "configuration uses the replacement name",
			ibPartition:     types.StringNull(),
			transportID:     types.StringValue("p1"),
			wantIBPartition: types.StringNull(),
			wantTransportID: types.StringValue("p1"),
		},
		{
			name:            "configuration sets neither",
			ibPartition:     types.StringNull(),
			transportID:     types.StringNull(),
			wantIBPartition: types.StringNull(),
			wantTransportID: types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			model := &instanceTemplateResourceModel{
				IBPartition:          tt.ibPartition,
				TransportPartitionID: tt.transportID,
			}

			instanceTemplateToResourceModel(context.Background(), api, model, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if !model.IBPartition.Equal(tt.wantIBPartition) {
				t.Errorf("ib_partition = %v, want %v", model.IBPartition, tt.wantIBPartition)
			}
			if !model.TransportPartitionID.Equal(tt.wantTransportID) {
				t.Errorf("transport_partition_id = %v, want %v", model.TransportPartitionID, tt.wantTransportID)
			}
		})
	}
}

// Test_hydrateAliasPairForImport covers the one case with no configuration to own the
// alias pair: a freshly imported template, whose state has neither half populated.
func Test_hydrateAliasPairForImport(t *testing.T) {
	tests := []struct {
		name                    string
		stateIB, stateTransport types.String
		apiIB, apiTransport     string
		wantIB, wantTransport   types.String
	}{
		{
			name:    "import with only the deprecated name from the API",
			stateIB: types.StringNull(), stateTransport: types.StringNull(),
			apiIB: "p1", apiTransport: "",
			// The replacement name is filled in, never the deprecated one, so an import
			// does not start life on a deprecated attribute.
			wantIB: types.StringNull(), wantTransport: types.StringValue("p1"),
		},
		{
			name:    "import with the replacement name from the API",
			stateIB: types.StringNull(), stateTransport: types.StringNull(),
			apiIB: "", apiTransport: "p2",
			wantIB: types.StringNull(), wantTransport: types.StringValue("p2"),
		},
		{
			name:    "import of a template with no partition",
			stateIB: types.StringNull(), stateTransport: types.StringNull(),
			apiIB: "", apiTransport: "",
			wantIB: types.StringNull(), wantTransport: types.StringNull(),
		},
		{
			// Not an import: state already owns the pair, so the API must not overwrite it.
			name:    "deprecated half already in state",
			stateIB: types.StringValue("p1"), stateTransport: types.StringNull(),
			apiIB: "p1", apiTransport: "p1",
			wantIB: types.StringValue("p1"), wantTransport: types.StringNull(),
		},
		{
			name:    "replacement half already in state",
			stateIB: types.StringNull(), stateTransport: types.StringValue("p2"),
			apiIB: "p1", apiTransport: "p2",
			wantIB: types.StringNull(), wantTransport: types.StringValue("p2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &instanceTemplateResourceModel{
				IBPartition:          tt.stateIB,
				TransportPartitionID: tt.stateTransport,
			}
			api := &swagger.InstanceTemplate{
				IbPartitionId:        tt.apiIB,
				TransportPartitionId: tt.apiTransport,
			}

			hydrateAliasPairForImport(model, api)

			if !model.IBPartition.Equal(tt.wantIB) {
				t.Errorf("ib_partition = %v, want %v", model.IBPartition, tt.wantIB)
			}
			if !model.TransportPartitionID.Equal(tt.wantTransport) {
				t.Errorf("transport_partition_id = %v, want %v", model.TransportPartitionID, tt.wantTransport)
			}
		})
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

func disk(size, diskType string) diskToCreateResourceModel {
	return diskToCreateResourceModel{Size: types.StringValue(size), Type: types.StringValue(diskType)}
}

// Sizes render in the user's unit, not the API's GiB.
func Test_disksToSet_preservesConfiguredUnit(t *testing.T) {
	set := func(disks ...diskToCreateResourceModel) types.Set {
		s, d := types.SetValueFrom(context.Background(), diskToCreateSchema, disks)
		if d.HasError() {
			t.Fatalf("building disk set: %v", d)
		}

		return s
	}

	tests := []struct {
		name     string
		apiDisks []swagger.DiskTemplate
		current  types.Set
		want     []diskToCreateResourceModel
	}{
		{
			name:     "TiB configured, API returns GiB",
			apiDisks: []swagger.DiskTemplate{{Size: "20480GiB", Type_: persistentSSD}},
			current:  set(disk("20TiB", persistentSSD)),
			want:     []diskToCreateResourceModel{disk("20TiB", persistentSSD)},
		},
		{
			name: "matched by capacity, not response order",
			apiDisks: []swagger.DiskTemplate{
				{Size: "1024GiB", Type_: persistentSSD},
				{Size: "100GiB", Type_: persistentSSD},
			},
			current: set(disk("100GiB", persistentSSD), disk("1TiB", persistentSSD)),
			want:    []diskToCreateResourceModel{disk("1TiB", persistentSSD), disk("100GiB", persistentSSD)},
		},
		{
			name:     "no matching capacity keeps the API value",
			apiDisks: []swagger.DiskTemplate{{Size: "2048GiB", Type_: persistentSSD}},
			current:  set(disk("1TiB", persistentSSD)),
			want:     []diskToCreateResourceModel{disk("2048GiB", persistentSSD)},
		},
		{
			name:     "nothing configured keeps the API value",
			apiDisks: []swagger.DiskTemplate{{Size: "1024GiB", Type_: persistentSSD}},
			current:  types.SetNull(diskToCreateSchema),
			want:     []diskToCreateResourceModel{disk("1024GiB", persistentSSD)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := disksToSet(context.Background(), tt.apiDisks, tt.current, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			var gotDisks []diskToCreateResourceModel
			if d := got.ElementsAs(context.Background(), &gotDisks, false); d.HasError() {
				t.Fatalf("reading disks: %v", d)
			}
			if !reflect.DeepEqual(gotDisks, tt.want) {
				t.Errorf("disks = %+v, want %+v", gotDisks, tt.want)
			}
		})
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

func Test_stringsToSet(t *testing.T) {
	t.Run("maps values", func(t *testing.T) {
		var diags diag.Diagnostics
		set := stringsToSet(context.Background(), []string{"disk-1"}, types.SetNull(types.StringType), &diags)
		if n := len(set.Elements()); n != 1 {
			t.Errorf("got %d elements, want 1", n)
		}
	})

	t.Run("empty stays null when unset", func(t *testing.T) {
		var diags diag.Diagnostics
		set := stringsToSet(context.Background(), nil, types.SetNull(types.StringType), &diags)
		if !set.IsNull() {
			t.Errorf("got %v, want null", set)
		}
	})
}
