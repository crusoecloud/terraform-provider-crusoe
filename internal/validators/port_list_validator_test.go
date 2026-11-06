package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPortListValidator(t *testing.T) {
	tests := []struct {
		name    string
		value   types.String
		wantErr bool
	}{
		{"null is skipped", types.StringNull(), false},
		{"unknown is skipped", types.StringUnknown(), false},
		{"empty means all ports", types.StringValue(""), false},
		{"single port", types.StringValue("80"), false},
		{"several ports", types.StringValue("80,443"), false},
		{"full range", types.StringValue("1-65535"), false},
		{"wildcard", types.StringValue("*"), false},
		{"range", types.StringValue("3000-8080"), false},
		{"whitespace around entries", types.StringValue(" 80 , 443 "), false},
		{"whitespace around range bounds", types.StringValue("3000 - 8080"), false},
		{"wildcard mixed with ports", types.StringValue("*,80"), false},
		{"port zero in range", types.StringValue("0-65535"), true},
		{"port above max", types.StringValue("65536"), true},
		{"descending range", types.StringValue("443-80"), true},
		{"not a number", types.StringValue("abc"), true},
		{"trailing comma", types.StringValue("80,"), true},
		{"bare delimiter", types.StringValue("-"), true},
		{"three bounds", types.StringValue("1-2-3"), true},
		{"negative port", types.StringValue("-80"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validator.StringRequest{Path: path.Root("source_ports"), ConfigValue: tt.value}
			resp := &validator.StringResponse{}

			PortListValidator{}.ValidateString(context.Background(), req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Errorf("HasError() = %t, want %t (diagnostics: %v)", got, tt.wantErr, resp.Diagnostics)
			}
		})
	}
}
