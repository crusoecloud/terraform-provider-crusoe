package vm

import (
	"context"
	"fmt"
	"strings"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

const (
	DiskOS       = "os"
	StateRunning = "STATE_RUNNING"
	StateStopped = "STATE_STOPPED"
	StateShutoff = "STATE_SHUTOFF"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (InstanceV1).
// Config-only inputs (ssh_key, image, custom_image, startup_script, shutdown_script,
// install_crusoe_watch_agent) are derived from InstancesPostRequestV1; nested attributes
// come from DiskAttachment, NetworkInterface, PublicIpv4Address, and PrivateIpv4Address.
const (
	apiDescID       = "ID of the VM."
	apiDescName     = "Name of the VM."
	apiDescType     = "Product name of the VM type."
	apiDescLocation = "Location the VM runs in."

	// disks (InstanceV1.disks -> DiskAttachment)
	apiDescDisks              = "Disks attached to the VM."
	apiDescDiskID             = "ID of the disk to attach."
	apiDescDiskAttachmentType = "Role the disk plays for the VM. Possible values: `os`, `data`."
	apiDescDiskMode           = "Access mode to attach the disk with. Possible values: `read-only`, `read-write`."

	// network_interfaces (InstanceV1.network_interfaces -> NetworkInterface)
	apiDescNetworkInterfaces = "Network interfaces attached to the VM."
	apiDescNIID              = "ID of the network interface."
	apiDescNIName            = "Name of the network interface."
	apiDescNINetwork         = "ID of the VPC network the interface is attached to."
	apiDescNISubnet          = "ID of the VPC subnet the interface is attached to."
	apiDescNIInterfaceType   = "Type of the network interface."
	apiDescExternalDNSName   = "External DNS name of the network interface."

	// public_ipv4 (NetworkInterface.ips[].public_ipv4 -> PublicIpv4Address)
	apiDescPublicIpv4ID      = "ID of the public IPv4 address."
	apiDescPublicIpv4Address = "Public IPv4 address."
	apiDescPublicIpv4Type    = "Allocation type of the public IPv4 address (for example, `dynamic`)."

	// private_ipv4 (NetworkInterface.ips[].private_ipv4 -> PrivateIpv4Address)
	apiDescPrivateIpv4Address = "Private IPv4 address."

	// host_channel_adapters (InstanceV1.host_channel_adapters)
	apiDescHostChannelAdapters = "Host channel adapters attached to the VM."

	apiDescNvlinkDomainID = "ID of the NVLink domain the VM belongs to, if any."

	// config-only inputs (InstancesPostRequestV1)
	apiDescSSHKey                  = "SSH public key to grant access to the new VM."
	apiDescImage                   = "Name of the OS image to use for the new VM. Either `image` or `custom_image` should be supplied, not both."
	apiDescCustomImage             = "ID of a custom image to use for the new VM. Either `image` or `custom_image` should be supplied, not both."
	apiDescStartupScript           = "Script to run when the VM starts."
	apiDescShutdownScript          = "Script to run when the VM shuts down."
	apiDescInstallCrusoeWatchAgent = "Whether to install the Crusoe Watch Agent on the VM. Defaults to true."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project that owns the VM. " + project.ProviderDescProjectIDFallback
	// reservation_id keeps the existing provider deprecation/behavior wording rather than
	// the spec text; the field is deprecated and its behavior is provider-specific.
	providerDescReservationID = "ID of the reservation to which the VM belongs. If not provided or null, the lowest-cost reservation will be used by default. To opt out of using a reservation, set this to an empty string."
	providerDescIBPartitionID = "Infiniband Partition ID."
)

// instanceTypeFamily returns the product-family prefix of an instance type,
// e.g. "c1a" for "c1a.2x". ok is false if the value isn't in "<family>.<size>" form.
func instanceTypeFamily(instanceType string) (family string, ok bool) {
	parts := strings.SplitN(instanceType, ".", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", false
	}

	return parts[0], true
}

var FQDNDeprecationMessage = common.FormatDeprecationWithReplacement("v0.5.29", "internal_dns_name")

var vmNetworkInterfaceSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":             types.StringType,
		"name":           types.StringType,
		"network":        types.StringType,
		"subnet":         types.StringType,
		"interface_type": types.StringType,
		"private_ipv4": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"address": types.StringType,
			},
		},
		"public_ipv4": types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"id":      types.StringType,
				"address": types.StringType,
				"type":    types.StringType,
			},
		},
	},
}

var vmDiskAttachmentSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":              types.StringType,
		"attachment_type": types.StringType,
		"mode":            types.StringType,
	},
}

var vmHostChannelAdapterSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"ib_partition_id": types.StringType,
	},
}

// getDisksDiff compares the disks attached to two VM resource models and returns
// a diff of disks defined by disk ID.
func getDisksDiff(origDisks, newDisks []vmDiskResourceModel) (disksAdded []swagger.DiskAttachment, disksRemoved []string) {
	for _, newDisk := range newDisks {
		matched := false
		for _, origDisk := range origDisks {
			if newDisk.ID == origDisk.ID && newDisk.Mode == origDisk.Mode {
				matched = true

				break
			}
		}
		if !matched {
			disksAdded = append(disksAdded, swagger.DiskAttachment{
				DiskId:         newDisk.ID,
				AttachmentType: newDisk.AttachmentType,
				Mode:           newDisk.Mode,
			})
		}
	}

	for _, origDisk := range origDisks {
		matched := false
		for _, newDisk := range newDisks {
			if newDisk.ID == origDisk.ID && newDisk.Mode == origDisk.Mode {
				matched = true

				break
			}
		}
		if !matched {
			disksRemoved = append(disksRemoved, origDisk.ID)
		}
	}

	return disksAdded, disksRemoved
}

func getVM(ctx context.Context, apiClient *swagger.APIClient, projectID, vmID string) (*swagger.InstanceV1, error) {
	dataResp, httpResp, err := apiClient.VMsApi.GetInstance(ctx, projectID, vmID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find VM: %w", common.UnpackAPIError(err))
	}

	return &dataResp, nil
}

// vmNetworkInterfacesToTerraformDataModel creates a slice of Terraform-compatible network
// interface datasource instances from Crusoe API network interfaces.
//
// In the case that a warning is returned because IP addresses are missing - which should never
// be the case - we still return a partial response that should be usable.
func vmNetworkInterfacesToTerraformDataModel(networkInterfaces []swagger.NetworkInterface) (interfaces []vmNetworkInterfaceDataModel, warning string) {
	for i := range networkInterfaces {
		networkInterface := networkInterfaces[i]
		var publicIP string
		var privateIP string
		if len(networkInterface.Ips) == 0 {
			warning = "At least one network interface is missing IP addresses. Please reach out to support@crusoecloud.com" +
				" and let us know."
		} else {
			publicIP = networkInterface.Ips[0].PublicIpv4.Address
			privateIP = networkInterface.Ips[0].PrivateIpv4.Address
		}

		interfaces = append(interfaces, vmNetworkInterfaceDataModel{
			Id:            networkInterface.Id,
			Name:          networkInterface.Name,
			Network:       networkInterface.Network,
			Subnet:        networkInterface.Subnet,
			InterfaceType: networkInterface.InterfaceType,
			PrivateIpv4:   vmIPv4{Address: privateIP},
			PublicIpv4:    vmIPv4{Address: publicIP},
		})
	}

	return interfaces, warning
}

// vmNetworkInterfacesToTerraformResourceModel creates a slice of Terraform-compatible network
// interface resource instances from Crusoe API network interfaces. If a plan is provided, we want to ensure that
// the ordering of interfaces in the plan is maintained.
func vmNetworkInterfacesToTerraformResourceModel(networkInterfaces []swagger.NetworkInterface) (networkInterfacesList types.List, warning diag.Diagnostics) {
	interfaces := make([]vmNetworkInterfaceResourceModel, 0, len(networkInterfaces))
	for i := range networkInterfaces {
		networkInterface := networkInterfaces[i]
		var publicIP swagger.PublicIpv4Address
		var privateIP swagger.PrivateIpv4Address
		if len(networkInterface.Ips) == 0 {
			warning.AddWarning("Unexpected state when unmarshaling network interfaces for Instance",
				"At least one network interface is missing IP addresses. Please reach out to "+
					"support@crusoecloud.com and let us know.")
		} else {
			publicIP = *networkInterface.Ips[0].PublicIpv4
			privateIP = *networkInterface.Ips[0].PrivateIpv4
		}

		interfaces = append(interfaces, vmNetworkInterfaceResourceModel{
			ID:            types.StringValue(networkInterface.Id),
			Name:          types.StringValue(networkInterface.Name),
			Network:       types.StringValue(networkInterface.Network),
			Subnet:        types.StringValue(networkInterface.Subnet),
			InterfaceType: types.StringValue(networkInterface.InterfaceType),
			PrivateIpv4: types.ObjectValueMust(
				map[string]attr.Type{"address": types.StringType},
				map[string]attr.Value{"address": types.StringValue(privateIP.Address)},
			),
			PublicIpv4: vmPublicIPv4ResourceModel{
				ID:      types.StringValue(publicIP.Id),
				Address: types.StringValue(publicIP.Address),
				Type:    types.StringValue(publicIP.Type_),
			},
		})
	}

	values, diags := types.ListValueFrom(context.Background(), vmNetworkInterfaceSchema, interfaces)
	warning.Append(diags...)

	return values, warning
}

