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
a second `pyahu up` converges or does nothing. Resources managed by Pyahu that are no longer in
the stack are removed; persistent volume claims are retained to protect local data.

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

POSTGRES_URL                 postgresql://pyahu:hidden@localhost:5432/app?sslmode=disable
POSTGRES_PASSWORD            <hidden>
ZITADEL_ISSUER               https://zitadel.localhost
...

next: eval "$(pyahu env)"
```

The `up` summary redacts passwords, tokens, private keys, and credentials inside
URLs in both human and JSON output. Use the explicit `pyahu env` command when an
application needs the real connection values.

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

SERVICE        STATUS  VERSION     ENDPOINTS
postgres       ready   18.4        localhost:5432
zitadel        ready   v4.15.2     https://zitadel.localhost
rabbitmq       ready   4.3.2       localhost:5672, https://rabbitmq.localhost
redis          ready   8.1-alpine  localhost:6379
kafka          ready   4.3.0       localhost:9092
kafka-connect  ready   3.5.2       http://localhost:8083
kafka-ui       ready   v1.5.0      https://kafka-ui.localhost
```

### `pyahu describe <service>`

Details for a service: status, endpoints (host + in-cluster), environment variables,
config details, and pods.

Valid services: `postgres`, `zitadel`, `rabbitmq`, `redis`, `kafka`, `kafka-connect`, `kafka-ui`.

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
`RABBITMQ_URL`, `REDIS_URL`, `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_CONNECT_URL`,
`ZITADEL_ISSUER`.

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

## Connectors

### `pyahu connectors apply`

Registers the connectors declared under `services.kafkaConnect.connectors` and
waits for their tasks to reach `RUNNING`.

Run it after the application that owns a connector's source has booted at least
once: an outbox connector reads a table, publication, or stream that does **not
exist** on a fresh cluster. The command is idempotent — it re-applies the Secret
and recreates the registration Job.

| Flag | Default | Description |
| --- | --- | --- |
| `--name` | *(all)* | Apply only the connector with this name. |

```bash
pyahu connectors apply
pyahu connectors apply --name checkout-outbox
```

### `pyahu connectors status`

Shows the state of each connector **and of each task**.

| Flag | Default | Description |
| --- | --- | --- |
| `--format` | `human` | `human` or `json`. |

```bash
pyahu connectors status
pyahu connectors status --format json
```

```text
CONNECTOR            TASK  STATE    DETAIL
allpick-core-outbox  -     RUNNING  source
                     0     RUNNING  -
checkout-outbox      -     RUNNING  source
                     0     FAILED   ERR no such key
```

The command exits non-zero when **any task** is not `RUNNING`, or when a
connector has no tasks at all.

:::caution[Watching only the connector state hides the stop]
A connector stays `RUNNING` while its only task is `FAILED` — that is
`checkout-outbox` above. Alerting on the connector state misses it; alerting per
task does not. That is why the task row is the main information in this table.
:::

### `connectors[].optional`

A connector whose source only exists after the application boots carries
`optional: true`:

```yaml
services:
  kafkaConnect:
    connectors:
      - name: checkout-outbox
        kind: custom
        optional: true
        config:
          connector.class: com.redis.kafka.connect.RedisStreamSourceConnector
```

With `optional: true`, a registration that does not become healthy during
`pyahu up` is reported as a **warning** in the summary instead of an error — `up`
stays green and you run `pyahu connectors apply` after the first boot. Without
`optional`, the failure fails `up`, which is the right behavior for a connector
whose source should already exist.

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

More context in [Local certificates](/docs/certificates).

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

---

## Out-of-date notice

When a newer release exists, the CLI prints a notice at the end of any command:

```text
⚠ pyahu 0.4.0 is out of date — 0.7.0 is available
  pyahu upgrade
  curl -fsSL https://cli.pyahu.io/install.sh | sh
  https://github.com/pyahu/cli/releases/tag/v0.7.0
  silence this with PYAHU_NO_UPDATE_CHECK=1
```

The suggested command follows **how you installed it** — the CLI reads its own binary path: under
the mise install tree it becomes `mise use github:pyahu/cli@<version>`, under `GOBIN`/`GOPATH` it
becomes `go install`, and otherwise the install script.

The notice goes to **stderr**, never stdout: `eval "$(pyahu env)"` and `--output json` consume
stdout, and a banner there would be evaluated as shell or break the JSON.

It stays silent when:

| Situation | Why |
| --- | --- |
| `--output json` or `--quiet` | the output is read by a script |
| Locally built binary (version `dev`) | there is no version to compare |
| `PYAHU_NO_UPDATE_CHECK` is set | explicit opt-out; it does not even reach the network |
| Offline, behind a proxy, or GitHub API rate limited | the check fails silently |

The lookup runs **alongside** the command and the answer is cached for 24h in
`<config dir>/pyahu/version-check.json`, so it costs no wall-clock time: the command never waits
more than 700ms for it, and the next day the answer is already on disk.

### `pyahu check-update`

Asks the releases endpoint directly — it does not go through the 24h cache behind the passive notice.

| Flag | Default | Description |
| --- | --- | --- |
| `--exit-code` | `false` | Exit 1 when a newer release is available (for scripts/CI). |

```bash
pyahu check-update
pyahu check-update --output json      # {"current":"0.4.0","latest":"0.7.0","outdated":true}
pyahu check-update --exit-code        # 0 = up to date, 1 = a newer release exists
```

It exits **0** by default even when behind: being out of date is not a command failure. Anyone who
wants to fail a pipeline over it asks for `--exit-code`, in the spirit of `git diff --exit-code`.

### `pyahu upgrade`

Downloads the release, **verifies its SHA-256** against the published `checksums.txt`, and swaps
this binary for the one inside the archive.

| Flag | Default | Description |
| --- | --- | --- |
| `--yes` | `false` | Replace without prompting. |
| `--version` | *(latest)* | Install a specific version — this is also how you roll back. |

```bash
pyahu upgrade
pyahu upgrade --yes
pyahu upgrade --yes --version v0.6.1   # downgrade
```

The new binary is written next to the current one and renamed over it: the swap is atomic on the
same filesystem, and an interrupted download never leaves a half-written binary. On Unix the running
process stays on the old inode, so `upgrade` itself finishes normally.

:::caution[A binary owned by another tool is left alone]
If this `pyahu` came from **mise** or `go install`, the command refuses and prints the right
instruction — replacing the file there would be undone by the next `mise install`:

```text
error: this pyahu is managed by another tool; upgrade it with: mise use github:pyahu/cli@0.7.0
```
:::

The other refusals, all before downloading anything: a locally built binary (version `dev`), Windows
(a running executable cannot be replaced under it), and an install directory that is not writable —
there the message already carries both ways out (`sudo`, or reinstall into `~/.local/bin`).

## Recommended flow

```bash
pyahu init --preset platform
pyahu doctor
pyahu up
pyahu certs trust
pyahu services
eval "$(pyahu env)"
```
