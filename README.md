# Terraform Provider Dokku

This is an experimental terraform provider for provisioning apps on [Dokku](https://dokku.com/) installations. Only a small subset of configuration options are currently supported, and bugs may exist.

This provider currently supports Dokku >= v0.24 and < 0.30, although can be forced to run against any version. [Read more](#Tested-dokku-versions).

## Getting started

1. Add the provider to your terraform block

```hcl
terraform {
  required_providers {
    dokku = {
      source  = "aaronstillwell/dokku"
    }
  }
}
```

2. Initialise the provider with your host settings. The SSH key should be that of a [dokku user](https://dokku.com/docs/deployment/user-management/). Dokku users have dokku set as a forced command - the provider will not attempt to explicitly specify the dokku binary over SSH.

```hcl
provider "dokku" {
  ssh_host = "dokku.me"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "/home/user/.ssh/dokku-cert"
}
```

3. Declare resources. See examples for more info.

```hcl
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

### Tested dokku versions

The provider is currently tested against versions 0.24 through to 0.29 of dokku. Moving forward, it's likely the number of dokku versions being tested against may reduce slightly, with older versions being dropped as newer ones become available.

The provider will check the version of dokku being used and by default will fail if a version outside this range is detected. This behaviour can be disabled with the `fail_on_untested_version` attribute. E.g

```hcl
provider "dokku" {
  ssh_host = "dokku.me"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "/home/user/.ssh/dokku-cert"
  
  # Tell the provider not to fail if a dokku version is detected that hasn't
  # been tested against the current version of the provider.
  fail_on_untested_version = false
}
```

## Developing the provider

The easiest way to develop this provider further is to set up a [vagrant](https://www.vagrantup.com/) box with the provided vagrantfile.

1. Run `vagrant up`. This will create an ubuntu VM with the prerequisites needed to develop the provider.
1. SSH into the VM with `vagrant ssh`
1. Navigate to where the source is mounted in the VM `cd /vagrant`

From here you can build & test the provider. This VM has dokku running in a docker container, which can be SSH'd into from the VM like any other dokku install.

Please raise an issue if you have any difficulties developing.

### Run acceptance tests locally

You can run the full acceptance test suite locally with `make testacc`, but note these take some time to run (~10 min).

It may be preferrable to run _only_ the test you're working on, with e.g `TF_ACC=1 go test terraform-provider-dokku/internal/provider -v -run TestSetAppConfigVars`

### Manual testing with a terraform config

The `examples` directory can be used for ad-hoc testing configs manually. 

1. Navigate to the examples dir while over SSH into the vagrant vm `cd /vagrant/examples/`
1. Build the provider for use locally with `./build.sh`
1. You can then use the terraform files in `examples` and run them against the local dokku instance with `terraform apply`.

## Examples

See also [examples/main.tf](./examples/main.tf) for a commented example.

```hcl
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
