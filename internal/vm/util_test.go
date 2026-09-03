package vm

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

func Test_getDisksDiff(t *testing.T) {
	type args struct {
		orig []vmDiskResourceModel
		new  []vmDiskResourceModel
	}
	tests := []struct {
		name             string
		args             args
		wantDisksAdded   []swagger.DiskAttachment
		wantDisksRemoved []string
	}{
		{
			name: "all match",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234", AttachmentType: "data", Mode: "read-write"}},
				new:  []vmDiskResourceModel{{ID: "1234", AttachmentType: "data", Mode: "read-write"}},
			},
			wantDisksAdded:   nil,
			wantDisksRemoved: nil,
		},
		{
			name: "disk added",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234", AttachmentType: "data", Mode: "read-write"}},
				new: []vmDiskResourceModel{
					{ID: "1234", AttachmentType: "data", Mode: "read-write"},
					{ID: "2345", AttachmentType: "data", Mode: "read-only"},
				},
			},
			wantDisksAdded:   []swagger.DiskAttachment{{DiskId: "2345", AttachmentType: "data", Mode: "read-only"}},
			wantDisksRemoved: nil,
		},
		{
			name: "disk removed",
			args: args{
				orig: []vmDiskResourceModel{
					{ID: "1234", AttachmentType: "data", Mode: "read-only"},
					{ID: "2345", AttachmentType: "data", Mode: "read-only"},
				},
				new: []vmDiskResourceModel{{ID: "2345", AttachmentType: "data", Mode: "read-only"}},
			},
			wantDisksAdded:   nil,
			wantDisksRemoved: []string{"1234"},
		},
		{
			name: "disk added and removed",
			args: args{
				orig: []vmDiskResourceModel{{ID: "1234"}},
				new:  []vmDiskResourceModel{{ID: "2345"}},
			},
			wantDisksAdded:   []swagger.DiskAttachment{{DiskId: "2345"}},
			wantDisksRemoved: []string{"1234"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDisksAdded, gotDisksRemoved := getDisksDiff(tt.args.orig, tt.args.new)
			if !reflect.DeepEqual(gotDisksAdded, tt.wantDisksAdded) {
				t.Errorf("getDisksDiff() gotDisksAdded = %v, want %v", gotDisksAdded, tt.wantDisksAdded)
			}
			if !reflect.DeepEqual(gotDisksRemoved, tt.wantDisksRemoved) {
				t.Errorf("getDisksDiff() gotDisksRemoved = %v, want %v", gotDisksRemoved, tt.wantDisksRemoved)
			}
		})
	}
}

// Test_vmToTerraformResourceModel covers the shared transform that Create and Read
// now both use: the OS disk is filtered out of `disks`, name/type/location/project_id
// come from the API, nvlink_domain_id/reservation_id are represented as the API value
// (so Create matches Read — Create previously wrote null for the empty case), and
// install_crusoe_watch_agent defaults to true when unset (create-only, not returned).
func Test_vmToTerraformResourceModel(t *testing.T) {
	instance := &swagger.InstanceV1{
		Id:        "vm-1",
		Name:      "my-vm",
		Type_:     "c1a.2x",
		ProjectId: "proj-1",
		Location:  "us-east1-a",
		Disks: []swagger.AttachedDiskV1{
			{Id: "os-disk", AttachmentType: DiskOS, Mode: "read-write"},
			{Id: "data-disk", AttachmentType: "data", Mode: "read-write"},
		},
		ReservationId:  "",
		NvlinkDomainId: "",
	}

	state := &vmResourceModel{}
	vmToTerraformResourceModel(instance, state)

	if got := state.ID.ValueString(); got != "vm-1" {
		t.Errorf("id = %q, want %q", got, "vm-1")
	}
	if got := state.ProjectID.ValueString(); got != "proj-1" {
		t.Errorf("project_id = %q, want %q (from API)", got, "proj-1")
	}
	if got := state.Type.ValueString(); got != "c1a.2x" {
		t.Errorf("type = %q, want %q (from API)", got, "c1a.2x")
	}
	// nvlink_domain_id / reservation_id: empty API value maps to an empty string (not
	// null), so Create and Read agree.
	if state.NvlinkDomainID.IsNull() {
		t.Error("nvlink_domain_id = null, want empty string (matches Read)")
	}
	if state.ReservationID.IsNull() {
		t.Error("reservation_id = null, want empty string (matches Read)")
	}
	if !state.InstallCrusoeWatchAgent.ValueBool() {
		t.Error("install_crusoe_watch_agent = false, want true default when unset")
	}

	var disks []vmDiskResourceModel
	if d := state.Disks.ElementsAs(context.Background(), &disks, false); d.HasError() {
		t.Fatalf("reading disks: %v", d)
	}
	if len(disks) != 1 {
		t.Fatalf("got %d disks, want 1 (OS disk filtered out)", len(disks))
	}
	if disks[0].ID != "data-disk" {
		t.Errorf("disk = %q, want the data disk (OS disk must be filtered)", disks[0].ID)
	}
}

