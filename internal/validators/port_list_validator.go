package internal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const (
	minPort = 1
	maxPort = 65535

	// portListDescription states the rules the API applies to a port list, so a plan
	// rejects a bad entry before the API does.
	portListDescription = "Comma-separated list of ports from 1 to 65535, port ranges such as 3000-8080, or `*` for all ports"
)

// PortListValidator validates a comma-separated list of ports and port ranges. It
// mirrors the API's validation: each entry is a port from 1 to 65535, a range whose
// bounds are both in that range and in ascending order, or `*` for every port. An
// empty string passes, since it has always meant every port.
type PortListValidator struct{}

func (v PortListValidator) Description(ctx context.Context) string {
	return portListDescription
}

func (v PortListValidator) MarkdownDescription(ctx context.Context) string {
	return portListDescription
}

//nolint:gocritic // Implements Terraform defined interface
func (v PortListValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	input := req.ConfigValue.ValueString()
	if strings.TrimSpace(input) == "" {
		return
	}

	for _, entry := range strings.Split(input, ",") {
		entry = strings.TrimSpace(entry)
		if !validPortEntry(entry) {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid port list",
				fmt.Sprintf("%q is not a valid port or port range. Each entry must be a port from %d to %d, "+
					"a range such as 3000-8080, or `*`.", entry, minPort, maxPort))

			return
		}
	}
}

// validPortEntry reports whether one entry of a port list is a port, a range, or the
// wildcard.
func validPortEntry(entry string) bool {
	if entry == "*" {
		return true
	}

	bounds := strings.Split(entry, "-")
	switch len(bounds) {
	case 1:
		_, ok := parsePort(bounds[0])

		return ok
	case 2:
		start, startOK := parsePort(bounds[0])
		end, endOK := parsePort(bounds[1])

		return startOK && endOK && start <= end
	default:
		return false
	}
}

// parsePort parses s as a port and reports whether it is within the port range. It
// ignores surrounding spaces, because the provider has always removed whitespace from a
// port list before sending it, so a range written as "3000 - 8080" stays valid.
func parsePort(s string) (int, bool) {
	port, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}

	return port, minPort <= port && port <= maxPort
}
