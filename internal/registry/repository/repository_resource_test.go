package repository

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestRepositoryResource_Metadata(t *testing.T) {
	r := NewRegistryRepositoryResource()

	req := resource.MetadataRequest{
		ProviderTypeName: "crusoe",
	}
	resp := &resource.MetadataResponse{}

	r.Metadata(context.Background(), req, resp)

	expected := "crusoe_registry_repository"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestRepositoryResource_SchemaRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	// Top-level fields that must trigger RequiresReplace.
	topLevel := []string{"project_id", "location", "name", "mode"}
	for _, name := range topLevel {
		stringAttr, ok := schemaResp.Schema.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute not found or wrong type", name)
		}

		if !hasRequiresReplace(stringAttr.PlanModifiers) {
			t.Errorf("%q must have RequiresReplace plan modifier", name)
		}
	}

	// Nested upstream_registry fields that must trigger RequiresReplace.
	upstream, ok := schemaResp.Schema.Attributes["upstream_registry"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry attribute not found or wrong type")
	}

	nested := []string{"provider", "url"}
	for _, name := range nested {
		stringAttr, ok := upstream.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("upstream_registry.%q attribute not found or wrong type", name)
		}

		if !hasRequiresReplace(stringAttr.PlanModifiers) {
			t.Errorf("upstream_registry.%q must have RequiresReplace plan modifier", name)
		}
	}
}

func TestRepositoryResource_ProjectIDPlanModifiers(t *testing.T) {
	ctx := context.Background()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	projectAttr, ok := schemaResp.Schema.Attributes["project_id"].(schema.StringAttribute)
	if !ok {
		t.Fatal("project_id attribute not found or wrong type")
	}

	tests := []struct {
		name          string
		plan          types.String
		expectReplace bool
	}{
		{name: "unset project_id (unknown) keeps state and does not replace", plan: types.StringUnknown(), expectReplace: false},
		{name: "same project_id does not replace", plan: types.StringValue("proj-a"), expectReplace: false},
		{name: "different project_id requires replace", plan: types.StringValue("proj-b"), expectReplace: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
			req := planmodifier.StringRequest{
				StateValue: types.StringValue("proj-a"),
				PlanValue:  tt.plan,
				State:      tfsdk.State{Raw: raw},
				Plan:       tfsdk.Plan{Raw: raw},
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.plan}

			// The framework runs modifiers in order, feeding each one the plan value the previous one produced.
			for _, m := range projectAttr.PlanModifiers {
				req.PlanValue = resp.PlanValue
				m.PlanModifyString(ctx, req, resp)
			}

			if resp.RequiresReplace != tt.expectReplace {
				t.Errorf("RequiresReplace: expected %v, got %v", tt.expectReplace, resp.RequiresReplace)
			}
		})
	}
}

func TestRepositoryResource_SchemaModeValidator(t *testing.T) {
	ctx := context.Background()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	modeAttr, ok := schemaResp.Schema.Attributes["mode"].(schema.StringAttribute)
	if !ok {
		t.Fatal("mode attribute not found or wrong type")
	}

	if len(modeAttr.Validators) == 0 {
		t.Error("mode must have validators")
	}
}

func TestRepositoryResource_SchemaCredentialsNotRequiresReplace(t *testing.T) {
	ctx := context.Background()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	upstream, ok := schemaResp.Schema.Attributes["upstream_registry"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry attribute not found")
	}

	creds, ok := upstream.Attributes["upstream_registry_credentials"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry_credentials attribute not found")
	}

	for _, name := range []string{"username", "password"} {
		stringAttr, isString := creds.Attributes[name].(schema.StringAttribute)
		if !isString {
			t.Fatalf("upstream_registry_credentials.%q not found", name)
		}

		if hasRequiresReplace(stringAttr.PlanModifiers) {
			t.Errorf("upstream_registry_credentials.%q must NOT have RequiresReplace (credentials are updatable in place)", name)
		}
	}

	versionAttr, ok := creds.Attributes["credentials_version"].(schema.Int64Attribute)
	if !ok {
		t.Fatal("upstream_registry_credentials.credentials_version not found or wrong type")
	}
	if len(versionAttr.PlanModifiers) != 0 {
		t.Error("upstream_registry_credentials.credentials_version must have no plan modifiers (bumping it updates in place)")
	}
}

