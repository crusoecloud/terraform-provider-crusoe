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
