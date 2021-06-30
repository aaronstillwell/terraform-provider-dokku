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
  # These are currently the only supported options for an app.

  name = "rails-app"

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

resource "dokku_postgres_service" "rails-postgres" {
  name = "rails-postgres-11-test"
  // The image/version must already exist on the host via `docker pull`
  image         = "postgres"
  image_version = "11.12"
  # Not yet supported:
  # custom_env    = "FOO=BAR;FOO2=BAR2;"
  # password      = "test123"
  # root_password = "test123"
}

resource "dokku_redis_service" "rails-redis" {
  name = "rails-redis"
  // The image/version must already exist on the host via `docker pull`
  image         = "redis"
  image_version = "6.2.4"
  # Not yet supported
  # password      = ""
  # root_password = ""
}

resource "dokku_postgres_service_link" "rails-postgres-link" {
  app     = dokku_app.rails-app.name
  service = dokku_postgres_service.rails-postgres.name

  #alias = "TEST_DB_URL"
  # query_string = ""
}

resource "dokku_redis_service_link" "rails-redis-link" {
  app     = dokku_app.rails-app.name
  service = dokku_redis_service.rails-redis.name

  # alias = "TEST_REDIS_URL"
  # query_string = ""
}
