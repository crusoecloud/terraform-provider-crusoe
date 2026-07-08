package load_balancer

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestHealthCheckToSwagger_NullUnknownReturnNil checks that a null or unknown
// health_check maps to a nil payload without panicking.
func TestHealthCheckToSwagger_NullUnknownReturnNil(t *testing.T) {
	for name, obj := range map[string]types.Object{
		"null":    types.ObjectNull(loadBalancerHealthCheckSchema.AttrTypes),
		"unknown": types.ObjectUnknown(loadBalancerHealthCheckSchema.AttrTypes),
	} {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			got := healthCheckToSwagger(context.Background(), obj, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != nil {
				t.Errorf("healthCheckToSwagger(%s) = %+v, want nil", name, got)
			}
		})
	}
}

// TestHealthCheckToSwagger_Values checks that a populated health_check is decoded
// into the swagger payload with unquoted values.
func TestHealthCheckToSwagger_Values(t *testing.T) {
	obj, d := types.ObjectValueFrom(context.Background(), loadBalancerHealthCheckSchema.AttrTypes, healthCheckOptionsResourceModel{
		Timeout:      types.StringValue("30s"),
		Port:         types.StringValue("8080"),
		Interval:     types.StringValue("10s"),
		SuccessCount: types.StringValue("3"),
		FailureCount: types.StringValue("2"),
	})
	if d.HasError() {
		t.Fatalf("building object: %v", d)
	}

	var diags diag.Diagnostics
	got := healthCheckToSwagger(context.Background(), obj, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got == nil {
		t.Fatal("healthCheckToSwagger returned nil for a populated health_check")
	}

	for field, tc := range map[string]struct{ got, want string }{
		"timeout":       {got.Timeout, "30s"},
		"port":          {got.Port, "8080"},
		"interval":      {got.Interval, "10s"},
		"success_count": {got.SuccessCount, "3"},
		"failure_count": {got.FailureCount, "2"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q (unquoted)", field, tc.got, tc.want)
		}
	}
}
