# Redis Service, Kafka Connect Plugins, and Deferred Connectors

Status: draft
Date: 2026-09-07
Applies to: stack file `cli.pyahu.io/v1alpha1`, CLI v0.2.0

## Motivation

V1 provisions PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect (Debezium
image), and Kafka UI. Three gaps show up as soon as a real service stack is
declared:

1. **No Redis.** Services that keep hot state in Redis (or Valkey) have to run
   a container outside the cluster, and Kafka Connect inside the cluster
   cannot reach it without host networking tricks.
2. **Kafka Connect plugins are whatever the image ships.** The default
   `quay.io/debezium/connect` image carries only Debezium. A source or sink
   that needs another plugin (Redis Streams, JDBC, S3, a custom SMT) forces
   users to build and publish their own image. Some plugins additionally
   require the connector and its transforms to live in the **same plugin
   directory** (Kafka Connect isolates each plugin directory in its own
   classloader), so "just add a jar" is not enough.
3. **Connectors are registered too early.** `pyahu up` registers every
   connector and fails when a task is not `RUNNING` within two minutes
   (`internal/kube/client.go`, `WaitForStack`). Outbox connectors typically
   read a table, publication, or stream that only exists after the
   application has booted once — on a fresh cluster that is guaranteed to
   fail, and it fails the whole `up`.

This spec adds a `redis` service, a declarative `kafkaConnect.plugins` list,
per-connector `optional` registration, and `pyahu connectors apply|status`.
Nothing here changes the behavior of existing stack files.

## Non-goals

- Redis Sentinel or Cluster topologies. One instance, one PVC.
- Redis backup/restore through `pyahu backup`.
- Building or publishing a Kafka Connect image. Plugins are installed at pod
  start from declared artifacts.
- Deploying application workloads. The CLI stays infrastructure-only.

## 1. `services.redis`

```yaml
services:
  redis:
    enabled: true
    image: valkey/valkey
    version: 8.1-alpine
    ports:
      client: 6379
    auth:
      password: ""
    appendOnly: true
    storage: 1Gi
```

Defaults:

- `enabled`: `true` when `services.redis` is present
- `image`: `valkey/valkey` (Valkey is the upstream-compatible default; users
  can set `image: redis`)
- `version`: Pyahu default Valkey tag (confirm the tag exists on Docker Hub
  before pinning; fallback `redis` `7.4-alpine`). Both support `WAITAOF`,
  which requires Redis ≥ 7.2.
- `ports.client`: `6379`
- `auth.password`: empty (no `--requirepass`)
- `appendOnly`: `true` → container args `--appendonly yes --appendfsync everysec`.
  Applications that use the `WAITAOF` durability barrier fail every write
  when AOF is off, so AOF-on is the safe default for a local stack.
- `storage`: `1Gi`

Validation:

- `ports.client` in 1–65535 and not colliding with another enabled host port
- `storage` parseable as a Kubernetes quantity
- `image` and `version` non-empty

Kubernetes resources (namespace of the stack):

- Service `redis`, type NodePort, port 6379, stable NodePort `30379`
- StatefulSet `redis`, 1 replica, PVC `data` mounted at `/data`, TCP
  readiness/liveness probes on 6379 (image-agnostic — no dependency on
  `valkey-cli` vs `redis-cli`), requests `50m`/`64Mi`, limit `256Mi`
- k3d port mapping `<ports.client>:30379` on `server:0`. Adding Redis to an
  existing cluster requires `pyahu down` + `pyahu up`; the existing
  `missingDesiredPorts` check reports that.

Connection output (`pyahu env`):

- `REDIS_HOST` (`localhost`), `REDIS_PORT`, `REDIS_PASSWORD`
- `REDIS_URL` (`redis://[:password@]localhost:<port>`)

In-cluster DNS for other services (documented, not exported):
`redis.<namespace>.svc.cluster.local:6379`.

`pyahu services`, `pyahu describe redis`, and `pyahu logs redis` work like the
other services; `describe` shows `appendOnly` and the image under details.
`pyahu doctor` checks the client port. The `loader_test.go` case that rejects
`services.redis` becomes a case that accepts it.

## 2. `services.kafkaConnect.plugins`

```yaml
services:
  kafkaConnect:
    plugins:
      - name: redis-kafka-connect
        url: https://github.com/redis-field-engineering/redis-kafka-connect/releases/download/v1.1.0/redis-redis-kafka-connect-1.1.0.zip
        sha256: 7e4249ca356f702220cf09e9e150c8336e24824d7a72fce05d7999bf2a7e03df
      - name: redis-kafka-connect      # same name = same plugin directory
        file: connect-plugins/my-outbox-router.jar
```

Each entry is an artifact installed into plugin directory `<name>` before the
worker starts.

Fields:

- `name` (required): DNS label; the plugin directory. **Repeating a name is
  allowed and meaningful**: every artifact with that name lands in the same
  directory, which is how a transform gets access to a connector's classes.
