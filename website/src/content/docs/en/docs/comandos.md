---
title: Commands
description: Complete reference for all Pyahu CLI commands, with flags and examples.
---

This is the complete Pyahu CLI reference. Every command accepts the global flags from the
next section. Most support `--output json` for use in scripts.

```text
pyahu [comando] [flags]
```

## Global flags

Available on any command:

| Flag | Default | Description |
| --- | --- | --- |
| `-f, --file` | automatic discovery | Path to the project's `pyahu.yaml`. The global config is still applied. |
| `-o, --output` | `human` | Output format: `human` or `json`. |
| `--no-color` | `false` | Disable colors in the output. |
| `-q, --quiet` | `false` | Suppress non-essential output. |
| `-v, --verbose` | `false` | Show tool output (k3d, etc.). |
| `--no-input` | `false` | Never prompt interactively. |
| `--version` | — | Show version, commit, and build date. |
| `-h, --help` | — | Command help. |

The CLI searches for the stack file from the current directory upward, in this order: `pyahu.yaml`,
`pyahu.yml`, `.pyahu/stack.yaml`, `.pyahu/stack.yml`. Use `--file` only to point at another
path.

---

## Lifecycle

### `pyahu init`

Creates a `pyahu.yaml` from a preset.

| Flag | Default | Description |
| --- | --- | --- |
| `--preset` | `minimal` | Starting preset: `minimal` (PostgreSQL only) or `platform` (full stack). |
| `--force` | `false` | Overwrite an existing stack file. |

```bash
pyahu init --preset platform
pyahu init --preset minimal --force
pyahu init --preset platform -f infra/pyahu.yaml
```

### `pyahu up`

Creates the k3d cluster when needed and reconciles the Kubernetes resources. It is **idempotent**:
a second `pyahu up` converges or does nothing.

| Flag | Default | Description |
| --- | --- | --- |
| `--skip-wait` | `false` | Apply the resources without waiting for the services to become ready. |

Flow: preflight (doctor) → create/reuse the cluster → wait for the Kubernetes API → apply
the services → wait for readiness → print the summary.

```bash
pyahu up
pyahu up --skip-wait
pyahu up --output json
```

Human output at the end:

```text
Pyahu local stack is ready
cluster:    pyahu-local
namespace:  pyahu-local-dev
kubeconfig: /home/voce/.config/k3d/kubeconfig-pyahu-local.yaml

POSTGRES_URL                 postgresql://pyahu:pyahu_local@localhost:5432/app?sslmode=disable
ZITADEL_ISSUER               https://zitadel.localhost
...

next: eval "$(pyahu env)"
```

:::caution
Changing host ports after the cluster exists requires recreating the cluster. k3d fixes the
mappings at creation time. `pyahu up` detects missing mappings and asks for `pyahu down` followed
by `pyahu up`.
:::

### `pyahu down`

Removes the local Pyahu resources.

| Flag | Default | Description |
| --- | --- | --- |
| `--keep-cluster` | `false` | Remove only the stack namespace and keep the k3d cluster. |

```bash
pyahu down                 # deletes the entire k3d cluster
pyahu down --keep-cluster  # keeps the cluster, removes the namespace
```

### `pyahu doctor`

Checks dependencies and local ports before bringing up the stack. Works even without a stack file
(it uses defaults for the check).

```bash
pyahu doctor
pyahu doctor --output json
```

It checks:

- `k3d` installed on the `PATH`
- Docker or Podman running
- Other local clusters (k3d/Kind): **warning** only, does not fail
- Availability of the host ports for the enabled services (when the cluster does not exist yet)

```text
k3d                      ok    k3d is installed
container-runtime        ok    docker is available
local-clusters           ok    no other local Kubernetes clusters detected
port:postgres            ok    127.0.0.1:5432 is available
host                     ok    linux/amd64
```

---

## Inspection

### `pyahu status`

Shows the state of the cluster and of each service, including the pods.

```bash
pyahu status
pyahu status --output json
```

```text
cluster: pyahu-local
namespace: pyahu-local-dev
postgres   ready    1/1 ready
zitadel    ready    1/1 ready
kafka      waiting  starting
```

### `pyahu services`

Lists the enabled services, their state, and local endpoints. Aliases: `svc`, `ls`.

```bash
pyahu services
pyahu svc
pyahu services --output json
```

```text
cluster:   pyahu-local
namespace: pyahu-local-dev
state:     running

SERVICE        STATUS  VERSION  ENDPOINTS
postgres       ready   18.4     localhost:5432
zitadel        ready   v4.15.2  https://zitadel.localhost
rabbitmq       ready   4.3.2    localhost:5672, https://rabbitmq.localhost
kafka          ready   4.3.0    localhost:9092
kafka-connect  ready   3.5.2    http://localhost:8083
kafka-ui       ready   v1.5.0   https://kafka-ui.localhost
```

### `pyahu describe <service>`

Details for a service: status, endpoints (host + in-cluster), environment variables,
config details, and pods.

Valid services: `postgres`, `zitadel`, `rabbitmq`, `kafka`, `kafka-connect`, `kafka-ui`.

| Flag | Default | Description |
| --- | --- | --- |
| `--show-secrets` | `false` | Show secret values in the human output (masked by default). |

