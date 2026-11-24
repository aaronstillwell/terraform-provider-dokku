# Requires Terraform/OpenTofu >= 1.11 for write-only sink DSNs (`*_wo`).
ephemeral "onepassword_item" "datadog_logs" {
  vault = var.onepassword_vault_id
  uuid  = var.datadog_item_id
}

resource "dokku_global_logs" "default" {
  max_size        = "50m"
  app_label_alias = "com.dokku.app-name"
  vector_image    = "timberio/vector:0.61.0-debian"
  vector_networks = ["observability"]
  vector_running  = true

  vector_sink_wo      = "datadog_logs://?api_key=${ephemeral.onepassword_item.datadog_logs.password}&site=datadoghq.com"
  vector_sink_version = 1
}
