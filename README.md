# Terraform Provider Dokku

This is an experimental terraform provider for provisioning apps on [Dokku](https://dokku.com/) installations. Only a small subset of configuration options are currently supported, and bugs may exist.

## Getting started

1. Add the provider to your terraform block

```
terraform {
  required_providers {
    dokku = {
      source  = "aaronstillwell/dokku"
    }
  }
}
```

2. Initialise the provider with your host settings. 
```
provider "dokku" {
  ssh_host = "dokku.me"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "/home/user/.ssh/dokku-cert"
}
```

3. Declare resources. See examples for more info.

```
resource "dokku_app" "rails-app" {
  name = "rails-app"

  config_vars = {
    AWS_REGION                 = "eu-west-2"
    S3_DATA_BUCKET             = "app-data-source"
    ACTIVE_STORAGE_BUCKET_NAME = "active-storage"
  }

  domains = [
    "test-2.dokku.me"
  ]

  buildpacks = [
    "https://github.com/heroku/heroku-buildpack-nodejs.git",
    "https://github.com/heroku/heroku-buildpack-ruby.git"
  ]
}
```

## Developing

The easiest way to develop this provider further is to set up a [vagrant](https://www.vagrantup.com/) box locally with Dokku installed. 

1. `vagrant up`
2. TODO SSH key config
3. Install Dokku using the install instructions on the Dokku website
4. Run `./build.sh` in `examples`
5. You can then use the terraform files in `examples` and run them against the vagrant box with `terraform apply`

## Building (default tf provider instructions)

Run the following command to build the provider

```shell
go build -o terraform-provider-dokku
```

## Test sample configuration

First, build and install the provider.

```shell
make install
```

Then, run the following command to initialize the workspace and apply the sample configuration.

```shell
terraform init && terraform apply
```

## Examples

```
terraform {
  required_providers {
    dokku = {
      source  = "aaronstillwell/dokku"
    }
  }
}

provider "dokku" {
  ssh_host = "dokku.me"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "/home/users/.ssh/dokku-cert"
}

# Create an app...
resource "dokku_app" "rails-app" {
  name = "rails-app"

  config_vars = {
    AWS_REGION                 = "eu-west-2"
    S3_DATA_BUCKET             = "app-data-source"
    ACTIVE_STORAGE_BUCKET_NAME = "active-storage"
  }

  domains = [
    "test-2.dokku.me"
  ]

  buildpacks = [
    "https://github.com/heroku/heroku-buildpack-nodejs.git",
    "https://github.com/heroku/heroku-buildpack-ruby.git"
  ]
}

# Create accompanying services...
resource "dokku_postgres_service" "rails-postgres" {
  name          = "rails-postgres"
  image_version = "11.12"
}

resource "dokku_redis_service" "rails-redis" {
  name          = "rails-redis"
}

# Link the services to the app...
resource "dokku_postgres_service_link" "rails-postgres-link" {
  app     = dokku_app.rails-app.name
  service = dokku_postgres_service.rails-postgres.name

  alias = "TEST_DB_URL"
  # query_string = ""
}

resource "dokku_redis_service_link" "rails-redis-link" {
  app     = dokku_app.rails-app.name
  service = dokku_redis_service.rails-redis.name

  alias = "TEST_REDIS_URL"
  # query_string = ""
}
```
