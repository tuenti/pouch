# `approle-login`

`approle-login` is a helper tool that can be used with other tools
that use Vault as data source, but don't implement the AppRole authentication
backend.

`approle-login` just logins using AppRole authentication and writes the token
to a file that can be used as an environment file.

## Usage

Connection with Vault can be configured using `VAULT_ADDR` or with the
`-address` flag.

It can be used with AppRoles that require secret ID and role ID or only
role ID.

Some example commands:

To login using just a role ID and writing the token to a file:
```
$ approle-login -role-id 9d8dd313-a422-9ee0-51fb-9fd7e7c9e4ed -output /tmp/vault-token
```

To login with a role ID and a wrapped secret ID stored in a file as could
be provided by `pouchctl`:
```
$ approle-login -role-id 287757ec-3eb1-f34c-644b-b6eded5669f8 -wrapped-secret-id-path /var/run/wrapped-secret-id
VAULT_TOKEN=06db356a-a32a-3e0d-f190-1d35f4bd3e9b
```
