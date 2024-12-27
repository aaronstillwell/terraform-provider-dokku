---
page_title: "Provider: Dokku"
description: |-
  The Dokku provider allows Terraform to manage resources on a Dokku host.
---

# Dokku Provider

The Dokku provider allows Terraform to manage resources on a Dokku host. Dokku is a Docker-powered Platform-as-a-Service (PaaS) that helps you manage the deployment of your applications.

This provider enables you to manage Dokku applications and their associated services including:
- Application configuration (domains, environment variables, ports)
- Database services (PostgreSQL, MySQL, Redis)
- Service linking (connecting applications to databases)

## Example Usage

```terraform
terraform {
  required_providers {
    dokku = {
      source  = "aaronstillwell/dokku"
      version = "> 0.5"

      # Note: if you're building this provider locally and developing/testing with this example,
      # use the below instead.
      #
      # source  = "hashicorp.com/aaronstillwell/dokku"
      # version = "0.1"
    }
  }
}

provider "dokku" {
  ssh_host = "127.0.0.1"
  ssh_user = "dokku"
  ssh_port = 8022

  # The SSH key should be that of a dokku user. Dokku users have dokku set as a
  # forced command - the provider will not attempt to explicitly specify the
  # dokku binary over SSH.
  #
  # This can be an absolute path OR just contain the SSH key inline.
  ssh_cert = "/home/user/dokku-vagrant"
  #ssh_passphrase = "optional"
}

# Creates a dokku app
resource "dokku_app" "rails-app" {
  name = "rails-app"

  config_vars = {
    AWS_REGION                 = "eu-west-2"
    S3_DATA_BUCKET             = "app-data-source"
    ACTIVE_STORAGE_BUCKET_NAME = "active-storage"
  }

  # Add domains to the app https://dokku.com/docs/configuration/domains/
  domains = [
    "test-2.dokku.me"
  ]

  # Customize herokuish buildpacks to be used by this app
  # https://dokku.com/docs/deployment/builders/herokuish-buildpacks/
  buildpacks = [
    "https://github.com/heroku/heroku-buildpack-nodejs.git",
    "https://github.com/heroku/heroku-buildpack-ruby.git"
  ]

  # Additional host -> container port mappings
  # https://dokku.com/docs/networking/port-management/
  #ports = ["tcp:25:25"]

  # You can customize the address you want your app to listen on - particularly
  # useful if you want your app to be only accessible over a private network
  # for example.
  # https://dokku.com/docs/configuration/nginx/#binding-to-specific-addresses
  #nginx_bind_address_ipv4 = "192.168.5.5"
  #nginx_bind_address_ipv6 = "2345:0425:2CA1:0000:0000:0567:5673:23b5"
}

# Below are examples of the creation of services & how to link them to the 
# app created above. This is dependent on the necessary dokku plugins being
# installed.

resource "dokku_postgres_service" "rails-postgres" {
  name          = "rails-postgres-11-test"

  # Optionally configure the image and version(tag) the service should use.
  # Check the plugin docs for defaults.
  #image         = "postgres"
  #image_version = "11.12"

  # start/stop the service
  #stopped = true
}

resource "dokku_redis_service" "rails-redis" {
  name          = "rails-redis"

  # Optionally configure the image and version(tag) the service should use.
  # Check the plugin docs for defaults.
  #image         = "redis"
  #image_version = "6.2.4"

  # start/stop the service
  #stopped = true
}

resource "dokku_postgres_service_link" "rails-postgres-link" {
  app     = dokku_app.rails-app.name
  service = dokku_postgres_service.rails-postgres.name

  # Alternative environment variable name to use for exposing to the app
  #alias        = "BLUE_DATABASE"
  # Ampersand delimited querystring arguments to append to the service link
  #query_string = "pool=5"
}

resource "dokku_redis_service_link" "rails-redis-link" {
  app     = dokku_app.rails-app.name
  service = dokku_redis_service.rails-redis.name

  # Alternative environment variable name to use for exposing to the app
  #alias        = "BLUE_DATABASE"
  # Ampersand delimited querystring arguments to append to the service link
  #query_string = "pool=5"
}

resource "dokku_mysql_service" "mysql-db" {
  name = "my-mysql-db"

  # Optionally configure the image and version(tag) the service should use.
  # Check the plugin docs for defaults.
  #image         = "mysql"
  #image_version = "8"

  # start/stop the service
  #stopped = true
}

# resource "dokku_mysql_service_link" "mysql-db-link" {
#   app     = dokku_app.rails-app.name
#   service = dokku_mysql_service.mysql-db

#   # Alternative environment variable name to use for exposing to the app
#   #alias        = "BLUE_DATABASE"
#   # Ampersand delimited querystring arguments to append to the service link
#   #query_string = "pool=5"
# }

# Clickhouse doesn't have some of the options as other services due to
# limitations in the plugin at the time of implementation
resource "dokku_clickhouse_service" "clickhouse" {
  name = "my-clickhouse"
  # start/stop the service
  #stopped = true
}

resource "dokku_clickhouse_service_link" "clickhouse-link" {
  app     = dokku_app.rails-app.name
  service = dokku_clickhouse_service.clickhouse.name

  # Alternative environment variable name to use for exposing to the app
  #alias        = "BLUE_DATABASE"
  # Ampersand delimited querystring arguments to append to the service link
  #query_string = "pool=5"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `ssh_host` (String) The hostname of your Dokku server. Can be set via DOKKU_SSH_HOST environment variable.

### Optional

- `fail_on_untested_version` (Boolean) Whether to fail if the Dokku version has not been tested with this provider. Defaults to true. Can be set via DOKKU_FAIL_ON_UNTESTED_VERSION environment variable.
- `skip_known_hosts_check` (Boolean) Whether to skip SSH known hosts verification. Defaults to false. Can be set via DOKKU_SKIP_KNOWN_HOSTS_CHECK environment variable.
- `ssh_cert` (String) Either a path to the SSH private key for connecting to your Dokku server OR the source for an SSH key directly. Can be set via DOKKU_SSH_CERT environment variable.
- `ssh_passphrase` (String) An optional passphrase to be used in conjunction with the provided SSH key.
- `ssh_port` (Number) The SSH port of your Dokku server. Defaults to 22. Can be set via DOKKU_SSH_PORT environment variable.
- `ssh_user` (String) The SSH user to connect to your Dokku server. Defaults to 'dokku'. Can be set via DOKKU_SSH_USER environment variable.