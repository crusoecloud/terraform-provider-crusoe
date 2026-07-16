package repository

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// upstreamRegistryWithCreds builds a fully-populated upstream_registry model, as a
// caller seeds it from the plan or prior state, including the write-only password.
func upstreamRegistryWithCreds() *upstreamRegistryResourceModel {
	return &upstreamRegistryResourceModel{
		Provider: types.StringValue("docker-hub"),
		Url:      types.StringValue("https://registry-1.docker.io"),
		UpstreamRegistryCrdentials: &upstreamRegistryCredentialsResourceModel{
			Username: types.StringValue("user"),
			Password: types.StringValue("s3cr3t"),
		},
	}
}

// pullThroughCacheAPIObj mirrors a typical GET/CREATE response: the canonical
// field values are present but the write-only password is omitted from the
// credentials block.
func pullThroughCacheAPIObj() *swagger.Repository {
	return &swagger.Repository{
		Location: "us-east1-a",
		Name:     "my-repo",
		Mode:     "pull-through-cache",
		UpstreamRegistry: &swagger.UpstreamRegistry{
			Provider: "docker-hub",
			Url:      "https://registry-1.docker.io",
			UpstreamRegistryCredentials: &swagger.UpstreamRegistryCredentials{
				Username: "user",
				Password: "", // never echoed by the API
			},
		},
	}
}

// Test_repositoryToResourceModel_preservesWriteOnlyCredentials checks that the
// transform sources location/name/mode/provider/url from the API but preserves the
// caller's credentials, since the API never returns the write-only password.
func Test_repositoryToResourceModel_preservesWriteOnlyCredentials(t *testing.T) {
	model := &repositoryResourceModel{UpstreamRegistry: upstreamRegistryWithCreds()}

	repositoryToResourceModel(pullThroughCacheAPIObj(), model, "proj-123")

	if got := model.Location.ValueString(); got != "us-east1-a" {
		t.Errorf("location = %q, want %q (from API)", got, "us-east1-a")
	}
	if got := model.Name.ValueString(); got != "my-repo" {
		t.Errorf("name = %q, want %q (from API)", got, "my-repo")
	}
	if got := model.Mode.ValueString(); got != "pull-through-cache" {
		t.Errorf("mode = %q, want %q (from API)", got, "pull-through-cache")
	}
	if got := model.ProjectID.ValueString(); got != "proj-123" {
		t.Errorf("project_id = %q, want %q", got, "proj-123")
	}
	if model.UpstreamRegistry == nil {
		t.Fatal("upstream_registry was dropped")
	}
	if got := model.UpstreamRegistry.Provider.ValueString(); got != "docker-hub" {
		t.Errorf("provider = %q, want %q (from API)", got, "docker-hub")
	}
	if model.UpstreamRegistry.UpstreamRegistryCrdentials == nil {
		t.Fatal("credentials block was dropped; write-only creds must be preserved")
	}
	if got := model.UpstreamRegistry.UpstreamRegistryCrdentials.Password.ValueString(); got != "s3cr3t" {
		t.Errorf("password = %q, want preserved %q (API never echoes it)", got, "s3cr3t")
	}
}

// Test_repositoryToResourceModel_createReadIdentical checks that Create and Read
// converge on identical state for the same API object, each seeding the write-only
// credentials from its own source (plan vs prior state).
func Test_repositoryToResourceModel_createReadIdentical(t *testing.T) {
	apiObj := pullThroughCacheAPIObj()

	// Create seeds upstream_registry from the plan.
	createState := &repositoryResourceModel{UpstreamRegistry: upstreamRegistryWithCreds()}
	repositoryToResourceModel(apiObj, createState, "proj-123")

	// Read seeds upstream_registry from prior state (equal to what Create wrote).
	readState := &repositoryResourceModel{UpstreamRegistry: upstreamRegistryWithCreds()}
	repositoryToResourceModel(apiObj, readState, "proj-123")

	if !reflect.DeepEqual(createState, readState) {
		t.Errorf("Create and Read produced different state:\n create = %+v\n read   = %+v", createState, readState)
	}
}

// Test_repositoryToResourceModel_standardModeNoUpstream verifies that a standard
// (non-cache) repository, which the API returns without an upstream registry, maps
// to a nil upstream_registry regardless of what the model previously carried.
func Test_repositoryToResourceModel_standardModeNoUpstream(t *testing.T) {
	apiObj := &swagger.Repository{
		Location: "us-east1-a",
		Name:     "std-repo",
		Mode:     "standard",
	}
	model := &repositoryResourceModel{UpstreamRegistry: upstreamRegistryWithCreds()}

	repositoryToResourceModel(apiObj, model, "proj-123")

	if model.UpstreamRegistry != nil {
		t.Errorf("upstream_registry = %+v, want nil for a standard repository", model.UpstreamRegistry)
	}
}