func Test_instanceTypeFamily(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFamily string
		wantOK     bool
	}{
		{name: "cpu type", input: "c1a.2x", wantFamily: "c1a", wantOK: true},
		{name: "storage type", input: "s1a.40x", wantFamily: "s1a", wantOK: true},
		{name: "gpu type with dash", input: "l40s-48gb.4x", wantFamily: "l40s-48gb", wantOK: true},
		{name: "missing size", input: "c1a", wantFamily: "", wantOK: false},
		{name: "empty", input: "", wantFamily: "", wantOK: false},
		{name: "leading dot", input: ".2x", wantFamily: "", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFamily, gotOK := instanceTypeFamily(tt.input)
			if gotFamily != tt.wantFamily || gotOK != tt.wantOK {
				t.Errorf("instanceTypeFamily(%q) = (%q, %v), want (%q, %v)",
					tt.input, gotFamily, gotOK, tt.wantFamily, tt.wantOK)
			}
		})
	}
}

func Test_resizeRequiresReplace(t *testing.T) {
	tests := []struct {
		name        string
		state       types.String
		plan        types.String
		wantReplace bool
		wantWarning bool
	}{
		{
			name:        "same family increase resizes in place",
			state:       types.StringValue("c1a.2x"),
			plan:        types.StringValue("c1a.4x"),
			wantReplace: false,
			wantWarning: true,
		},
		{
			name:        "same family decrease resizes in place",
			state:       types.StringValue("s1a.80x"),
			plan:        types.StringValue("s1a.20x"),
			wantReplace: false,
			wantWarning: true,
		},
		{
			name:        "different family requires replace",
			state:       types.StringValue("c1a.2x"),
			plan:        types.StringValue("s1a.20x"),
			wantReplace: true,
			wantWarning: false,
		},
		{
			name:        "gpu family change requires replace",
			state:       types.StringValue("a40.1x"),
			plan:        types.StringValue("a100.2x"),
			wantReplace: true,
			wantWarning: false,
		},
		{
			name:        "unparseable plan requires replace",
			state:       types.StringValue("c1a.2x"),
			plan:        types.StringValue("c1a"),
			wantReplace: true,
			wantWarning: false,
		},
		{
			name:        "unchanged is a no-op",
			state:       types.StringValue("c1a.2x"),
			plan:        types.StringValue("c1a.2x"),
			wantReplace: false,
			wantWarning: false,
		},
		{
			name:        "null state is a no-op",
			state:       types.StringNull(),
			plan:        types.StringValue("c1a.2x"),
			wantReplace: false,
			wantWarning: false,
		},
		{
			name:        "unknown plan is a no-op",
			state:       types.StringValue("c1a.2x"),
			plan:        types.StringUnknown(),
			wantReplace: false,
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue: tt.state,
				PlanValue:  tt.plan,
			}
			resp := &stringplanmodifier.RequiresReplaceIfFuncResponse{}
			resizeRequiresReplace(context.Background(), req, resp)

			if resp.RequiresReplace != tt.wantReplace {
				t.Errorf("resizeRequiresReplace() RequiresReplace = %v, want %v", resp.RequiresReplace, tt.wantReplace)
			}
			if resp.Diagnostics.WarningsCount() > 0 != tt.wantWarning {
				t.Errorf("resizeRequiresReplace() warnings = %d, wantWarning %v", resp.Diagnostics.WarningsCount(), tt.wantWarning)
			}
		})
	}
}

