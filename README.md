# Terraform Provider Dokku

This is an experimental terraform provider for provisioning apps on [Dokku](https://dokku.com/) installations. Only a small subset of configuration options are currently supported.

This provider is not yet published, but the intent is to do so once a wider subset of configuration options are provided.

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
provider "dokku" {
  ssh_host = "dokku.me"
  ssh_user = "dokku"
  ssh_port = 8022
  ssh_cert = "~/.ssh/dokku-cert"
}

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
}

resource "dokku_postgres_service" "rails-postgres" {
  name          = "rails-postgres"
  image_version = "11.12"
}

resource "dokku_redis_service" "rails-redis" {
  name          = "rails-redis"
  image_version = "6.2.4"
}
```
