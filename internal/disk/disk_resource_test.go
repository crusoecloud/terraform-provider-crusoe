package disk

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestDiskResourceSchema_BlockSizeDeprecated verifies block_size is marked
// deprecated on the resource while staying functional (Optional+Computed with its
// value validator) so existing customer configurations keep working (CCX-3067).
func TestDiskResourceSchema_BlockSizeDeprecated(t *testing.T) {
	ctx := context.Background()
	schemaResp := &resource.SchemaResponse{}
	NewDiskResource().Schema(ctx, resource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["block_size"].(schema.Int64Attribute)
	if !ok {
		t.Fatal("block_size attribute not found or not an Int64Attribute")
	}
	if attr.DeprecationMessage == "" {
		t.Error("block_size should have a DeprecationMessage")
	}
	if !attr.Optional || !attr.Computed {
		t.Errorf("block_size should remain Optional+Computed for backwards compatibility; got Optional=%v Computed=%v",
			attr.Optional, attr.Computed)
	}
	if len(attr.Validators) == 0 {
		t.Error("block_size should retain its value validator")
	}
}
