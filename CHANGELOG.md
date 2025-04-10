## 0.5.27

ENHANCEMENTS:

* Allow fallback project IDs to be used with the VPC network resource

## 0.5.26

ENHANCEMENTS:

* N/A

BUG FIXES:

* N/A

## 0.5.25

ENHANCEMENTS:

* Adds CMK node pool version specification.

BUG FIXES:

* Removes deprecated CMK cluster configuration parameter.

## 0.5.24 (January 9, 2025)

ENHANCEMENTS:

* Adds initial support for Crusoe Managed Kubernetes (CMK) clusters and node pools.

## 0.5.23 (December 5, 2024)

ENHANCEMENTS:

* Adds placement policy for instance templates.
* Adds maintenance policy for instances and instance templates.

BUG FIXES:

* Fixes an issue with creating a VM with disks set to an empty list.

## 0.5.22 (October 22, 2024)
BUG FIXES:

* Fixes representation of disks to be order-agnostic. 

## 0.5.21 (August 16, 2024)

BUG FIXES:

* Fixes an issue with conversion of disk size from GiB to TiB.

## 0.5.20 (August 13, 2024)

ENHANCEMENTS:

* Upgraded GoReleaser to V2.

## 0.5.19 (July 19, 2024)

ENHANCEMENTS:

* Improved functionality when associating resources with a reservation ID.
* Adds block size specification for disks.

BUG FIXES:

* Implements retry on HTTP errors to fix intermittent failures that occurred when polling for an operation result, causing the local TF state to be out of sync with the created resources.

## 0.5.18 (May 28, 2024)

NEW FEATURES:

* Support associating VMs and instance templates with reservations.

BUG FIXES:

* Handle multiple network interfaces with equal or unspecified subnets.

## 0.5.17 (May 14, 2024)

BUG FIXES:

* Fixes an issue where network interfaces were not saved if they were not specified.

## 0.5.16 (May 13, 2024)

ENHANCEMENTS:

* Support multiple network interfaces when creating a VM.

## 0.5.15 (April 18, 2024)

ENHANCEMENTS:

* Upgraded go version to 1.21.

BUG FIXES:

* Fixes a separate issue with ordering of disks introduced in v0.5.14.

## 0.5.14 (April 15, 2024)

BUG FIXES:

* Fixes ordering of disks attached to VMs.

## 0.5.13 (April 9, 2024)

NEW FEATURES:

* Support for instance templates and creating VMs with an instance template.  Please contact [support](mailto:support@crusoecloud.com) to enable this feature for your organization.

ENHANCEMENTS:
* Support state upgrades for disks, VPC networks, VPC subnets, IB partitions and Firewall Rules.
* Update the warning message for setting a default project.

## 0.5.12 (March 15, 2024)

BUG FIXES:

* Fixes an issue with the version update warning.

## 0.5.11 (March 8, 2024)

ENHANCEMENTS:

* Add support for importing firewall rules and IB partitions.
* Provide notification when a new version of the Terraform provider is available.

## 0.5.10 (February 14, 2024)

ENHANCEMENTS:

* Add support for project deletion.

## 0.5.9 (February 7, 2024)

ENHANCEMENTS:

* Adds documentation on the resources and datasources supported by the Crusoe Cloud provider.
* Adds a Makefile rule (`make docs`) to autogenerate documentation based on the provider schemas using the `tfplugindocs` library.

## 0.5.8 (February 6, 2024)

ENHANCEMENTS:

* Automate tagging and releasing a new version of the Terraform provider upon merging of new changes.

## 0.5.7 (February 5, 2024)

ENHANCEMENTS:

* Update Crusoe Cloud API version to ensure FQDN values are not populated from the API.

## 0.5.6 (February 5, 2024)

BUG FIXES:

* If a VM has no disks attached, explicitly set the list of attachments to nil.

ENHANCEMENTS:

* Add support for migrating startup and shutdown scripts from older versions to newer versions.

## 0.5.5 (February 2, 2024)

ENHANCEMENTS:

* Add support for migrating the `tfstate` from older versions of the VM resource to newer versions of the schema with breaking changes.

## 0.5.4 (January 19, 2024)

ENHANCEMENTS:

* Add linting to the pipeline when merging new changes.

## 0.5.3 (January 8, 2024)

NEW FEATURES:

* Support for non-default VPC networks and subnets through new resources and datasources.
* VMs can now optionally be created in a non-default subnet.
  * This is done by specifying a subnet as an object in the network_interfaces 
  * `network_interfaces = { subnet = <subnet_id> }`
* Adds support for egress firewall rules.

## 0.5.2 (December 7, 2023)

BUG FIXES:

* Explicitly set the `host_channel_adapters` field to be null for non-IB enabled VMs.

## 0.5.1 (December 6, 2023)

BUG FIXES:

* Fixes an issue with Terraform reporting unexpected host_channel_adapters.

## 0.5.0 (December 5, 2023)

NEW FEATURES:

* Adds support for projects through the projects resource and datasource.
* Changes the version of the Crusoe Cloud API used from `v1alpha4` to `v1alpha5`.
* Adds support for specifying a disk `attachment_type` and `mode` when attaching a disk to a VM.

UPGRADE NOTES:

* Infiniband partitions are no longer specified using the `ib_partition_id` and instead as an object under the `host_channel_adapters` attribute.
  * ```host_channel_adapters = { ib_partition_id = <ib_partition_id> }```
* A project ID must be specified when creating resources. This can be specified in two ways:
  * Using the `project_id` top-level attribute of the resource of the resource being created. The project_id can be stored as a local variable, as suggested in the new `project-variable` example.
  * Having a `default_project` specified in the `~/.crusoe/config` file (for example, `default_project=my_cool_project_name`)

## 0.4.2 (October 24, 2023)

ENHANCEMENTS:

* Support updating firewall rules using the firewall rules resource.

## 0.4.1 (October 10, 2023)

ENHANCEMENTS:

* Errors from the Crusoe Cloud API are now unpacked to provide more informative error messages.

## 0.4.0 (September 20, 2023)

NEW FEATURES:

* Add support for static public IPs.

ENHANCEMENTS:

* Requires specifying `location` when creating a VM resource (previously optional).

UPGRADE NOTES:

* The location of the VM should be specified in the `.tf` file.

## 0.3.3 (August 24, 2023)

BUG FIXES:

* Skip validation in the SSH key and Regex validators if the value is still unknown (which is the case for variables before evaluation).

## 0.3.2 (August 22, 2023)

ENHANCEMENTS:

* Adds support for hot-attaching a disk to a VM.
* Does not require VMs to be in the stopped state when attaching disks.

## 0.3.1 (August 18, 2023)

NEW FEATURES:

* Adds support for specifying an image when creating a VM.
* Allow specifying an `image` as a top-level attribute for the VM resource which creates the VM with the specified curated image.

## 0.3.0 (August 7, 2023)

NEW FEATURES:

* Add support for Infiniband (IB) enabled VMs.
* Adds the IB Networks and Partitions datasources which can be used fetch existing network and partitions a user's Crusoe Cloud account.
* Adds the IB Partition datasource which can be used to create new partitions in an existing IB network.
* Allow specifying an `ib_partition_id` as a top-level attribute for the VM resource which creates the IB VM in that partition.

## 0.2.2 (June 15, 2023)

ENHANCEMENTS:

* Add support for `"*"` as a shorthand for specifying all ports for firewall rule sources and destinations.

## 0.2.1 (May 27, 2023)

BUG FIXES:

* Ignore null and unknown values in the storage size validator.

## 0.2.0 (May 27, 2023)

NEW FEATURES:

* Initial release!
