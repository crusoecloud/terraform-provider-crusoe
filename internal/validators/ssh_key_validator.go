package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"golang.org/x/crypto/ssh"
)

// SSHKeyValidator validates that a given SSH public key is valid.
type SSHKeyValidator struct{}

func (v SSHKeyValidator) Description(ctx context.Context) string {
	return "Input must be a valid SSH public key"
}

func (v SSHKeyValidator) MarkdownDescription(ctx context.Context) string {
	return "Input must be a valid SSH public key"
}

//nolint:gocritic // Implements Terraform defined interface
func (v SSHKeyValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// skip validation if the value is still unknown, which is the case for vars before evaluation
	if req.ConfigValue.IsUnknown() {
		return
	}

	input := req.ConfigValue.ValueString()

	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(input))
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid SSH Key",
			"The given SSH Key is not a valid public RSA or ECDSA key.")
	}
}
