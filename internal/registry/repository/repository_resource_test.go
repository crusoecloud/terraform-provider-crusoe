package repository

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

func repositoryResourceSchema(t *testing.T) schema.Schema {
	t.Helper()

	resp := &resource.SchemaResponse{}
	NewRegistryRepositoryResource().Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func TestRepositoryResourceSchema_planModifiers(t *testing.T) {
	s := repositoryResourceSchema(t)

	stringModifiers := func(attrs map[string]schema.Attribute, name string) []planmodifier.String {
		t.Helper()
		attr, ok := attrs[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is not a string attribute", name)
		}

		return attr.PlanModifiers
	}

	for _, name := range []string{"project_id", "location", "name", "mode"} {
		if len(stringModifiers(s.Attributes, name)) == 0 {
			t.Errorf("%s should have plan modifiers (requires replacement)", name)
		}
	}

	upstream, ok := s.Attributes["upstream_registry"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry is not a single nested attribute")
	}
	if len(upstream.PlanModifiers) == 0 {
		t.Error("upstream_registry should require replacement when added or removed")
	}
	for _, name := range []string{"provider", "url"} {
		if len(stringModifiers(upstream.Attributes, name)) == 0 {
			t.Errorf("upstream_registry.%s should require replacement", name)
		}
	}

	credentials, ok := upstream.Attributes["upstream_registry_credentials"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("upstream_registry_credentials is not a single nested attribute")
	}
	if len(credentials.PlanModifiers) == 0 {
		t.Error("upstream_registry_credentials should require replacement when removed")
	}
	for _, name := range []string{"username", "password"} {
		if len(stringModifiers(credentials.Attributes, name)) != 0 {
			t.Errorf("upstream_registry_credentials.%s should be updatable in place", name)
		}

		credential, ok := credentials.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("upstream_registry_credentials.%s is not a string attribute", name)
		}
		if len(credential.Validators) == 0 {
			t.Errorf("upstream_registry_credentials.%s should reject an empty value", name)
		}
	}
}

var credentialsAttrTypes = map[string]attr.Type{
	"username": types.StringType,
	"password": types.StringType,
}

func credentialsObject(username, password attr.Value) types.Object {
	return types.ObjectValueMust(credentialsAttrTypes, map[string]attr.Value{
		"username": username,
		"password": password,
	})
}

func TestRequiresReplaceIfAddedOrRemoved(t *testing.T) {
	set := types.ObjectValueMust(map[string]attr.Type{}, map[string]attr.Value{})
	null := types.ObjectNull(map[string]attr.Type{})

	tests := []struct {
		name        string
		state, plan types.Object
		want        bool
	}{
		{name: "unchanged set", state: set, plan: set},
		{name: "unchanged null", state: null, plan: null},
		{name: "added", state: null, plan: set, want: true},
		{name: "removed", state: set, plan: null, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &objectplanmodifier.RequiresReplaceIfFuncResponse{}
			requiresReplaceIfAddedOrRemoved(context.Background(),
				planmodifier.ObjectRequest{StateValue: tt.state, PlanValue: tt.plan}, resp)
			if resp.RequiresReplace != tt.want {
				t.Errorf("requiresReplaceIfAddedOrRemoved = %v, want %v", resp.RequiresReplace, tt.want)
			}
		})
	}
}

