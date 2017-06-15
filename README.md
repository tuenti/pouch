# The Pouch project

Pouch and friends are a set of tools to manage provisioning of secrets on
hosts based on the [AppRole](https://www.vaultproject.io/docs/auth/approle.html)
authentication method of [Vault](https://www.vaultproject.io).

_This project is under development and there may be changes on its commands and configurations_

Pouch encourages the application of the good practices of this
authentication method by using secret IDs with response wrapping.

The typical workflow to provision machines using Pouch consists on:

* Properly configure Vault roles and policies
* Install a machine with `pouch` as part of its initial provisioning
* Push wrapped secret ID to the machine
* `pouch` unwraps the secret and uses it to obtain the rest of its secrets

This workflow has some advantages:

* Secret IDs and tokens TTLs can be minimal
* Final secrets are only extracted from Vault by the host needing them

## Tools in the pouch project

* [`pouch`](https://github.com/tuenti/pouch/tree/master/cmd/pouch) is a daemon able to login with
  Vault using AppRole authentication method with wrapped secret IDs, it can
  request secrets and use them to fill templates.
* [`pouchctl`](https://github.com/tuenti/pouch/tree/master/cmd/pouchctl) is a cli tool that can be
  used to push wrapped secret ids to hosts using `pouch`.
* [`terraform-provisioner-vault-secret-id`](https://github.com/tuenti/pouch/tree/master/cmd/terraform-provisioner-vault-secret-id)
  is a Terraform plugin that provides a provisioner that can be used to push wrapped
  secret ids to hosts using `pouch`.
* [`approle-login`](https://github.com/tuenti/pouch/tree/master/cmd/approle-login) is a helper tool
  that can be used with other tools that use Vault as data source but don't implement the AppRole
  authentication backend.

## Credits & Contact

Pouch is a project by [Tuenti Technologies S.L.](http://github.com/tuenti)

You can follow Tuenti engineering team on Twitter [@tuentieng](http://twitter.com/tuentieng).

## License

`pouch` is available under the Apache License, Version 2.0. See LICENSE file
for more info.
