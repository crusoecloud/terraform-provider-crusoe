package disk

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

// TestDisksDataSourceSchema_BlockSizeDeprecated verifies the data source's
// block_size attribute is marked deprecated for consistency with the resource. It
// still reports the disk's actual block size; only the deprecation flag is new (CCX-3067).
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
	if blockSize.DeprecationMessage == "" {
		t.Error("data source block_size should have a DeprecationMessage")
	}
}
