resource "dokku_app" "api" {
  name       = "api"
  from_image = "ghcr.io/example/api:latest"
}

resource "dokku_app_logs" "api" {
  app                        = dokku_app.api.name
  max_size                   = "100m"
  app_label_alias            = "service"
  rebuild_on_max_size_change = true

  # Use vector_sink / vector_cron_sink for non-secret DSNs (or Terraform < 1.11),
  # or vector_sink_wo + vector_sink_version for DSNs containing API keys/tokens.
  vector_sink = "console://"
}