func TestRequiresReplaceIfCredentialsCleared(t *testing.T) {
	filled := credentialsObject(types.StringValue("user"), types.StringValue("s3cr3t"))
	emptied := credentialsObject(types.StringValue(""), types.StringValue(""))
	nulled := credentialsObject(types.StringNull(), types.StringNull())
	null := types.ObjectNull(credentialsAttrTypes)

	tests := []struct {
		name        string
		state, plan types.Object
		want        bool
	}{
		{name: "unchanged", state: filled, plan: filled},
		{name: "changed password", state: filled, plan: credentialsObject(types.StringValue("user"), types.StringValue("new"))},
		{name: "added", state: null, plan: filled},
		{name: "added to emptied", state: emptied, plan: filled},
		{name: "removed", state: filled, plan: null, want: true},
		{name: "emptied", state: filled, plan: emptied, want: true},
		{name: "nulled out", state: filled, plan: nulled, want: true},
		{
			name:  "password emptied only",
			state: filled,
			plan:  credentialsObject(types.StringValue("user"), types.StringValue("")),
			want:  true,
		},
		{
			name:  "username emptied only",
			state: filled,
			plan:  credentialsObject(types.StringValue(""), types.StringValue("s3cr3t")),
			want:  true,
		},
		{
			name:  "password nulled only",
			state: filled,
			plan:  credentialsObject(types.StringValue("user"), types.StringNull()),
			want:  true,
		},
		{name: "still empty", state: emptied, plan: emptied},
		{
			name:  "empty username stays empty",
			state: credentialsObject(types.StringValue(""), types.StringValue("s3cr3t")),
			plan:  credentialsObject(types.StringValue(""), types.StringValue("new")),
		},
		{
			name:  "password not decided yet",
			state: filled,
			plan:  credentialsObject(types.StringValue("user"), types.StringUnknown()),
		},
		{name: "whole object not decided yet", state: filled, plan: types.ObjectUnknown(credentialsAttrTypes)},
		{name: "created", state: types.ObjectNull(credentialsAttrTypes), plan: filled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &objectplanmodifier.RequiresReplaceIfFuncResponse{}
			requiresReplaceIfCredentialsCleared(context.Background(),
				planmodifier.ObjectRequest{StateValue: tt.state, PlanValue: tt.plan}, resp)
			if resp.RequiresReplace != tt.want {
				t.Errorf("requiresReplaceIfCredentialsCleared = %v, want %v", resp.RequiresReplace, tt.want)
			}
		})
	}
}

func repositoryModelWithPassword(password string) *repositoryResourceModel {
	return &repositoryResourceModel{
		ProjectID: types.StringValue("proj-123"),
		Location:  types.StringValue("us-east1-a"),
		Name:      types.StringValue("my-repo"),
		Mode:      types.StringValue("pull-through-cache"),
		UpstreamRegistry: &upstreamRegistryResourceModel{
			Provider: types.StringValue("google-gar"),
			Url:      types.StringValue("https://us-central1-docker.pkg.dev"),
			UpstreamRegistryCrdentials: &upstreamRegistryCredentialsResourceModel{
				Username: types.StringValue("_json_key"),
				Password: types.StringValue(password),
			},
		},
	}
}

type recordedRequest struct {
	method string
	path   string
	body   map[string]string
}

func runUpdate(t *testing.T, state, plan *repositoryResourceModel) ([]recordedRequest, repositoryResourceModel, *resource.UpdateResponse) {
	t.Helper()
	ctx := context.Background()

	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		rec := recordedRequest{method: r.Method, path: r.URL.Path}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &rec.body); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		requests = append(requests, rec)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{}")); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	r := &repositoryResource{client: &common.CrusoeClient{
		APIClient: common.NewAPIClient(server.URL, "key", "secret"),
		ProjectID: "fallback-project",
	}}

	s := repositoryResourceSchema(t)
	emptyValue := tftypes.NewValue(s.Type().TerraformType(ctx), nil)

	req := resource.UpdateRequest{
		State: tfsdk.State{Schema: s, Raw: emptyValue},
		Plan:  tfsdk.Plan{Schema: s, Raw: emptyValue},
	}
	if diags := req.State.Set(ctx, state); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	if diags := req.Plan.Set(ctx, plan); diags.HasError() {
		t.Fatalf("set plan: %v", diags)
	}

	resp := &resource.UpdateResponse{State: tfsdk.State{Schema: s, Raw: emptyValue}}
	r.Update(ctx, req, resp)

	var got repositoryResourceModel
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &got); diags.HasError() {
			t.Fatalf("get new state: %v", diags)
		}
	}

	return requests, got, resp
}

