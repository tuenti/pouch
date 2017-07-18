# pouch

`pouch` is a daemon that can be used with [AppRole](https://www.vaultproject.io/docs/auth/approle.html)
authentication method of [Vault](https://www.vaultproject.io) to get secrets
that can be used to fill configuration files.

When running `pouch` with systemd it can be used to control the lifecycle of
other units requiring secrets, this units can wait to `pouch` to notify the
readiness of the secrets and they can also be reloaded or restarted if
secrets change.

## Pouchfile

The `Pouchfile` is the configuration file of `pouch` and its only configuration
method. It is a YAML file with the following fields:

```
wrapped_secret_id_path: <path>
```
Path where `pouch` expects to find the wrapped secret ID, if this file is
empty or doesn't exist, `pouch` waits for it to contain a wrapped secret ID.

```
vault:
  address: <vault address>
  role_id: <role ID>
  secret_id: <secret ID>
  token: <vault token>
```
Vault configuration, `address` is required. For convenience authentication
using a role ID without secret ID, using a role ID with a fixed secret ID or
just a token are also supported. But its encouraged to use role ID with a
wrapped temporal secret ID.

```
systemd:
  enabled: <enable systemd integration>
```
Configuration of integration with systemd. By default `pouch` uses systemd
integration if it can detect it.

```
secrets:
  name:
    vault_url: <Vault HTTP API url>
    http_method: <HTTP method to use to request secrets>
    data:
      <key>: <value>
      <...>
    files:
    - path: <path to file to create>
      mode: <mode for the file and subdirectories if they are created>
      template: <inline template for the file>
      template_file: <path to file containing a template>
      notify:
      - <notifier>
      <...>
  <...>
```
Map of secrets to be retrieved from Vault using its [HTTP API](https://www.vaultproject.io/api/index.html).
Secrets are retrieved using the configured AppRole, so obviously this AppRole
needs to have permissions to do these requests. Requests are done using HTTP,
to the `vault_url` using the specified `http_method`.
Payload can be added to the request using the `data` field, any value is
allowed. Data `value` can be a [go template](https://golang.org/pkg/text/template),
in that case these functions are available:
* `env`: to get environment variables
* `hostname`: to get the hostname

Once retrieved a list of `files` are provisioned using the JSON response from
Vault.

```
notifiers:
  name:
    command: <command>
    timeout: <command timeout>
```
Map of notifiers that can be used to notify changes on files. It is intended
to reload services or any other required trigger. It is specified as a
`command` that is run inside a shell, and `timeout` as a maximum execution
time.

As an example:

```
wrapped_secret_id_path: /var/run/vault-wrapped-secret-id
vault:
  address: https://127.0.0.1:8200
  role_id: kubelet
systemd:
  enabled: false
secrets:
  kubelet_certs:
  - vault_url: /v1/kubernetes-pki/issue/kubelet
    http_method: POST
    files:
    - path: /etc/kubernetes/ssl/client.key
      mode: 0600
      template: |
        {{ .private_key }}
    - path: /etc/kubernetes/ssl/client.crt
      mode: 0600
      template: |
        {{ .certificate }}
    - path: /etc/kubernetes/ssl/ca.crt
      mode: 0600
      template: |
        {{ .issuing_ca }}

```

## Integration with systemd

`pouch` is better suited to work with systemd.

It supports `notify` service type. If it's started from a service unit
with `Type=notify`, it will notify its readiness only after all secrets
have been retrieved and files populated. This can be used to control when
other units can be started, a unit with `Requires=pouch.service` won't be
started till configuration files are ready.
