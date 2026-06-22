# Pyahu CLI V1: Local Cluster

Status: draft
Date: 2026-06-22

## Intent

Pyahu CLI is the developer entry point into the Pyahu platform. The first
release must deliver an excellent local infrastructure experience before adding
cloud and web-platform parity.

V1 provisions a local k3d cluster and installs only the base infrastructure
services:

- PostgreSQL
- ZITADEL
- RabbitMQ
- Kafka
- Kafka Connect with Debezium
- Kafka UI

There is no observability stack in v1. Secrets and ConfigMaps are plain
Kubernetes resources. Infisical, Glitch, Vault, and custom secret backends are
out of scope.

This phase does not manage application workloads, remote clusters, or
Telepresence-style remote development. Those capabilities can be added later
after the local infrastructure experience is excellent.

## Product Principles

1. A developer should understand what the CLI is doing within the first 100ms.
2. Long operations must stream useful progress, not raw debug noise.
3. Re-running `pyahu up` must be safe and idempotent.
4. Failures must explain the broken dependency, the current state, and the next
   command to run.
5. The local environment should use real services on Kubernetes, not mock
   containers hidden behind the CLI.
6. Defaults must work on a normal developer laptop, with explicit resource
   requirements for the full platform preset.
7. Human output is the default; JSON output is stable for scripts.
8. Generated cluster resources must be labeled so Pyahu can status, reconcile,
   and clean them.

## Non-Goals

- Observability, metrics, tracing, log aggregation, dashboards.
- Production cloud deployment.
- Application deployment and app lifecycle management.
- Remote cluster connections, mirroring, and Telepresence-style workflows.
- Pyahu account login or organization/project management.
- Secret managers beyond Kubernetes Secret.
- Multi-node HA by default.
- Supporting arbitrary Helm charts in the public CLI surface.
- Operator-backed service lifecycle. This can become an optional mode later, but
  v1 prioritizes fast local boot.

## UX Contract

The binary is `pyahu`.

Canonical v1 commands:

```bash
pyahu init
pyahu up
pyahu status
pyahu services
pyahu describe <service>
pyahu logs <service>
pyahu env
pyahu certs status
pyahu certs trust
pyahu certs rotate
pyahu backup postgres [database] --dir <dir>
pyahu restore postgres [database] --source <path-or-s3-uri>
pyahu kubeconfig
pyahu down
pyahu doctor
pyahu completion <shell>
```

Global flags:

```bash
--file, -f       path to project stack file; global config still applies
--output, -o     human|json; default human
--no-color       disable color even on a TTY
--quiet, -q      suppress non-essential output
--verbose, -v    include diagnostic details
--no-input       never prompt; fail with an actionable message instead
```

Command behavior:

- `pyahu init` writes a starter `pyahu.yaml`.
- `pyahu init --preset platform` writes all v1 infrastructure services.
- `pyahu up` creates or reconciles the k3d cluster and all enabled services.
- `pyahu status` shows cluster, service, endpoint, and readiness state.
- `pyahu services` lists enabled services, readiness, versions, and local
  endpoints.
- `pyahu describe <service>` shows detailed service metadata, endpoints,
  environment keys, configuration details, and pod state. Human output masks
  secret values unless `--show-secrets` is set.
- `pyahu logs <service>` streams service logs with `--follow` and `--tail`.
- `pyahu env` prints application connection settings as shell, dotenv, or JSON.
- `pyahu kubeconfig` prints the kubeconfig path or writes kubeconfig to stdout.
- `pyahu down` deletes the Pyahu-owned k3d cluster by default.
- `pyahu down --keep-cluster` removes stack resources but keeps the cluster.
- `pyahu doctor` checks Docker or Podman, k3d, ports, CPU, memory, disk, and
  Kubernetes API access. It also warns when other local Kubernetes clusters are
  present.

Exit codes:

- `0`: success
- `1`: generic failure
- `2`: usage or stack file validation failure
- `3`: missing or unhealthy local dependency
- `4`: cluster provisioning failure
- `5`: service provisioning failure
- `6`: readiness timeout
- `130`: interrupted by SIGINT or SIGTERM

## Default Local Experience

`pyahu init --preset platform && pyahu up` should produce:

- a k3d cluster named from `metadata.name`
- a Pyahu namespace for service workloads
- Kubernetes Secrets and ConfigMaps from the stack file
- a local CA plus Kubernetes TLS Secret for `localhost` and `*.localhost` (the
  wildcard covers `zitadel.localhost`, `kafka-ui.localhost`, ...)
- PostgreSQL reachable from the host
- ZITADEL reachable from the host at `https://zitadel.localhost`
- RabbitMQ AMQP reachable from the host; management UI at `https://rabbitmq.localhost`
- Kafka bootstrap reachable from the host
- Kafka Connect REST API reachable from the host
- Kafka UI reachable at `https://kafka-ui.localhost`
- a final connection summary and `pyahu env` hint

HTTP/HTTPS services share the Traefik entrypoints on host 80/443 and are routed
by `*.localhost` hostname; TCP services and the Kafka Connect REST API use
dedicated host ports.

Default host ports:

| Service | Stack field | Port |
| --- | --- | --- |
| Traefik web (HTTP UIs) | shared, when any HTTP service is enabled | 80 |
| Traefik websecure (HTTP UIs) | shared, when `localTLS.enabled` | 443 |
| PostgreSQL | `services.postgres.ports.primary` | 5432 |
| PostgreSQL read replicas | `services.postgres.ports.read` | 5433 |
| RabbitMQ AMQP | `services.rabbitmq.ports.amqp` | 5672 |
| Kafka bootstrap | `services.kafka.ports.bootstrap` | 9092 |
| Kafka Connect REST API | `services.kafkaConnect.ports.rest` | 8083 |

HTTP UIs are reached by hostname through Traefik, not by a dedicated host port:
ZITADEL `zitadel.localhost`, RabbitMQ management `rabbitmq.localhost`, Kafka UI
`kafka-ui.localhost`.

If a default port is busy, `pyahu doctor` and `pyahu up` must fail before
provisioning and show the exact stack-file override.

`pyahu up` must generate local TLS material idempotently when `localTLS.enabled`
is true. The CA lives under the OS user config directory
(`~/.config/pyahu/certs` on Linux and
`~/Library/Application Support/pyahu/certs` on macOS), the project wildcard
certificate lives under `.pyahu/local/certs`, and the cluster receives a
Kubernetes TLS Secret plus a CA ConfigMap. The host trust store is changed only
by the explicit `pyahu certs trust` command. If the host does not trust the CA,
human output should tell the user to run that command before using
`https://*.localhost` without browser or client warnings.

If other local Kubernetes clusters are present, for example k3d or Kind
clusters, `pyahu doctor` and `pyahu up` must warn but should not fail only for
that reason. V1 recommends one local cluster by default to avoid port,
kubeconfig context, and resource contention issues.

## Feedback Model

The CLI should use a typed event stream internally:

```text
preflight.started
preflight.completed
cluster.creating
cluster.ready
service.installing
service.waiting
service.ready
service.failed
summary.completed
```

The human renderer turns events into compact progress lines and keeps long waits
alive with useful state such as the pod phase, Kubernetes condition, image pull
status, or last readiness message.

JSON output for long-running commands is newline-delimited JSON events. JSON
output for state commands such as `status`, `doctor`, `env`, and `kubeconfig` is
a single JSON document.

Color and animation are enabled only for interactive terminals and must respect
`NO_COLOR`, `TERM=dumb`, and `--no-color`.

## Runtime and Dependencies

Runtime dependencies for users:

- Docker or Podman with Docker-compatible API
- k3d

k3d port mappings are fixed at cluster creation. If the stack changes in a way
that requires a host port mapping not present in the saved generated k3d config,
`pyahu up` must stop and instruct the user to recreate the cluster with
`pyahu down` followed by `pyahu up`.

Optional dependencies:

- AWS CLI for `pyahu restore postgres --source s3://...`

The CLI should not require `kubectl` or `helm` for normal operation. It may
print optional debugging commands that use them.

Implementation dependencies:

- Cobra for CLI command structure, help, flags, suggestions, and shell
  completion.
- Go `context` and `os/signal.NotifyContext` for cancellation.
- Go `log/slog` for structured diagnostic logs.
- Kubernetes client-go or controller-runtime client for Kubernetes API access.
- YAML v3 for stack parsing, with strict unknown-field validation.

The Go toolchain target should move to a supported baseline. As of 2026-06-21,
Go 1.26.4 is the current stable download and Go supports the two most recent
major releases. Use a `go 1.25.0` module directive unless a dependency requires
newer features, and avoid Go 1.26-only syntax in v1.

## Backup and Restore

V1 backup/restore is command-driven and should not add stack-file YAML. The
first supported service is PostgreSQL.

`pyahu backup postgres [database] --dir <dir>`:

- Uses the configured PostgreSQL primary pod in the local cluster.
- Runs `pg_dump --format=custom --no-owner --no-acl` inside the pod.
- Streams the dump directly to a file on the host.
- Defaults to the first configured `services.postgres.databases` entry when the
  database argument is omitted.
- Writes to `.pyahu/backups` under the project when `--dir` is omitted.

`pyahu restore postgres [database] --source <path-or-s3-uri>`:

- Restores a PostgreSQL custom-format dump with `pg_restore` inside the primary
  pod.
- Accepts a host file path.
- Accepts `s3://...` only by shelling out to `aws s3 cp` into a temporary host
  file; there is no S3 configuration in YAML and no required AWS SDK
  dependency.
- Supports `--s3-endpoint-url` for S3-compatible providers.
- Defaults to `--clean=true`, adding `--clean --if-exists` to `pg_restore`.
- Requires interactive confirmation when `--clean=true`; scripts and
  `--no-input` runs must pass `--yes`.

## Service Choices

PostgreSQL:

- Use a single-primary PostgreSQL StatefulSet for v1 local.
- Optionally create a separate `postgres-read` StatefulSet for read replicas.
- Create one PostgreSQL service per stack in the Pyahu namespace.
- Use stack-configured local credentials, stored as Kubernetes Secrets.
- Create configured databases during first boot.
- Expose the primary locally through `services.postgres.ports.primary`.
- Expose read replicas locally through `services.postgres.ports.read` when
  `services.postgres.readReplicas > 0`.