// hcaRefList builds a host channel adapter list to stand in for the plan (Create) or the
// prior state (Read). An empty string means the attribute is unset.
func hcaRefList(t *testing.T, ibPartitionID, transportPartitionID string) types.List {
	t.Helper()

	list, diags := types.ListValueFrom(context.Background(), vmHostChannelAdapterSchema,
		[]vmHostChannelAdapterResourceModel{{
			IBPartitionID:        ibPartitionID,
			TransportPartitionID: transportPartitionID,
		}})
	if diags.HasError() {
		t.Fatalf("building reference list: %v", diags)
	}

	return list
}

// readHCA reads the single host channel adapter out of a mapped list.
func readHCA(t *testing.T, list types.List) vmHostChannelAdapterResourceModel {
	t.Helper()

	var hcas []vmHostChannelAdapterResourceModel
	if diags := list.ElementsAs(context.Background(), &hcas, true); diags.HasError() {
		t.Fatalf("reading host channel adapters: %v", diags)
	}
	if len(hcas) != 1 {
		t.Fatalf("got %d host channel adapters, want 1", len(hcas))
	}

	return hcas[0]
}

// Test_vmHostChannelAdaptersToTerraformResourceModel covers the partition alias pair on a
// host channel adapter. ib_partition_id and transport_partition_id name the same
// partition, so whichever name the configuration uses has to survive, and the replacement
// name always ends up populated.
//
// Preserving the deprecated half matters because vmToTerraformResourceModel runs only in
// Create and Read: Update never rewrites host channel adapter state. A diff on that half
// would have no way to resolve itself.
func Test_vmHostChannelAdaptersToTerraformResourceModel(t *testing.T) {
	tests := []struct {
		name          string
		apiIB         string
		apiTransport  string
		ref           types.List
		wantIB        string
		wantTransport string
	}{
		{
			name:  "API reports only the deprecated name",
			apiIB: "p1", apiTransport: "",
			ref: hcaRefList(t, "p1", ""),
			// The replacement name is filled in from the deprecated one, so a rename in
			// the configuration is a no-op rather than a diff.
			wantIB: "p1", wantTransport: "p1",
		},
		{
			name:  "API reports only the replacement name, configuration uses the deprecated one",
			apiIB: "", apiTransport: "p1",
			ref: hcaRefList(t, "p1", ""),
			// The deprecated half comes from the reference, so a configuration still on
			// ib_partition_id does not see a diff it cannot resolve.
			wantIB: "p1", wantTransport: "p1",
		},
		{
			name:  "API reports both names",
			apiIB: "p1", apiTransport: "p1",
			ref:    hcaRefList(t, "", "p1"),
			wantIB: "p1", wantTransport: "p1",
		},
		{
			name:  "API reports neither name, reference supplies the partition",
			apiIB: "", apiTransport: "",
			ref:    hcaRefList(t, "", "p1"),
			wantIB: "", wantTransport: "p1",
		},
		{
			name:  "API reports neither name and there is no reference",
			apiIB: "", apiTransport: "",
			ref:    types.ListNull(vmHostChannelAdapterSchema),
			wantIB: "", wantTransport: "",
		},
		{
			name:  "unknown reference on create",
			apiIB: "p1", apiTransport: "",
			ref:    types.ListUnknown(vmHostChannelAdapterSchema),
			wantIB: "p1", wantTransport: "p1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := []swagger.HostChannelAdapter{{
				IbPartitionId:        tt.apiIB,
				TransportPartitionId: tt.apiTransport,
			}}

			got := readHCA(t, vmHostChannelAdaptersToTerraformResourceModel(api, tt.ref))

			if got.IBPartitionID != tt.wantIB {
				t.Errorf("ib_partition_id = %q, want %q", got.IBPartitionID, tt.wantIB)
			}
			if got.TransportPartitionID != tt.wantTransport {
				t.Errorf("transport_partition_id = %q, want %q", got.TransportPartitionID, tt.wantTransport)
			}
		})
	}
}

// Test_vmHostChannelAdaptersToTerraformResourceModel_empty checks that a VM with no host
// channel adapters maps to an empty list rather than a list holding a blank adapter.
func Test_vmHostChannelAdaptersToTerraformResourceModel_empty(t *testing.T) {
	got := vmHostChannelAdaptersToTerraformResourceModel(nil, types.ListNull(vmHostChannelAdapterSchema))

	if got.IsNull() || got.IsUnknown() {
		t.Fatalf("got %v, want a known empty list", got)
	}
	if n := len(got.Elements()); n != 0 {
		t.Errorf("got %d host channel adapters, want 0", n)
	}
}