```bash
pyahu describe postgres
pyahu describe zitadel --show-secrets
pyahu describe kafka-connect --output json
```

:::note
In the human output, passwords and tokens appear as `<hidden>` and passwords in URLs become `hidden`.
Use `--show-secrets` or `pyahu env` when you need the real values.
:::

### `pyahu logs <service>`

Streams a service's logs.

| Flag | Default | Description |
| --- | --- | --- |
| `--follow` | `false` | Follow the logs in real time. |
| `--tail` | `100` | Number of initial lines to show. |

```bash
pyahu logs postgres --tail 50
pyahu logs zitadel --follow
pyahu logs kafka-connect --tail 200
```

---

## Connection and data

### `pyahu env`

Prints the connection variables for local apps.

| Flag | Default | Description |
| --- | --- | --- |
| `--format` | `shell` | `shell` (with `export`), `dotenv`, or `json`. |

```bash
pyahu env                 # export VAR='valor'
pyahu env --format dotenv # VAR=valor
pyahu env --format json
eval "$(pyahu env)"       # loads into the current shell
```

The variables cover each enabled service, for example `POSTGRES_URL`,
`RABBITMQ_URL`, `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_CONNECT_URL`, `ZITADEL_ISSUER`.

### `pyahu kubeconfig`

Prints the path of the local cluster's kubeconfig.

| Flag | Default | Description |
| --- | --- | --- |
| `--raw` | `false` | Write the kubeconfig content to stdout instead of the path. |

```bash
pyahu kubeconfig
pyahu kubeconfig --raw > kubeconfig.yaml
export KUBECONFIG="$(pyahu kubeconfig)"
```

### `pyahu backup postgres [database]`

Runs a real PostgreSQL dump on the primary pod straight into a file on the host
(`pg_dump --format=custom`). Without `[database]`, it uses the first configured database.

| Flag | Default | Description |
| --- | --- | --- |
| `--dir` | `.pyahu/backups` | Host directory for the `.dump` file. |

```bash
pyahu backup postgres app --dir ./backups
pyahu backup postgres            # uses the first configured database
```

The file is named `<stack>-<database>-<YYYYMMDD-HHMMSS>.dump` (UTC).

### `pyahu restore postgres [database]`

Restores a custom PostgreSQL dump from a local file or from `s3://`.

| Flag | Default | Description |
| --- | --- | --- |
| `--source` | — (required) | Path to the `.dump` file or `s3://` URI. |
| `--s3-endpoint-url` | — | S3-compatible endpoint for `s3://` sources. |
| `--clean` | `true` | Remove matching objects before restoring. |
| `--yes` | `false` | Confirm the destructive restore without a prompt. |

```bash
pyahu restore postgres app --source ./backups/pyahu-local-app-20260622-131500.dump
pyahu restore postgres app --source s3://meu-bucket/dev/app.dump --yes
pyahu restore postgres app \
  --source s3://bucket/app.dump \
  --s3-endpoint-url http://localhost:9000 \
  --yes
```

:::caution
With `--clean` (the default), the restore may drop existing objects. In interactive runs it
asks for confirmation; in scripts, with `--no-input`, or with non-human output, pass `--yes` on purpose.
`s3://` sources use `aws s3 cp` on the host.
:::

---

## Local TLS

### `pyahu certs status`

Shows the state of the local CA, the wildcard certificate, and the host trust.

```bash
pyahu certs status
pyahu certs status --output json
```

```text
local CA:      ~/.config/pyahu/certs/ca.crt
CA status:     valid until 2036-06-19
host trust:    trusted
certificate:   .pyahu/local/certs/localhost.crt
cert status:   valid until 2027-07-24
domains:       *.localhost, localhost, zitadel.localhost
```

### `pyahu certs trust`

Installs the local Pyahu CA into the host's trust store. On macOS it may prompt for a password. After that,
`curl` and browsers accept `https://zitadel.localhost` (and the other `*.localhost` UIs) without warnings.

```bash
pyahu certs trust
```

### `pyahu certs rotate`

Regenerates the local CA and the wildcard certificate. Run `pyahu certs trust` and `pyahu up` afterward
to re-trust the CA and update the TLS Secret in the cluster.

```bash
pyahu certs rotate
pyahu certs trust
pyahu up
```

More context in [Local certificates](/en/docs/certificados).

---

## Shell

### `pyahu completion [shell]`

Generates the autocomplete script. Supported shells: `bash`, `zsh`, `fish`, `powershell`.

```bash
pyahu completion zsh > "${fpath[1]}/_pyahu"
pyahu completion bash | sudo tee /etc/bash_completion.d/pyahu > /dev/null
pyahu completion fish > ~/.config/fish/completions/pyahu.fish
```

---

## JSON output for scripts

Almost all read commands support `--output json`:

```bash
pyahu doctor --output json
pyahu status --output json
pyahu services --output json
pyahu describe postgres --output json
pyahu env --format json
pyahu certs status --output json
```

## Recommended flow

```bash
pyahu init --preset platform
pyahu doctor
pyahu up
pyahu certs trust
pyahu services
eval "$(pyahu env)"
```
