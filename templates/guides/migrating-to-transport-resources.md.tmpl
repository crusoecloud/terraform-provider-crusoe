---
page_title: "Migrating to the transport resources"
subcategory: ""
description: |-
  Move from the deprecated InfiniBand resources and attributes to the transport equivalents.
---

# Migrating to the transport resources

Provider version v1.3.0 added the transport resources. They cover both InfiniBand
and RoCE fabrics. The InfiniBand-only resources and attributes they replace are
deprecated. The provider removes them in the next major version.

Every deprecated name still works. You can migrate one piece at a time.

## What changed

| Deprecated                                                                           | Replacement                                            |
| ------------------------------------------------------------------------------------ | ------------------------------------------------------ |
| `crusoe_ib_partition` resource                                                       | `crusoe_transport_partition` resource                  |
| `crusoe_ib_networks` data source                                                     | `crusoe_transport_networks` data source                |
| `ib_network_id` on `crusoe_ib_partition`                                             | `transport_network_id` on `crusoe_transport_partition` |
| `ib_partition_id` on `crusoe_compute_instance` (`host_channel_adapters`)             | `transport_partition_id`                               |
| `ib_partition_id` on `crusoe_compute_instance_by_template` (`host_channel_adapters`) | `transport_partition_id`                               |
| `ib_partition_id` on `crusoe_kubernetes_node_pool`                                   | `transport_partition_id`                               |
| `ib_partition` on `crusoe_instance_template`                                         | `transport_partition_id`                               |
| `ib_network_id` and `ib_partition_id` on the `crusoe_compute_instance` data source   | `transport_network_id` and `transport_partition_id`    |

## Move a partition to the new resource type

The `crusoe_ib_partition` and `crusoe_transport_partition` resources manage the
same partition. A `moved` block changes the resource type in state. Your partition
stays in place. Terraform does not destroy it and does not create a new one.

This needs Terraform CLI 1.8 or later and provider v1.4.0 or later.

1. Change the resource type and rename `ib_network_id` to `transport_network_id`.

   ```terraform
   resource "crusoe_transport_partition" "example" {
     name                 = "my-partition"
     transport_network_id = data.crusoe_transport_networks.example.transport_networks[0].id
   }
   ```

2. Add a `moved` block that points at the old address.

   ```terraform
   moved {
     from = crusoe_ib_partition.example
     to   = crusoe_transport_partition.example
   }
   ```

3. Run `terraform plan`. The plan reports a move and no other change. If the plan
   reports a destroy, stop and contact support@crusoecloud.com.

4. Run `terraform apply`.

5. Remove the `moved` block after every user of the configuration has applied it.

### On Terraform 1.7 or earlier

Terraform added moves between resource types in version 1.8. On an earlier version,
remove the old resource from state and import the partition under the new type. Both
steps only edit state. Neither step deletes the partition.

```terraform
removed {
  from = crusoe_ib_partition.example

  lifecycle {
    destroy = false
  }
}

import {
  to = crusoe_transport_partition.example
  id = "<partition-id>,<project-id>"
}
```

The import ID takes `<partition-id>` alone if the project comes from your Crusoe
configuration.

The equivalent commands are below. Run `terraform state rm` before any `apply` that
no longer has the `crusoe_ib_partition` block. An `apply` in between destroys the
partition.

```shell
terraform state rm crusoe_ib_partition.example
terraform import crusoe_transport_partition.example <partition-id>,<project-id>
terraform plan
```

## Rename the partition attributes on other resources

Both names address the same partition, so a rename plans as an update in place. The
provider does not replace the VM, the instance template, or the node pool.

Change `ib_partition_id` to `transport_partition_id` on `crusoe_compute_instance`
and `crusoe_compute_instance_by_template`:

```terraform
resource "crusoe_compute_instance" "example" {
  # ...

  host_channel_adapters = [
    {
      transport_partition_id = crusoe_transport_partition.example.id
    }
  ]
}
```

Change `ib_partition_id` to `transport_partition_id` on
`crusoe_kubernetes_node_pool`, and `ib_partition` to `transport_partition_id` on
`crusoe_instance_template`:

```terraform
resource "crusoe_instance_template" "example" {
  # ...

  transport_partition_id = crusoe_transport_partition.example.id
}
```

Set one name or the other. The provider rejects a plan that sets both names to
different partitions. Setting both names to the same partition stays valid.

Use provider v1.3.1 or later for these renames. Earlier versions plan a replacement
on `crusoe_instance_template` and `crusoe_kubernetes_node_pool`.

## Switch the data source

Replace `crusoe_ib_networks` with `crusoe_transport_networks`. The result attribute
changes name from `ib_networks` to `transport_networks`. The nested attributes stay
the same.

```terraform
data "crusoe_transport_networks" "example" {}

output "network_id" {
  value = data.crusoe_transport_networks.example.transport_networks[0].id
}
```
