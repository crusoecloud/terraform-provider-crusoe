package disk

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// TestDisksDataSourceSchema_BlockSizeDeprecated verifies the data source's block_size
// attribute still tells the reader that block_size is deprecated (CCX-3067), but does so
// through its description rather than a DeprecationMessage.
//
// The attribute is Computed. Terraform propagates an attribute deprecation to every
// reference to a containing object, so a DeprecationMessage here warns anyone who passes a
// whole disk to an output, about a read-only attribute they cannot remove from their
// configuration. The disk resource keeps its DeprecationMessage, because there block_size
// is an attribute the practitioner writes.
func TestDisksDataSourceSchema_BlockSizeDeprecated(t *testing.T) {
	ctx := context.Background()
	schemaResp := &datasource.SchemaResponse{}
	NewDisksDataSource().Schema(ctx, datasource.SchemaRequest{}, schemaResp)

	disks, ok := schemaResp.Schema.Attributes["disks"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("disks attribute not found or not a ListNestedAttribute")
	}
	blockSize, ok := disks.NestedObject.Attributes["block_size"].(schema.Int64Attribute)
	if !ok {
		t.Fatal("block_size nested attribute not found or not an Int64Attribute")
	}

	if blockSize.DeprecationMessage != "" {
		t.Error("data source block_size should not have a DeprecationMessage: it is Computed, " +
			"so the warning propagates to every reference to a containing disk and the reader " +
			"has nothing to act on")
	}
	if !strings.Contains(blockSize.MarkdownDescription, blockSizeDeprecationMessage) {
		t.Error("data source block_size description should carry the deprecation notice")
	}
}
