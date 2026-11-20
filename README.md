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

This provider is currently tested against Dokku >= v0.30 and <= 0.38, although can be forced to run against any version. [Read more](#Tested-dokku-versions).

**Proxy compatibility:** The `dokku_app` resource assumes Dokku's nginx proxy. Caddy and other alternative proxies are not currently supported. App creation and refresh run `nginx:set` and `nginx:report` even when no nginx settings are explicitly configured. These operations may fail if the nginx plugin is disabled or unavailable, and the provider does not handle Caddy's rebuild requirements after domain or port changes.

## Key Features

- 📦 **Apps**: Create and manage Dokku applications
- 🗄️ **Databases**: Provision PostgreSQL, MySQL, and Redis services
- 🔗 **Service Links**: Connect your apps to databases
- 🌐 **Domains**: Configure custom domains for your apps
- 🔧 **Config**: Manage environment variables and app settings
- 📜 **Log Configuration & Drains**: Manage global and per-app Dokku log settings and Vector log drains

## Scheduled Postgres backups

`dokku_postgres_backup` manages the default scheduled backup independently of
`dokku_postgres_service`. It supports S3-compatible storage such as Cloudflare R2,
with a bucket, endpoint URL, region, signature version, and cron schedule.
See the [resource documentation](docs/resources/postgres_backup.md) and
[R2/1Password example](examples/resources/dokku_postgres_backup/resource.tf).
This resource requires a provider build containing this feature and
Terraform/OpenTofu >= 1.11 for native write-only credentials. Pass credentials
from ephemeral inputs or an ephemeral 1Password item; increment
`credentials_version` after rotating them. Schedule and storage changes update
in place, and removing the backup resource retains the database and stored objects.
Retention remains a bucket lifecycle policy.

## Log configuration & drains

`dokku_global_logs` and `dokku_app_logs` manage global and per-application Dokku
log settings and Vector log drains targeting Dokku `0.38.x`.
See the [global logs documentation](docs/resources/global_logs.md)
([example](examples/resources/dokku_global_logs/resource.tf)) and
[app logs documentation](docs/resources/app_logs.md)
([example](examples/resources/dokku_app_logs/resource.tf)).
Both resources support dual sink attributes: use write-only `vector_sink_wo`
and `vector_cron_sink_wo` (with `*_version` and Terraform/OpenTofu >= 1.11) for
DSNs containing API keys or tokens, or `vector_sink` and `vector_cron_sink` for
non-secret DSNs or older Terraform versions. `dokku_global_logs` also manages
`vector_networks` and optional `vector_running` lifecycle control (`logs:vector-start`
/ `logs:vector-stop`, restarting Vector when `vector_image` or `vector_networks`
change while running).

## How To Use

1. Add the provider to your terraform block

```hcl
terraform {
  required_providers {
    dokku = {
      source  = "aaronstillwell/dokku"
      version = "> 0.5"
    }
  }
}
```

2. Initialise the provider with your host settings. The SSH key should be that of a [dokku user](https://dokku.com/docs/deployment/user-management/). Dokku users have dokku set as a forced command - the provider will not attempt to explicitly specify the dokku binary over SSH.

An SSH key can be provided as an absolute path or inline. If omitted, the provider resolves `~/.ssh/config` via `ssh -G` (`IdentityFile` including `.pub` files matched against the SSH agent such as 1Password bookmarks, `IdentitiesOnly`, and `IdentityAgent`), falling back to active SSH agent keys and `~/.ssh/id_{ed25519,rsa,ecdsa,dsa}`.

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

The provider is currently tested against versions 0.30 through to 0.38 of dokku. Moving forward, it's likely the number of dokku versions being tested will change, with older versions being dropped as newer ones become available.

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

The backup and logs contract tests (`TestBackupWriteOnlyCLI`,
`TestBackupWriteOnlyUnsupportedCLI`, and `TestLogsWriteOnlyCLI`) run with
`go test ./...` and need no Dokku host or cloud credentials. They use an
in-process SSH server and installed Terraform and OpenTofu CLIs to exercise
real provider RPCs. Install CLI versions >= 1.11; missing CLIs are skipped.
Set `DOKKU_BACKUP_TEST_CLIS` to a platform path-separated list of binaries to
require a compatibility matrix (used by both backup and logs contract tests).
Set `DOKKU_BACKUP_TEST_OLD_CLIS` to explicitly test rejection of unsupported
clients in the backup contract tests. Use `go test -race ./...` for the race
checks.

Regenerate documentation with tfplugindocs v0.23.0 (requires Go >= 1.24):

```sh
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.23.0 generate --provider-name terraform-provider-dokku
```

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