func TestRequiresReplaceIfCredentialsRemoved(t *testing.T) {
	ctx := context.Background()
	credsType := map[string]attr.Type{
		"username":            types.StringType,
		"password":            types.StringType,
		"credentials_version": types.Int64Type,
	}
	credsValue := func(username string, version int64) types.Object {
		return types.ObjectValueMust(credsType, map[string]attr.Value{
			"username":            types.StringValue(username),
			"password":            types.StringNull(), // write-only: always null in plan and state
			"credentials_version": types.Int64Value(version),
		})
	}
	null := types.ObjectNull(credsType)

	tests := []struct {
		name          string
		state         types.Object
		plan          types.Object
		expectReplace bool
	}{
		{name: "removing credentials requires replace", state: credsValue("u", 1), plan: null, expectReplace: true},
		{name: "bumping credentials_version updates in place", state: credsValue("u", 1), plan: credsValue("u", 2), expectReplace: false},
		{name: "adding credentials updates in place", state: null, plan: credsValue("u", 1), expectReplace: false},
		{name: "no credentials before or after does not replace", state: null, plan: null, expectReplace: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RequiresReplaceIf skips on resource create/destroy, so the raw state and plan must be non-null.
			raw := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
			req := planmodifier.ObjectRequest{
				StateValue: tt.state,
				PlanValue:  tt.plan,
				State:      tfsdk.State{Raw: raw},
				Plan:       tfsdk.Plan{Raw: raw},
			}
			resp := &planmodifier.ObjectResponse{PlanValue: tt.plan}

			requiresReplaceIfCredentialsRemoved().PlanModifyObject(ctx, req, resp)

			if resp.RequiresReplace != tt.expectReplace {
				t.Errorf("RequiresReplace: expected %v, got %v", tt.expectReplace, resp.RequiresReplace)
			}
		})
	}
}

func TestRequiresReplaceIfUpstreamAddedOrRemoved(t *testing.T) {
	ctx := context.Background()
	upstreamType := map[string]attr.Type{"provider": types.StringType, "url": types.StringType}
	upstream := func(url string) types.Object {
		return types.ObjectValueMust(upstreamType, map[string]attr.Value{
			"provider": types.StringValue("aws-ecr"),
			"url":      types.StringValue(url),
		})
	}
	null := types.ObjectNull(upstreamType)

	tests := []struct {
		name          string
		state         types.Object
		plan          types.Object
		expectReplace bool
	}{
		{name: "removing the upstream_registry block requires replace", state: upstream("https://a"), plan: null, expectReplace: true},
		{name: "adding an upstream_registry block requires replace", state: null, plan: upstream("https://a"), expectReplace: true},
		{name: "a nested change leaves replacement to the child modifiers", state: upstream("https://a"), plan: upstream("https://b"), expectReplace: false},
		{name: "unchanged block does not replace", state: upstream("https://a"), plan: upstream("https://a"), expectReplace: false},
		{name: "no upstream_registry before or after does not replace", state: null, plan: null, expectReplace: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// RequiresReplaceIf skips on resource create/destroy, so the raw state and plan must be non-null.
			raw := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
			req := planmodifier.ObjectRequest{
				StateValue: tt.state,
				PlanValue:  tt.plan,
				State:      tfsdk.State{Raw: raw},
				Plan:       tfsdk.Plan{Raw: raw},
			}
			resp := &planmodifier.ObjectResponse{PlanValue: tt.plan}

			requiresReplaceIfUpstreamAddedOrRemoved().PlanModifyObject(ctx, req, resp)

			if resp.RequiresReplace != tt.expectReplace {
				t.Errorf("RequiresReplace: expected %v, got %v", tt.expectReplace, resp.RequiresReplace)
			}
		})
	}
}

