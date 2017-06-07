# `pouchctl`

`pouchctl` is a helper tool that can be used to interact with AppRole
information in a Vault server, and to push wrapped secret IDs to
provisioned hosts. This can be used when the whole provisioning process is
not fully automated or when we want to make a host to request its secrets
again.

## Usage

Connection with Vault can be configured using `VAULT_ADDR` and `VAULT_TOKEN`
environment variables or with the `-address` and `-token` flags.

Some example commands:

To request the role ID of a role:
```
$ pouchctl -role testrole -show-role-id
RoleID: 9d8dd313-a422-9ee0-51fb-9fd7e7c9e4ed
Use -gen-secret to obtain a wrapped secret
```

To generate a wrapped secret ID for a role:
```
$ pouchctl -role testrole -gen-secret
4b877dda-bcfc-3f45-8695-584e8736596b
```

To generate a wrapped secret ID and send it to a provisioned host:
```
$ pouchctl -role testrole -gen-secret -copy-to ssh://root@host.example.com/var/run/wrapped-secret-id
```
