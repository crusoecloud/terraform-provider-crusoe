package common

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// A Crusoe Kubernetes version has up to three parts: an upstream semver, a
// Crusoe build component, and a bootstrap tarball URL —
// "1.35.5-cmk.22-https://example.com/bootstrap.tar.gz" is build 22 of upstream
// 1.35.5, with a tarball. Each part is separated from the last by a hyphen.
const (
	cmkBuildSeparator = "-cmk."
	versionSeparator  = "-"

	// versionPartsWithTarball mirrors the gateway's own threshold
	// (minPartsForTarballExtraction): splitting on the separator yields
	// ["<semver>", "cmk.<n>"] for a plain version, and a third part only when a
	// tarball URL follows. The URL may itself contain separators, so everything
	// from the third part on is tarball.
	versionPartsWithTarball = 3
)

// errUnexpectedK8sVersionValue marks a framework-supplied value that is not the
// string this custom type is built on. It cannot happen through the schema, so
// it is a programming error rather than anything a user can cause.
var errUnexpectedK8sVersionValue = errors.New("unexpected Kubernetes version value type")

// K8sVersionType is the string type for a Crusoe Kubernetes version, in which
// two different spellings can name the same version.
//
// Requests and responses do not use the same spelling, and neither side is
// wrong:
//
//   - A create takes an image display name, which always carries the build
//     component ("1.35.5-cmk.22") because the gateway resolves it by exact
//     display-name match. It may also carry a bootstrap tarball URL.
//   - A CMKv2 (pod) control plane reports the upstream semver alone
//     ("1.35.5"): kubernetes-manager stores major.minor plus patch, and the
//     node pool version-skew check requires exactly three dot-separated
//     components, so the build component cannot ride on the response.
//   - A CMKv1 control plane derives its version from the control plane image
//     and so echoes the full display name back. A node pool reports the
//     version its resolved worker image carries, never the tarball.
//
// Terraform holds a provider to returning the planned value for any attribute
// whose plan was known, so a response in the other spelling raises "Provider
// produced inconsistent result after apply". Semantic equality is the
// framework's mechanism for exactly this: when the prior and new values name
// the same version, the prior (planned, user-authored) value is preserved.
type K8sVersionType struct {
	basetypes.StringType
}

var (
	_ basetypes.StringTypable                    = K8sVersionType{}
	_ basetypes.StringValuableWithSemanticEquals = K8sVersion{}
)

func (t K8sVersionType) String() string {
	return "common.K8sVersionType"
}

func (t K8sVersionType) Equal(o attr.Type) bool {
	other, ok := o.(K8sVersionType)
	if !ok {
		return false
	}

	return t.StringType.Equal(other.StringType)
}

func (t K8sVersionType) ValueFromString(_ context.Context, in basetypes.StringValue) (
	basetypes.StringValuable, diag.Diagnostics,
) {
	return K8sVersion{StringValue: in}, nil
}

func (t K8sVersionType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("unable to convert Kubernetes version from Terraform value: %w", err)
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("%w: expected a string, got %T", errUnexpectedK8sVersionValue, attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("%w: %v", errUnexpectedK8sVersionValue, diags)
	}

	return stringValuable, nil
}

func (t K8sVersionType) ValueType(context.Context) attr.Value {
	return K8sVersion{}
}

// K8sVersion is a Crusoe Kubernetes version value. See K8sVersionType.
type K8sVersion struct {
	basetypes.StringValue
}

// NewK8sVersionValue, NewK8sVersionNull, and NewK8sVersionUnknown mirror the
// types.String* constructors for the custom type.
func NewK8sVersionValue(value string) K8sVersion {
	return K8sVersion{StringValue: basetypes.NewStringValue(value)}
}

func NewK8sVersionNull() K8sVersion {
	return K8sVersion{StringValue: basetypes.NewStringNull()}
}

func NewK8sVersionUnknown() K8sVersion {
	return K8sVersion{StringValue: basetypes.NewStringUnknown()}
}

func (v K8sVersion) Type(context.Context) attr.Type {
	return K8sVersionType{}
}

func (v K8sVersion) Equal(o attr.Value) bool {
	other, ok := o.(K8sVersion)
	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals reports whether the receiver (the prior value) and
// newValue name the same Kubernetes version. Returning true preserves the prior
// value, which is what keeps a user's spelling in state.
func (v K8sVersion) StringSemanticEquals(_ context.Context, newValuable basetypes.StringValuable) (
	bool, diag.Diagnostics,
) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(K8sVersion)
	if !ok {
		diags.AddError(
			"Semantic equality check error",
			fmt.Sprintf("Expected a common.K8sVersion value, got %T. Please report this to the provider developers.",
				newValuable),
		)

		return false, diags
	}

	return SameK8sVersion(v.ValueString(), newValue.ValueString()), diags
}

// SameK8sVersion reports whether two Crusoe Kubernetes version strings name the
// same version.
//
// Only the differences the API itself introduces are ignored, and only in the
// direction it introduces them:
//
//   - A tarball URL is dropped, because a version resolves to an image and the
//     image carries no tarball. So "1.35.5-cmk.22-https://…" and
//     "1.35.5-cmk.22" are the same version.
//   - The build component is dropped by a CMKv2 control plane. So
//     "1.35.5-cmk.22" and "1.35.5" are the same version, but only when the
//     shorter spelling is exactly the longer one's upstream semver.
//
// Everything else is a real difference. In particular "1.35.5-cmk.22" and
// "1.35.5-cmk.23" are NOT equal: two builds of one upstream release are
// different images, and an upgrade between them is a change the customer
// should see. Reducing both sides to their upstream semver would have made
// them equal and hidden it.
func SameK8sVersion(a, b string) bool {
	if a == b {
		return true
	}

	// An empty version says nothing about which version is meant, so it can
	// only equal another empty one — already handled above.
	if a == "" || b == "" {
		return false
	}

	a, b = withoutTarball(a), withoutTarball(b)

	return a == b || a == upstreamSemver(b) || b == upstreamSemver(a)
}

// withoutTarball drops a bootstrap tarball URL, matching how the gateway splits
// one off (checkImageVersionForBootstrapTarball).
func withoutTarball(version string) string {
	parts := strings.Split(version, versionSeparator)
	if len(parts) < versionPartsWithTarball {
		return version
	}

	return strings.Join(parts[:versionPartsWithTarball-1], versionSeparator)
}

// upstreamSemver drops the Crusoe build component, leaving the upstream
// Kubernetes version: "1.35.5-cmk.22" becomes "1.35.5". A version that carries
// no build component is returned unchanged.
func upstreamSemver(version string) string {
	if idx := strings.Index(version, cmkBuildSeparator); idx >= 0 {
		return version[:idx]
	}

	return version
}
