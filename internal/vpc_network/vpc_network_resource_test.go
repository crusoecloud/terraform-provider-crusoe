package vpc_network

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// newImportStateResponse builds an empty ImportStateResponse backed by the
// given resource schema, mirroring how the framework initializes state before
// calling ImportState.
func newImportStateResponse(ctx context.Context, t *testing.T, schemaFn func(context.Context, resource.SchemaRequest, *resource.SchemaResponse)) *resource.ImportStateResponse {
	t.Helper()

	schemaResp := &resource.SchemaResponse{}
	schemaFn(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaResp.Diagnostics)
	}

	return &resource.ImportStateResponse{
		State: tfsdk.State{
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
			Schema: schemaResp.Schema,
		},
	}
}

// TestVPCNetworkImportState exercises the ImportState path end-to-end: it
// confirms the parsed identifiers land in the "id" and "project_id" schema
// attributes, that the client project ID is used as a fallback, and that an
// invalid identifier surfaces a diagnostic. The parsing logic itself is covered
// by TestParseResourceIdentifiers in the common package.
func TestVPCNetworkImportState(t *testing.T) {
	const (
		resourceUUID = "11111111-1111-1111-1111-111111111111"
		projectUUID  = "22222222-2222-2222-2222-222222222222"
		fallbackUUID = "33333333-3333-3333-3333-333333333333"
	)

	ctx := context.Background()

	t.Run("explicit project id from suffix", func(t *testing.T) {
		r := &vpcNetworkResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
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
		r := &vpcNetworkResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
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
		r := &vpcNetworkResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: "not-a-uuid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Error("expected diagnostics error for invalid identifier, got none")
		}
	})
}
