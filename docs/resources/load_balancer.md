---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "crusoe_load_balancer Resource - terraform-provider-crusoe"
subcategory: ""
description: |-
  This feature is currently in development. Reach out to support@crusoecloud.com with any questions.
---

# crusoe_load_balancer (Resource)

This feature is currently in development. Reach out to support@crusoecloud.com with any questions.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `algorithm` (String)
- `destinations` (Attributes List) (see [below for nested schema](#nestedatt--destinations))
- `location` (String)
- `name` (String)
- `network_interfaces` (Attributes List) (see [below for nested schema](#nestedatt--network_interfaces))
- `protocols` (List of String)

### Optional

- `health_check` (Attributes) (see [below for nested schema](#nestedatt--health_check))
- `project_id` (String)
- `type` (String)

### Read-Only

- `id` (String) The ID of this resource.
- `ips` (Attributes List) (see [below for nested schema](#nestedatt--ips))

<a id="nestedatt--destinations"></a>
### Nested Schema for `destinations`

Optional:

- `cidr` (String)
- `resource_id` (String)


<a id="nestedatt--network_interfaces"></a>
### Nested Schema for `network_interfaces`

Optional:

- `network` (String)
- `subnet` (String)


<a id="nestedatt--health_check"></a>
### Nested Schema for `health_check`

Read-Only:

- `failure_count` (String)
- `interval` (String)
- `port` (String)
- `success_count` (String)
- `timeout` (String)


<a id="nestedatt--ips"></a>
### Nested Schema for `ips`

Read-Only:

- `private_ipv4` (Attributes) (see [below for nested schema](#nestedatt--ips--private_ipv4))
- `public_ipv4` (Attributes) (see [below for nested schema](#nestedatt--ips--public_ipv4))

<a id="nestedatt--ips--private_ipv4"></a>
### Nested Schema for `ips.private_ipv4`

Read-Only:

- `address` (String)


<a id="nestedatt--ips--public_ipv4"></a>
### Nested Schema for `ips.public_ipv4`

Read-Only:

- `address` (String)
- `id` (String)
- `type` (String)
