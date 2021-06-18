terraform {
  required_providers {
    dokku = {
      version = "0.1"
      source  = "hashicorp.com/aaronstillwell/dokku"
    }
  }
}

provider "dokku" {
  ssh_host = "127.0.0.1"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "dokku-vagrant"
}

# data "dokku_apps" "all" {}

resource "dokku_app" "rails-app" {
  # This is currently the only supported resource, and these are the
  # only supported options.

  name = "rails-app-old"

  config_vars = {
    AWS_REGION                 = "eu-west-2"
    S3_DATA_BUCKET             = "app-data-source"
    ACTIVE_STORAGE_BUCKET_NAME = "active-storage"
  }

  domains = [
    "test-2.dokku.me"
  ]

  # TODO NGINX settings
}

# resource "dokku_postgres_service" "rails-postgres" {
#   # TODO
#   name          = "rails-postgres"
#   image         = ""
#   image_version = "12.0"
#   password      = ""
#   root_password = ""
# }

# resource "dokku_redis_service" "rails-redis" {
#   # TODO
#   name          = "rails-redis"
#   image         = ""
#   image_version = ""
#   password      = ""
#   root_password = ""
# }

# resource "dokku_postgres_service_link" "rails-postgres-link" {
#   app     = dokku_app.rails-app.id
#   service = dokku_postgres_service.rails-postgres.id

#   alias        = ""
#   query_string = ""
# }

# resource "dokku_service_link" "rails-redis-link" {
#   app     = dokku_app.rails-app.id
#   service = dokku_predis_link.rails-redis.id

#   alias        = ""
#   query_string = ""
# }