- Readiness requires the pod to be Ready and `pg_isready` to pass.

ZITADEL:

- Use the official ZITADEL container directly for v1 local.
- Use the v1 PostgreSQL service rather than the chart's bundled PostgreSQL
  subchart.
- Use stack-configured admin credentials and master key.
- Use local HTTPS through k3s Traefik by default.
- Terminate TLS at Traefik with the generated `pyahu-local-tls` Secret.
- Keep ZITADEL itself on h2c internally; do not add cert-manager, Envoy, ACME,
  or public CA issuance for v1 local TLS.
- Readiness requires deployment availability and an HTTP health check.

RabbitMQ:

- Use a single-node RabbitMQ management StatefulSet for v1 local.
- Expose AMQP and management UI locally.
- Use stack-configured users, vhosts, and permissions.
- Import RabbitMQ definitions on boot from a generated Kubernetes Secret.
- Readiness requires the pod to be Ready and AMQP port reachable.

Kafka:

- Use a single-node Apache Kafka KRaft StatefulSet for v1 local.
- Configure an internal listener for pods and an external listener for host
  clients.
- Create configured topics after the broker is ready.
- Readiness requires the pod to be Ready and bootstrap endpoint reachability.

Kafka Connect / Debezium:

- Use the official Debezium Connect container for v1 local.
- Expose the Kafka Connect REST API locally through
  `services.kafkaConnect.ports.rest`.
- Configure Kafka Connect against the internal Kafka service.
- Use compact internal topics for Kafka Connect configs, offsets, and status.
- Define Kafka Connect connectors declaratively under
  `services.kafkaConnect.connectors`.
- Support `type: source` Debezium PostgreSQL connectors through
  `kind: debezium.postgres`.
- Support custom `type: source` and `type: sink` connectors through
  `kind: custom` plus `config.connector.class`; plugin availability is the
  responsibility of the configured Kafka Connect image.
- Use the local PostgreSQL user from `services.postgres.auth` for connector
  database access. In the local official PostgreSQL container this user has full
  rights.
- Apply connector definitions through the Kafka Connect REST API after the
  service is reachable.
- Readiness requires the Kafka Connect pod to be Ready and the REST port to
  respond.

Kafka UI:

- Use the official Kafbat Kafka UI container for v1 local.
- Expose the UI locally through `services.kafkaUI.ports.http`.
- Configure Kafka UI against the internal Kafka service.
- Configure Kafka Connect integration when Kafka Connect is enabled.
- Readiness requires the Kafka UI pod to be Ready and the HTTP port to respond.

## Acceptance Criteria

V1 is acceptable when:

- `pyahu init --preset platform` creates a valid stack file.
- `pyahu up` creates a new k3d cluster and provisions all enabled services.
- `pyahu up` can be run again without breaking existing services.
- `pyahu status --output json` returns machine-readable service states.
- `pyahu services --output json` returns machine-readable service endpoint
  metadata.
- `pyahu describe postgres --output json` returns machine-readable service
  details.
- `pyahu env --format dotenv` returns usable connection settings.
- `pyahu logs postgres --tail 50` returns logs without requiring kubectl.
- `pyahu describe kafka-connect` shows the Kafka Connect endpoint, image,
  internal topics, and declared connectors.
- `pyahu describe kafka-ui` shows the Kafka UI endpoint, image, Kafka
  bootstrap target, and Kafka Connect integration when enabled.
- `pyahu down` deletes the Pyahu-owned k3d cluster.
- Ctrl-C exits promptly and a later `pyahu up` can resume reconciliation.
- `pyahu doctor` catches missing Docker/k3d and busy default ports before
  provisioning.
- `pyahu doctor` reports other local k3d or Kind clusters as warnings.
- The warm path, with images already cached, completes under 6 minutes on a
  developer machine with 8 CPU cores and 16 GB RAM allocated to containers.
- The second idempotent `pyahu up` completes under 60 seconds when no changes
  are needed.

## References Checked

- Go release downloads and release policy: https://go.dev/dl/ and
  https://go.dev/doc/devel/release
- Cobra CLI framework: https://github.com/spf13/cobra
- CLI design guidelines: https://clig.dev/
- k3d config files: https://k3d.io/v5.4.1/usage/configfile/
- Kubernetes client-go: https://pkg.go.dev/k8s.io/client-go
- ZITADEL Kubernetes docs: https://zitadel.com/docs/self-hosting/deploy/kubernetes
- ZITADEL Docker/local docs: https://zitadel.com/docs/self-hosting/deploy/compose
- RabbitMQ container docs: https://hub.docker.com/_/rabbitmq
- RabbitMQ definitions import docs: https://www.rabbitmq.com/docs/definitions
- RabbitMQ password hashing docs: https://www.rabbitmq.com/docs/passwords
- Apache Kafka container docs: https://hub.docker.com/r/apache/kafka
- Kafbat Kafka UI repository and container docs: https://github.com/kafbat/kafka-ui