func TestCredentialsChanged(t *testing.T) {
	// username and password are write-only, so plan and state only ever carry credentials_version.
	registry := func(version types.Int64) *upstreamRegistryResourceModel {
		return &upstreamRegistryResourceModel{
			UpstreamRegistryCrdentials: &upstreamRegistryCredentialsResourceModel{
				Username:           types.StringNull(),
				Password:           types.StringNull(),
				CredentialsVersion: version,
			},
		}
	}
	noCreds := &upstreamRegistryResourceModel{}

	tests := []struct {
		name  string
		state *upstreamRegistryResourceModel
		plan  *upstreamRegistryResourceModel
		want  bool
	}{
		{name: "unchanged version sends nothing", state: registry(types.Int64Value(1)), plan: registry(types.Int64Value(1)), want: false},
		{name: "unset version on both sides sends nothing", state: registry(types.Int64Null()), plan: registry(types.Int64Null()), want: false},
		{name: "bumped version sends credentials", state: registry(types.Int64Value(1)), plan: registry(types.Int64Value(2)), want: true},
		{name: "version set for the first time sends credentials", state: registry(types.Int64Null()), plan: registry(types.Int64Value(1)), want: true},
		{name: "credentials added to a repo without them sends credentials", state: noCreds, plan: registry(types.Int64Null()), want: true},
		{name: "no credentials in plan sends nothing", state: registry(types.Int64Value(1)), plan: noCreds, want: false},
		{name: "no upstream registry sends nothing", state: nil, plan: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := credentialsChanged(tt.state, tt.plan); got != tt.want {
				t.Errorf("credentialsChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepositoryResource_SchemaPasswordSensitive(t *testing.T) {
	ctx := context.Background()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	upstream, ok := schemaResp.Schema.Attributes["upstream_registry"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry attribute not found or wrong type")
	}
	creds, ok := upstream.Attributes["upstream_registry_credentials"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry_credentials attribute not found or wrong type")
	}
	pw, ok := creds.Attributes["password"].(schema.StringAttribute)
	if !ok {
		t.Fatal("upstream_registry_credentials.password attribute not found or wrong type")
	}

	if !pw.Sensitive {
		t.Error("password must be marked Sensitive")
	}
	if !pw.WriteOnly {
		t.Error("password must be WriteOnly so it never reaches plan or state")
	}

	user, ok := creds.Attributes["username"].(schema.StringAttribute)
	if !ok {
		t.Fatal("upstream_registry_credentials.username attribute not found or wrong type")
	}
	if !user.WriteOnly {
		t.Error("username must be WriteOnly so it never reaches plan or state")
	}
}

func TestRepositoryResource_SchemaIsValid(t *testing.T) {
	ctx := context.Background()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	if diags := schemaResp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("schema is invalid: %v", diags)
	}
}

func TestUpstreamCredentials(t *testing.T) {
	t.Run("uses the config's write-only password", func(t *testing.T) {
		got := upstreamCredentials(&upstreamRegistryResourceModel{
			UpstreamRegistryCrdentials: &upstreamRegistryCredentialsResourceModel{
				Username: types.StringValue("example-access-key-id"),
				Password: types.StringValue("s3cr3t"),
			},
		})
		if got == nil || got.Username != "example-access-key-id" || got.Password != "s3cr3t" {
			t.Errorf("upstreamCredentials() = %+v, want username example-access-key-id and password s3cr3t", got)
		}
	})
	t.Run("no credentials block returns nil", func(t *testing.T) {
		if got := upstreamCredentials(&upstreamRegistryResourceModel{}); got != nil {
			t.Errorf("upstreamCredentials() = %+v, want nil", got)
		}
	})
	t.Run("no upstream registry returns nil", func(t *testing.T) {
		if got := upstreamCredentials(nil); got != nil {
			t.Errorf("upstreamCredentials() = %+v, want nil", got)
		}
	})
}

func TestUpgradeRepositoryStateV2ToV3(t *testing.T) {
	ctx := context.Background()
	priorSchema := repositorySchemaV2()
	r := NewRegistryRepositoryResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

	prior := tfsdk.State{Schema: *priorSchema, Raw: tftypes.NewValue(priorSchema.Type().TerraformType(ctx), nil)}
	if diags := prior.Set(ctx, repositoryModelV2{
		ProjectID: types.StringValue("proj-123"),
		Location:  types.StringValue("us-southcentral1-a"),
		Name:      types.StringValue("ptc-ecr"),
		Mode:      types.StringValue("pull-through-cache"),
		UpstreamRegistry: &upstreamRegistryModelV2{
			Provider: types.StringValue("aws-ecr"),
			Url:      types.StringValue("https://123456789012.dkr.ecr.us-east-1.amazonaws.com"),
			UpstreamRegistryCrdentials: &upstreamRegistryCredentialsV2{
				Username: types.StringValue("example-access-key-id"),
				Password: types.StringValue("s3cr3t"),
			},
		},
	}); diags.HasError() {
		t.Fatalf("building v2 state: %v", diags)
	}

	req := resource.UpgradeStateRequest{State: &prior}
	resp := &resource.UpgradeStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil)},
	}
	upgradeRepositoryStateV2ToV3(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade failed: %v", resp.Diagnostics)
	}

	var upgraded repositoryResourceModel
	if diags := resp.State.Get(ctx, &upgraded); diags.HasError() {
		t.Fatalf("reading upgraded state: %v", diags)
	}
	if upgraded.Name.ValueString() != "ptc-ecr" || upgraded.ProjectID.ValueString() != "proj-123" {
		t.Errorf("top-level fields not carried over: %+v", upgraded)
	}
	creds := upgraded.UpstreamRegistry.UpstreamRegistryCrdentials
	if creds == nil {
		t.Fatal("credentials block dropped; it must stay so removing it from config still plans a replace")
	}
	if !creds.Username.IsNull() {
		t.Error("username must be null after upgrade: the v2 username must not survive in state")
	}
	if !creds.Password.IsNull() {
		t.Error("password must be null after upgrade: the v2 password must not survive in state")
	}
	if !creds.CredentialsVersion.IsNull() {
		t.Error("credentials_version must start null after upgrade")
	}
}

// hasRequiresReplace reports whether the modifiers include stringplanmodifier.RequiresReplace().
func hasRequiresReplace(modifiers []planmodifier.String) bool {
	ctx := context.Background()
	want := stringplanmodifier.RequiresReplace().Description(ctx)
	for _, m := range modifiers {
		if m.Description(ctx) == want {
			return true
		}
	}

	return false
}
