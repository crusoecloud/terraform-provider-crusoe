package vm

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const (
	DiskOS       = "os"
	StateRunning = "STATE_RUNNING"
)

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

func getVM(ctx context.Context, apiClient *swagger.APIClient, projectID, vmID string) (*swagger.InstanceV1Alpha5, error) {
	dataResp, httpResp, err := apiClient.VMsApi.GetInstance(ctx, projectID, vmID)
	if err != nil {
		return nil, fmt.Errorf("failed to find VM: %w", common.UnpackAPIError(err))
	}
	defer httpResp.Body.Close()

	return &dataResp, nil
}

// vmNetworkInterfacesToTerraformDataModel creates a slice of Terraform-compatible network
// interface datasource instances from Crusoe API network interfaces.
//
// In the case that a warning is returned because IP addresses are missing - which should never
// be the case - we still return a partial response that should be usable.
func vmNetworkInterfacesToTerraformDataModel(networkInterfaces []swagger.NetworkInterface) (interfaces []vmNetworkInterfaceDataModel, warning string) {
	for _, networkInterface := range networkInterfaces {
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
// interface resource instances from Crusoe API network interfaces.
func vmNetworkInterfacesToTerraformResourceModel(networkInterfaces []swagger.NetworkInterface) (networkInterfacesList types.List, warning string) {
	interfaces := make([]vmNetworkInterfaceResourceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		var publicIP swagger.PublicIpv4Address
		var privateIP swagger.PrivateIpv4Address
		if len(networkInterface.Ips) == 0 {
			warning = "At least one network interface is missing IP addresses. Please reach out to support@crusoecloud.com" +
				" and let us know."
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

	values, _ := types.ListValueFrom(context.Background(), vmNetworkInterfaceSchema, interfaces)

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

func vmDiskAttachmentToTerraformResourceModel(diskAttachments []swagger.DiskAttachment) (diskAttachmentsList types.List, diags diag.Diagnostics) {
	attachments := make([]vmDiskResourceModel, 0, len(diskAttachments))
	for _, diskAttachment := range diskAttachments {
		attachments = append(attachments, vmDiskResourceModel{
			ID:             diskAttachment.DiskId,
			AttachmentType: diskAttachment.AttachmentType,
			Mode:           diskAttachment.Mode,
		})
	}

	diskAttachmentsList, diags = types.ListValueFrom(context.Background(), vmDiskAttachmentSchema, attachments)

	return diskAttachmentsList, diags
}

func findInstance(ctx context.Context, client *swagger.APIClient, instanceID string) (*swagger.InstanceV1Alpha5, error) {
	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId: optional.EmptyString(),
	}

	projectsResp, projectHttpResp, err := client.ProjectsApi.ListProjects(ctx, opts)

	defer projectHttpResp.Body.Close()

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

func vmToTerraformResourceModel(instance *swagger.InstanceV1Alpha5, state *vmResourceModel) {
	state.ID = types.StringValue(instance.Id)
	state.Name = types.StringValue(instance.Name)
	state.Type = types.StringValue(instance.Type_)
	state.ProjectID = types.StringValue(instance.ProjectId)
	state.FQDN = types.StringValue(fmt.Sprintf("%s.%s.compute.internal", instance.Name, instance.Location))
	state.Location = types.StringValue(instance.Location)
	networkInterfaces, _ := vmNetworkInterfacesToTerraformResourceModel(instance.NetworkInterfaces)
	state.NetworkInterfaces = networkInterfaces

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
	if len(disks) > 0 {
		tDisks, _ := types.ListValueFrom(context.Background(), vmDiskAttachmentSchema, disks)
		state.Disks = tDisks
	} else {
		state.Disks = types.ListNull(vmDiskAttachmentSchema)
	}

	if len(instance.HostChannelAdapters) > 0 {
		state.HostChannelAdapters = vmHostChannelAdaptersToTerraformResourceModel(instance.HostChannelAdapters)
	} else {
		state.HostChannelAdapters = types.ListNull(vmHostChannelAdapterSchema)
	}
}