func TestRepositoryResourceUpdate_changedCredentialsPatchesRepository(t *testing.T) {
	requests, got, resp := runUpdate(t, repositoryModelWithPassword("old-key"), repositoryModelWithPassword("new-key"))
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}

	if len(requests) != 1 {
		t.Fatalf("got %d API requests, want 1", len(requests))
	}
	req := requests[0]
	if req.method != http.MethodPatch {
		t.Errorf("method = %s, want PATCH", req.method)
	}
	if want := "/projects/proj-123/ccr/repositories/my-repo"; req.path != want {
		t.Errorf("path = %s, want %s", req.path, want)
	}
	if req.body["username"] != "_json_key" || req.body["password"] != "new-key" {
		t.Errorf("body = %v, want the planned credentials", req.body)
	}

	if pw := got.UpstreamRegistry.UpstreamRegistryCrdentials.Password.ValueString(); pw != "new-key" {
		t.Errorf("state password = %q, want %q", pw, "new-key")
	}
	if got.ProjectID.ValueString() != "proj-123" {
		t.Errorf("state project_id = %q, want %q", got.ProjectID.ValueString(), "proj-123")
	}
}

func TestRepositoryResourceUpdate_clearedCredentialsFails(t *testing.T) {
	bothEmpty := repositoryModelWithPassword("")
	bothEmpty.UpstreamRegistry.UpstreamRegistryCrdentials.Username = types.StringValue("")

	passwordEmpty := repositoryModelWithPassword("")

	usernameEmpty := repositoryModelWithPassword("old-key")
	usernameEmpty.UpstreamRegistry.UpstreamRegistryCrdentials.Username = types.StringValue("")

	// An empty value is left out of the request body, so a PATCH would keep the
	// repository's current credentials while state recorded the empty one.
	for name, plan := range map[string]*repositoryResourceModel{
		"both values emptied": bothEmpty,
		"password emptied":    passwordEmpty,
		"username emptied":    usernameEmpty,
	} {
		t.Run(name, func(t *testing.T) {
			requests, _, resp := runUpdate(t, repositoryModelWithPassword("old-key"), plan)
			if !resp.Diagnostics.HasError() {
				t.Error("emptying a credential should fail instead of reporting a success that only changed state")
			}
			if len(requests) != 0 {
				t.Errorf("got %d API requests, want 0", len(requests))
			}
		})
	}
}

// A freshly imported repository has no credentials in state, whether or not the
// repository has them: the API never returns the password. Setting real credentials
// must reach the API, and empty ones must be refused rather than only stored.
func TestRepositoryResourceUpdate_importedStateWithoutCredentials(t *testing.T) {
	imported := repositoryModelWithPassword("ignored")
	imported.UpstreamRegistry.UpstreamRegistryCrdentials = nil

	t.Run("real credentials are sent", func(t *testing.T) {
		requests, _, resp := runUpdate(t, imported, repositoryModelWithPassword("new-key"))
		if resp.Diagnostics.HasError() {
			t.Fatalf("update diagnostics: %v", resp.Diagnostics)
		}
		if len(requests) != 1 || requests[0].method != http.MethodPatch {
			t.Errorf("got %v, want one PATCH", requests)
		}
	})

	t.Run("empty credentials are refused", func(t *testing.T) {
		empty := repositoryModelWithPassword("")
		empty.UpstreamRegistry.UpstreamRegistryCrdentials.Username = types.StringValue("")

		requests, _, resp := runUpdate(t, imported, empty)
		if !resp.Diagnostics.HasError() {
			t.Error("empty credentials should fail instead of being recorded in state")
		}
		if len(requests) != 0 {
			t.Errorf("got %d API requests, want 0", len(requests))
		}
	})
}

func TestRepositoryResourceUpdate_unchangedCredentialsSkipsAPI(t *testing.T) {
	requests, _, resp := runUpdate(t, repositoryModelWithPassword("same"), repositoryModelWithPassword("same"))
	if resp.Diagnostics.HasError() {
		t.Fatalf("update diagnostics: %v", resp.Diagnostics)
	}
	if len(requests) != 0 {
		t.Errorf("got %d API requests, want 0", len(requests))
	}
}