// vmHostChannelAdaptersToTerraformResourceModel creates a slice of Terraform-compatible host channel adapters
// instances from Crusoe API host channel adapters interfaces.
func vmHostChannelAdaptersToTerraformResourceModel(hostChannelAdapters []swagger.HostChannelAdapter) (hcaList types.List) {
	hcas := make([]vmHostChannelAdapterResourceModel, 0, 1)
	if len(hostChannelAdapters) >= 1 {
		hcas = append(hcas, vmHostChannelAdapterResourceModel{IBPartitionID: hostChannelAdapters[0].IbPartitionId})
	}

	values, _ := types.ListValueFrom(context.Background(), vmHostChannelAdapterSchema, hcas)

	return values
}

func findInstance(ctx context.Context, client *swagger.APIClient, instanceID string) (*swagger.InstanceV1, error) {
	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId: optional.EmptyString(),
	}

	projectsResp, projectHttpResp, err := client.ProjectsApi.ListProjects(ctx, opts)
	if projectHttpResp != nil {
		defer projectHttpResp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query for projects: %w", err)
	}

	for _, project := range projectsResp.Items {
		vm, getVMErr := getVM(ctx, client, project.Id, instanceID)

		if getVMErr != nil {
			continue
		}

		if vm.Id == instanceID {
			return vm, nil
		}
	}

	return nil, errProjectNotFound
}

// takes instance and populates state pointer (could be empty or non-empty)
func vmToTerraformResourceModel(instance *swagger.InstanceV1, state *vmResourceModel) {
	state.ID = types.StringValue(instance.Id)
	state.Name = types.StringValue(instance.Name)
	state.Type = types.StringValue(instance.Type_)
	state.ProjectID = types.StringValue(instance.ProjectId)
	state.Location = types.StringValue(instance.Location)
	networkInterfaces, _ := vmNetworkInterfacesToTerraformResourceModel(instance.NetworkInterfaces)
	state.NetworkInterfaces = networkInterfaces
	state.ReservationID = types.StringValue(instance.ReservationId)
	state.NvlinkDomainID = types.StringValue(instance.NvlinkDomainId)

	internalDNSName := types.StringValue(fmt.Sprintf("%s.%s.compute.internal", instance.Name, instance.Location))
	state.InternalDNSName = internalDNSName
	state.FQDN = internalDNSName // fqdn is deprecated but kept for backward compatibility

	if len(instance.NetworkInterfaces) > 0 {
		state.ExternalDNSName = types.StringValue(instance.NetworkInterfaces[0].ExternalDnsName)
	} else {
		state.ExternalDNSName = types.StringNull()
	}

	if len(instance.Disks) > 0 {
		disks := make([]vmDiskResourceModel, 0, len(instance.Disks))
		for i := range instance.Disks {
			disk := instance.Disks[i]
			if disk.AttachmentType != DiskOS {
				disks = append(disks, vmDiskResourceModel{
					ID:             disk.Id,
					AttachmentType: disk.AttachmentType,
					Mode:           disk.Mode,
				})
			}
		}
		tDisks, _ := types.SetValueFrom(context.Background(), vmDiskAttachmentSchema, disks)
		state.Disks = tDisks
	} else {
		state.Disks = types.SetNull(vmDiskAttachmentSchema)
	}

	if len(instance.HostChannelAdapters) > 0 {
		state.HostChannelAdapters = vmHostChannelAdaptersToTerraformResourceModel(instance.HostChannelAdapters)
	} else {
		state.HostChannelAdapters = types.ListNull(vmHostChannelAdapterSchema)
	}

	// install_crusoe_watch_agent is not returned by the API (create-time-only flag);
	// preserve the existing state value, defaulting to true when empty (e.g., imports).
	if state.InstallCrusoeWatchAgent.IsNull() || state.InstallCrusoeWatchAgent.IsUnknown() {
		state.InstallCrusoeWatchAgent = types.BoolValue(true)
	}
}
