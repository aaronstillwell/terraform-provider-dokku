---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "dokku_clickhouse_service Resource - terraform-provider-dokku"
subcategory: ""
description: |-
  Manages a ClickHouse service in Dokku. Requires the ClickHouse Dokku plugin to be installed.
---

# dokku_clickhouse_service (Resource)

Manages a ClickHouse service in Dokku. Requires the ClickHouse Dokku plugin to be installed.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The name of the ClickHouse service.

### Optional

- `stopped` (Boolean) Whether the ClickHouse service is stopped. When true, the database service will not be running but data will be preserved.

### Read-Only

- `id` (String) The ID of this resource.