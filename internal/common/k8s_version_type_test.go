package common

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const tarballSuffix = "-https://example.com/bootstrap-v2.tar.gz"

func TestSameK8sVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want bool
	}{
		// The differences the API itself introduces.
		{"cmkv2 drops the build component", "1.35.5-cmk.22", "1.35.5", true},
		{"and in the other direction", "1.35.5", "1.35.5-cmk.22", true},
		{"cmkv1 echoes the display name", "1.35.5-cmk.22", "1.35.5-cmk.22", true},
		{"tarball dropped on resolution", "1.35.5-cmk.22" + tarballSuffix, "1.35.5-cmk.22", true},
		{"tarball dropped and cmkv2 shortens", "1.35.5-cmk.22" + tarballSuffix, "1.35.5", true},

		// Real differences. The build-component case is the one a naive
		// "compare upstream semvers" rule would wrongly call equal.
		{"different build of one release", "1.35.5-cmk.22", "1.35.5-cmk.23", false},
		{"different patch", "1.35.5-cmk.22", "1.35.6", false},
		{"different minor", "1.35.5-cmk.22", "1.36.1", false},
		{"different patch, both spelled fully", "1.35.5-cmk.22", "1.35.6-cmk.22", false},

		// Degenerate input.
		{"both empty", "", "", true},
		{"one empty", "", "1.35.5", false},
		{"other empty", "1.35.5", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := SameK8sVersion(tt.a, tt.b); got != tt.want {
				t.Errorf("SameK8sVersion(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			// The relation must be symmetric: the framework may hold either
			// spelling as the prior value depending on whether it is comparing
			// a plan against a response or state against config.
			if got := SameK8sVersion(tt.b, tt.a); got != tt.want {
				t.Errorf("SameK8sVersion(%q, %q) = %v, want %v (not symmetric)", tt.b, tt.a, got, tt.want)
			}
		})
	}
}

// TestK8sVersionStringSemanticEquals checks the framework-facing entry point,
// which is what actually preserves the user's spelling.
func TestK8sVersionStringSemanticEquals(t *testing.T) {
	t.Parallel()

	prior := NewK8sVersionValue("1.35.5-cmk.22")

	equal, diags := prior.StringSemanticEquals(context.Background(), NewK8sVersionValue("1.35.5"))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !equal {
		t.Error("a cmkv2 response should be semantically equal to the configured display name")
	}

	equal, diags = prior.StringSemanticEquals(context.Background(), NewK8sVersionValue("1.36.1"))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if equal {
		t.Error("a different upstream version must not be semantically equal")
	}
}

// TestK8sVersionStringSemanticEqualsWrongType pins the diagnostic rather than a
// silent false, so a future refactor that hands in a plain string is loud.
func TestK8sVersionStringSemanticEqualsWrongType(t *testing.T) {
	t.Parallel()

	prior := NewK8sVersionValue("1.35.5-cmk.22")

	equal, diags := prior.StringSemanticEquals(context.Background(), basetypes.NewStringValue("1.35.5"))
	if !diags.HasError() {
		t.Error("expected an error diagnostic for a non-K8sVersion value")
	}
	if equal {
		t.Error("expected false alongside the error diagnostic")
	}
}

// TestK8sVersionTypeEqual guards the type identity the framework relies on to
// decide whether two schemas describe the same attribute.
func TestK8sVersionTypeEqual(t *testing.T) {
	t.Parallel()

	if !(K8sVersionType{}).Equal(K8sVersionType{}) {
		t.Error("K8sVersionType should equal itself")
	}
	if (K8sVersionType{}).Equal(basetypes.StringType{}) {
		t.Error("K8sVersionType should not equal a plain StringType")
	}
}

// TestSameK8sVersionPreReleaseSemver covers an upstream semver carrying a hyphen
// of its own. The schema validator rejects this spelling today, so it is not
// reachable from a configuration — but the earlier "first two hyphen-separated
// parts" rule reduced both of these to "1.35.5-rc.1" and called two different
// builds equal, which is the silent-wrong-answer class this guards against.
func TestSameK8sVersionPreReleaseSemver(t *testing.T) {
	t.Parallel()

	if SameK8sVersion("1.35.5-rc.1-cmk.22", "1.35.5-rc.1-cmk.23") {
		t.Error("two builds of one pre-release must not compare equal")
	}
	if !SameK8sVersion("1.35.5-rc.1-cmk.22"+tarballSuffix, "1.35.5-rc.1-cmk.22") {
		t.Error("a tarball suffix should still be dropped on a pre-release version")
	}
	if !SameK8sVersion("1.35.5-rc.1-cmk.22", "1.35.5-rc.1") {
		t.Error("dropping the build component should still compare equal")
	}
}
