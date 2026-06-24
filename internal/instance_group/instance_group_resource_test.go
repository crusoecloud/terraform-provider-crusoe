package instance_group

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestInstanceGroupResource_Metadata(t *testing.T) {
	r := NewInstanceGroupResource()

	req := resource.MetadataRequest{
		ProviderTypeName: "crusoe",
	}
	resp := &resource.MetadataResponse{}

	r.Metadata(context.Background(), req, resp)

	expected := "crusoe_compute_instance_group"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}
