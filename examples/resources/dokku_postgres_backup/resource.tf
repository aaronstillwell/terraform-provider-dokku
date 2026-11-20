# Requires a provider build containing dokku_postgres_backup and
# Terraform/OpenTofu >= 1.11. Configure the Dokku and 1Password providers separately.
# The 1Password login item stores the access-key ID in username and secret in password.
ephemeral "onepassword_item" "r2_backup" {
  vault = var.onepassword_vault_id
  uuid  = var.r2_backup_item_id
}

resource "dokku_postgres_service" "app" {
  name = "app-db"
}

resource "dokku_postgres_backup" "app" {
  service_name = dokku_postgres_service.app.name

  bucket            = "database-backups"
  endpoint_url      = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
  region            = "auto"
  signature_version = "s3v4"
  schedule          = "0 3 * * *"

  access_key_id_wo     = ephemeral.onepassword_item.r2_backup.username
  secret_access_key_wo = ephemeral.onepassword_item.r2_backup.password
  credentials_version  = 1
}
