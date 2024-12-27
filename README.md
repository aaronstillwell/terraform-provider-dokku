<h1 align="center">
  Terraform Provider for Dokku
</h1>

<h4 align="center">
  🚀 Manage your <a href="https://dokku.com/">Dokku</a> applications and services using Terraform!
</h4>

<p align="center">
  <a href="#key-features">Key Features</a> •
  <a href="#how-to-use">How To Use</a> •
  <a href="#developing">Developing</a> •
  <a href="#full-example">Full Example</a> •
  <a href="https://registry.terraform.io/providers/aaronstillwell/dokku/latest/docs">Terraform Registry</a> 
</p>

[![CircleCI](https://circleci.com/gh/aaronstillwell/terraform-provider-dokku.svg?style=shield)](https://circleci.com/gh/aaronstillwell/terraform-provider-dokku)
[![GitHub release](https://img.shields.io/github/v/release/aaronstillwell/terraform-provider-dokku?include_prereleases=&sort=semver)](https://github.com/aaronstillwell/terraform-provider-dokku/releases/)
[![License](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)


This is a terraform provider for provisioning apps on [Dokku](https://dokku.com/) installations. Not all configuration options are currently supported.

This provider is currently tested against Dokku >= v0.24 and <= 0.34, although can be forced to run against any version. [Read more](#Tested-dokku-versions).

## Key Features

- 📦 **Apps**: Create and manage Dokku applications
- 🗄️ **Databases**: Provision PostgreSQL, MySQL, and Redis services
- 🔗 **Service Links**: Connect your apps to databases
- 🌐 **Domains**: Configure custom domains for your apps
- 🔧 **Config**: Manage environment variables and app settings

## How To Use

1. Add the provider to your terraform block

```hcl
terraform {
  required_providers {
    dokku = {
      source  = "aaronstillwell/dokku"
      version = "> 0.4"
    }
  }
}
```

2. Initialise the provider with your host settings. The SSH key should be that of a [dokku user](https://dokku.com/docs/deployment/user-management/). Dokku users have dokku set as a forced command - the provider will not attempt to explicitly specify the dokku binary over SSH.

An SSH key can be provided as an absolute path or inline.

```hcl
provider "dokku" {
  ssh_host = "dokku.me"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "/home/user/.ssh/dokku-cert" # can also provide an SSH key directly as a string
  ssh_passphrase = "this is optional"
  # skip_known_hosts_check = true
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

The provider is currently tested against versions 0.24 through to 0.34 of dokku. Moving forward, it's likely the number of dokku versions being tested against may reduce slightly, with older versions being dropped as newer ones become available.

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

## Developing

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

## Full Example

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