- `url` + `sha256`: download and verify. `sha256` is **required** with `url`;
  an unverified download is not accepted.
- `file`: path relative to the stack file directory, must exist, ≤ 1 MiB. Meant
  for small jars (an SMT) that have no public URL yet. Stored in a Secret.
- Exactly one of `url` or `file`.

Installation:

- One `emptyDir` volume shared by an initContainer (`install-plugins`, image
  `alpine:3.21` — busybox provides `wget`, `sha256sum`, `unzip`, `tar`) and
  the worker.
- For each plugin: fetch or copy, verify the hash, then
  - `.zip` / `.tar.gz` / `.tgz`: extract to a temp dir and **flatten every
    `*.jar` found recursively** into `/plugins/<name>/` (release archives
    nest jars under `lib/`);
  - `.jar`: copy into `/plugins/<name>/`.
  Any failure exits non-zero naming the plugin.
- The worker mounts the volume at `/kafka/connect-extra` and gets
  `CONNECT_PLUGIN_PATH=/kafka/connect,/kafka/connect-extra` (the Debezium
  entrypoint maps `CONNECT_*` env to worker properties; `plugin.path` scans
  each listed directory's immediate children as plugins). If that mapping
  turns out not to be honored, the fallback is mounting the volume with a
  `subPath` per plugin under `/kafka/connect/<name>`.
- The Secret with `file` artifacts is named `kafka-connect-plugin-files` and
  mounted read-only into the initContainer.

Acceptance: `GET http://localhost:<rest>/connector-plugins` lists the
connector classes; `?connectorsOnly=false` also lists transforms.

Connector registration Job: before the `PUT /connectors/<name>/config`, wait
(up to 2 minutes) until `/connector-plugins?connectorsOnly=false` contains the
connector's `connector.class`, and fail with a message that names the missing
plugin. This separates "plugin not installed" from "connector unhealthy".

`pyahu describe kafka-connect` lists declared plugins and, when the cluster is
running, the classes the worker actually reports.

## 3. Optional connectors and `pyahu connectors`

```yaml
services:
  kafkaConnect:
    connectors:
      - name: orders-outbox
        kind: debezium.postgres
        optional: true
        ...
```

- `connectors[].optional` (default `false`). When `true`, a registration Job
  that does not reach `RUNNING` during `pyahu up` is reported as a **warning**
  in the `up` summary ("connector orders-outbox is not healthy yet — run
  `pyahu connectors apply` after the application has started") instead of
  failing the command. Non-optional connectors keep today's behavior.
- `connectors[].type` now defaults to `source` for `kind: custom` as well
  (today only `debezium.postgres` gets the default, while validation requires
  the field for every kind).

New command group:

- `pyahu connectors apply [--name <connector>]` — re-applies the connector
  Secret and Job for all (or one) connectors and waits for `RUNNING`.
  `applyJob` already deletes and recreates a Job that has not succeeded, so
  the command is idempotent and safe to run repeatedly.
- `pyahu connectors status [--format json]` — calls
  `GET /connectors?expand=status` through the host port and prints one row
  per connector with the connector state **and each task state**. Exit code is
  non-zero when any task is not `RUNNING` **or a connector has no tasks** — a
  connector can stay `RUNNING` while its only task is `FAILED`, and that is
  the silent-stop mode that alerting on the connector state misses.

## Implementation notes

- Files: `pkg/schema/stack.go` (types, defaults, validation, `ConnectionEnv`,
  `EnabledServices`, `enabledHostPorts`), `internal/kube/redis.go` (new),
  `internal/kube/apply.go` (`nodePortRedis`), `internal/kube/client.go`
  (`ApplyStack`, `WaitForStack`, optional-connector warnings),
  `internal/kube/kafka_connect.go` (plugins initContainer, Secret, env, Job
  wait), `internal/runtime/k3d/k3d.go` (port mapping),
  `internal/catalog/catalog.go`, `internal/doctor/doctor.go`,
  `internal/cli/connectors.go` (new), `internal/config/loader_test.go`.
- Tests follow the existing pattern: builders in
  `internal/kube/builders_test.go`, schema defaults/validation next to the
  schema, CLI golden output in `internal/cli/commands_test.go` with a fake
  Connect REST server for `status`.
- Docs: `docs/specs/stack-file-v1alpha1.md` (Redis section; `plugins` and
  `optional` under Kafka Connect), `examples/platform.yaml`, `README.md`,
  website pages `configuracao.md`, `comandos.md`,
  `kafka-connect-debezium.md` (pt and en), and `docs/specs/README.md`.
- Release: tag `v0.2.0` after the three sections land on `main`.

## Open questions

- Whether the Debezium entrypoint honors `CONNECT_PLUGIN_PATH` when
  `KAFKA_CONNECT_PLUGINS_DIR` is also set — verified during implementation
  with `/connector-plugins`; the `subPath` fallback is documented above.
- Valkey tag to pin as default. `8.1-alpine` is expected to exist; confirm
  with `docker manifest inspect` before committing the constant.
