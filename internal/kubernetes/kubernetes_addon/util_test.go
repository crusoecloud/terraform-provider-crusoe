package kubernetes_addon

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const (
	testClusterID    = "11111111-1111-1111-1111-111111111111"
	testProjectID    = "22222222-2222-2222-2222-222222222222"
	testAltProjectID = "33333333-3333-3333-3333-333333333333"
)

func TestBuildAddonID(t *testing.T) {
	if got, want := buildAddonID(testClusterID, "cert-manager"), testClusterID+":cert-manager"; got != want {
		t.Errorf("buildAddonID = %q, want %q", got, want)
	}
}

func TestParseAddonImportID(t *testing.T) {
	client := &common.CrusoeClient{ProjectID: testProjectID}

	tests := []struct {
		name        string
		id          string
		wantCluster string
		wantAddon   string
		wantProject string
		wantErr     bool
	}{
		{"two parts uses client project", testClusterID + ":cert-manager", testClusterID, "cert-manager", testProjectID, false},
		{"explicit project", testClusterID + ":cert-manager," + testAltProjectID, testClusterID, "cert-manager", testAltProjectID, false},
		{"too few parts", testClusterID, "", "", "", true},
		{"colon project separator", testClusterID + ":cert-manager:" + testAltProjectID, "", "", "", true},
		{"too many project parts", testClusterID + ":cert-manager," + testProjectID + ",extra", "", "", "", true},
		{"non-uuid cluster", "not-a-uuid:cert-manager", "", "", "", true},
		{"empty addon type", testClusterID + ":," + testProjectID, "", "", "", true},
		{"non-uuid project", testClusterID + ":cert-manager,not-a-uuid", "", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cluster, addon, project, errMsg := parseAddonImportID(resource.ImportStateRequest{ID: tc.id}, client)
			if tc.wantErr {
				if errMsg == "" {
					t.Fatalf("expected an error for id %q, got none", tc.id)
				}

				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected error for id %q: %s", tc.id, errMsg)
			}
			if cluster != tc.wantCluster || addon != tc.wantAddon || project != tc.wantProject {
				t.Errorf("parseAddonImportID(%q) = (%q,%q,%q), want (%q,%q,%q)",
					tc.id, cluster, addon, project, tc.wantCluster, tc.wantAddon, tc.wantProject)
			}
		})
	}
}

func TestInstalledAddonToModel(t *testing.T) {
	addon := &swagger.KubernetesInstalledAddon{
		AddonType:        "cert-manager",
		Version:          "v1.19.1",
		InstalledVersion: "v1.19.0",
		Status:           "Running",
		Ready:            true,
		Category:         "optional",
	}

	var m kubernetesAddonResourceModel
	installedAddonToModel(addon, testClusterID, testProjectID, &m)

	if got, want := m.ID.ValueString(), testClusterID+":cert-manager"; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
	if m.ProjectID.ValueString() != testProjectID {
		t.Errorf("ProjectID = %q, want %q", m.ProjectID.ValueString(), testProjectID)
	}
	if m.ClusterID.ValueString() != testClusterID {
		t.Errorf("ClusterID = %q, want %q", m.ClusterID.ValueString(), testClusterID)
	}
	if m.AddonType.ValueString() != "cert-manager" {
		t.Errorf("AddonType = %q, want %q", m.AddonType.ValueString(), "cert-manager")
	}
	if m.Version.ValueString() != "v1.19.1" {
		t.Errorf("Version = %q, want %q", m.Version.ValueString(), "v1.19.1")
	}
	if m.InstalledVersion.ValueString() != "v1.19.0" {
		t.Errorf("InstalledVersion = %q, want %q", m.InstalledVersion.ValueString(), "v1.19.0")
	}
	if m.Status.ValueString() != "Running" {
		t.Errorf("Status = %q, want %q", m.Status.ValueString(), "Running")
	}
	if !m.Ready.ValueBool() {
		t.Error("Ready = false, want true")
	}
	if m.Category.ValueString() != "optional" {
		t.Errorf("Category = %q, want %q", m.Category.ValueString(), "optional")
	}
}

func TestInstalledAddonToModel_EmptyFieldsAreNull(t *testing.T) {
	// A freshly-created add-on has not reconciled yet: empty API strings must map
	// to null, not "".
	var m kubernetesAddonResourceModel
	installedAddonToModel(&swagger.KubernetesInstalledAddon{AddonType: "cert-manager"}, testClusterID, testProjectID, &m)

	for name, v := range map[string]bool{
		"installed_version": m.InstalledVersion.IsNull(),
		"status":            m.Status.IsNull(),
		"version":           m.Version.IsNull(),
		"category":          m.Category.IsNull(),
	} {
		if !v {
			t.Errorf("%s should be null when the API returns an empty string", name)
		}
	}
}

func TestAvailableAddonToModel(t *testing.T) {
	addon := &swagger.KubernetesAvailableAddon{
		AddonType:   "cert-manager",
		Description: "Automates TLS certs.",
		Category:    "optional",
		Removable:   true,
		Versions: []swagger.KubernetesAvailableAddonVersion{
			{Version: "v1.19.1"},
			{Version: "v1.18.0"},
		},
	}

	got, err := availableAddonToModel(addon)
	if err != nil {
		t.Fatalf("availableAddonToModel returned error: %v", err)
	}
	if got.AddonType.ValueString() != "cert-manager" {
		t.Errorf("AddonType = %q, want %q", got.AddonType.ValueString(), "cert-manager")
	}
	if got.Description.ValueString() != "Automates TLS certs." {
		t.Errorf("Description = %q", got.Description.ValueString())
	}
	if got.Category.ValueString() != "optional" {
		t.Errorf("Category = %q, want %q", got.Category.ValueString(), "optional")
	}
	if !got.Removable.ValueBool() {
		t.Error("Removable = false, want true")
	}

	var versions []string
	if diags := got.Versions.ElementsAs(context.Background(), &versions, false); diags.HasError() {
		t.Fatalf("versions ElementsAs: %v", diags)
	}
	if want := []string{"v1.19.1", "v1.18.0"}; !reflect.DeepEqual(versions, want) {
		t.Errorf("versions = %v, want %v", versions, want)
	}
}

func TestSchemasMarkedLimitedAvailability(t *testing.T) {
	ctx := context.Background()

	resourceResp := &resource.SchemaResponse{}
	NewKubernetesAddonResource().Schema(ctx, resource.SchemaRequest{}, resourceResp)

	addonsResp := &datasource.SchemaResponse{}
	NewKubernetesAddonsDataSource().Schema(ctx, datasource.SchemaRequest{}, addonsResp)

	availableResp := &datasource.SchemaResponse{}
	NewKubernetesAvailableAddonsDataSource().Schema(ctx, datasource.SchemaRequest{}, availableResp)

	for name, desc := range map[string]string{
		"crusoe_kubernetes_addon":                        resourceResp.Schema.MarkdownDescription,
		"crusoe_kubernetes_addons data source":           addonsResp.Schema.MarkdownDescription,
		"crusoe_kubernetes_available_addons data source": availableResp.Schema.MarkdownDescription,
	} {
		if !strings.HasPrefix(desc, common.LimitedAvailabilityMessage) {
			t.Errorf("%s description should start with common.LimitedAvailabilityMessage, got %q", name, desc)
		}
	}
}
