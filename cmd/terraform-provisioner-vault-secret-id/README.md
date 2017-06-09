# terraform-provisioner-vault-secret-id

`terraform-provisioner-vault-secret-id` is a Terraform plugin that provides a
provisioner for wrapped secret ids as `pouch` expects them.

Its way of working is quite simple, it connects to vault to obtain the wrapped
secret ID and sends it as a file to the resource being provisioned.

It is implemented as a provisioner because implementing it as a data source
or as a resource would provoke uneeded requests to vault when running
`terraform plan` or `terraform apply`.

## Installation

To install the plugin, so `terraform` can use it, copy it to a known path, and
add it to terraform configuration file (`~/.terrraformrc` for Unix systems and
`%APPDATA%/terraform.rc` for Windows), for example if we installed the plugin
in /usr/bin, the configuration to add would be:

```
provisioners {
  vault-secret-id = "/usr/bin/terraform-provisioner-vault-secret-id"
}
```

If you already have a `provisioners` section in this file, add only the line
for this plugin.

## Usage

Once installed, this provisioner can be used as any other provisioner, for
example, to create a digital ocean droplet with a wrapped secret ID in
`/var/run/wrapped-secret-id`:

```
resource "digitalocean_droplet" "pouch-test" {
  image  = "ubuntu-16-04-x64"
  name   = "pouch-test"
  region = "ams3"
  size   = "512mb"
  ssh_keys = ["..."]

  provisioner "vault-secret-id" {
    role = "testrole"
	wrap_ttl = "30s"
    destination = "/var/run/wrapped-secret-id"
  }
}
```

It can also be used inside a `null_resource`, for example if we want to be
able to reprovision secrets without having to reprovision the whole resource:

```
variable "secrets_version" {
  type = "string"
}

resource "digitalocean_droplet" "pouch-test" {
  image  = "ubuntu-16-04-x64"
  name   = "pouch-test"
  region = "ams3"
  size   = "512mb"
  ssh_keys = ["..."]
}

resource "null_resource" "vault-test-secret-id" {
  triggers {
    version = "${var.secrets_version}"
  }

  connection {
    host = "${digitalocean_droplet.pouch-test.ipv4_address}"
  }

  provisioner "vault-secret-id" {
    role = "testrole"
    wrap_ttl = "30s"
    destination = "/var/run/wrapped-secret-id"
  }
}
```

Connection with Vault can be configured as usual using environment variables
`VAULT_ADDR` and `VAULT_TOKEN`.

### Argument reference

The following arguments are supported:

* `role`: Role for what to generate a secret ID (required)
* `destination`: Path of the file in the resource where secret will be stored (required)
* `wrap_ttl`: TTL for the wrapped secret ID
* `address`: URL of the root of the Vault server
* `token`: Token to use to authenticate to Vault
