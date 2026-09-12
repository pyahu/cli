# Pyahu Stack File V1Alpha1

Status: draft
Date: 2026-06-22

## Decision

V1 uses YAML.

Reasons:

- k3d config, Kubernetes resources, Helm values, and the existing repo examples
  are YAML-based.
- The v1 shape contains nested service settings where YAML is easier to scan
  than TOML.
- A future TOML frontend can be added later if there is a strong user need.

The primary default stack file is `pyahu.yaml`. Users can override it with
`--file` or `-f`.

Global user defaults are optional and live in the OS user config directory:
`$XDG_CONFIG_HOME/pyahu.yaml` or `~/.config/pyahu.yaml` on Linux, and
`~/Library/Application Support/pyahu.yaml` on macOS. The CLI loads global
defaults first, then project config. Project values override global values.
Mapping nodes merge recursively; scalar values and sequences are replaced by
the project value.

Default discovery order:

1. Global `$XDG_CONFIG_HOME/pyahu.yaml`, when present
2. Path passed via `--file`, or project discovery:
3. `pyahu.yaml`
4. `pyahu.yml`
5. `.pyahu/stack.yaml`
6. `.pyahu/stack.yml`

The CLI should keep generated state under `.pyahu/local/` and should never write
runtime state back into the project stack file.

## Minimal Stack

```yaml
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: demo

services:
  postgres:
    enabled: true
    databases:
      - name: app
```

## Platform Preset

`pyahu init --preset platform` should generate:

```yaml
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: pyahu-local

cluster:
  runtime: k3d
  name: pyahu-local
  namespace: pyahu-local-dev
  servers: 1
  agents: 0

localTLS:
  enabled: true
  domains:
    - localhost
    - "*.localhost"
    - zitadel.localhost

services:
  postgres:
    enabled: true
    version: "18.4"
    ports:
      primary: 5432
      read: 5433
    auth:
      username: pyahu
      password: pyahu_local
    instances: 1
    readReplicas: 0
    storage: 2Gi
    databases:
      - name: app
        owner: pyahu
      - name: zitadel
        owner: zitadel

  zitadel:
    enabled: true
    version: v4.15.2
    externalURL: https://zitadel.localhost
    databaseRef: postgres
    masterKey: MasterkeyNeedsToHave32Characters
    admin:
      username: admin@pyahu.local
      password: Password1!

  rabbitmq:
    enabled: true
    version: 4.3.2-management-alpine
    ports:
      amqp: 5672
    auth:
      username: pyahu
      password: pyahu_local
    replicas: 1
    storage: 2Gi
    management: true
    vhosts:
      - name: /
    users:
      - name: pyahu
        password: pyahu_local
        tags: administrator
        permissions:
          - vhost: /
            configure: .*
            write: .*
            read: .*

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

  kafka:
    enabled: true
    version: "4.3.0"
    ports:
      bootstrap: 9092
    replicas: 1
    storage: 4Gi
    topics:
      - name: app.events
        partitions: 1
        replicas: 1

  kafkaConnect:
    enabled: true
    image: quay.io/debezium/connect
    version: 3.5.2.Final
    ports:
      rest: 8083
    replicas: 1
    connectors: []

  kafkaUI:
    enabled: true
    image: ghcr.io/kafbat/kafka-ui
    version: v1.5.0
    replicas: 1
```

> HTTP UIs (ZITADEL, RabbitMQ management, Kafka UI) are exposed through Traefik
> on host 80/443 by `*.localhost` hostname, not by a per-service host port. The
> `zitadel.ports`, `rabbitmq.ports.management` and `kafkaUI.ports.http` fields are
> still accepted for backward compatibility but are ignored.

## Top-Level Fields

```yaml
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata: {}
cluster: {}
localTLS: {}
services: {}
configMaps: {}
secrets: {}
```

Required fields:

- `apiVersion`
- `kind`
- `metadata.name`
- at least one enabled service

Validation rules:

- Unknown fields are errors.
- Names must be DNS-label compatible unless otherwise documented.
- Service keys must be one of `postgres`, `zitadel`, `rabbitmq`, `redis`,
  `kafka`, `kafkaConnect`, or `kafkaUI`.
- Local host ports must be unique.
- Secret values cannot be read directly from CLI flags.
- Relative paths are resolved from the stack file directory.

## Cluster

```yaml
cluster:
  runtime: k3d
  name: demo
  k3sVersion: rancher/k3s:v1.36.4-k3s1
  servers: 1
  agents: 0
```

Defaults:

- `runtime`: `k3d`
- `name`: `metadata.name`
- `k3sVersion`: `rancher/k3s:v1.36.4-k3s1`
- `servers`: `1`
- `agents`: `0`

The default k3s minor matches the CLI's `client-go` minor. CI exercises the
default and the previous Kubernetes minor; explicitly configured versions
outside that window are best effort.

Host ports are configured under each service. Early prerelease `cluster.ports`
files are still accepted for migration, but new stack files should not use that
shape.

Pyahu generates a k3d config file under `.pyahu/local/k3d.yaml` from this
section. The generated file is an implementation detail and may contain
additional labels, host aliases, and port mappings required by the services.

## Local TLS

```yaml
localTLS:
  enabled: true
  domains:
    - localhost
    - "*.localhost"
    - zitadel.localhost
  secretName: pyahu-local-tls
  caConfigMapName: pyahu-local-ca
```

Defaults:

- `enabled`: `true`
- `domains`: `localhost` and `*.localhost`; these are always included when local
  TLS is enabled. `*.localhost` covers every `.localhost` subdomain
  (`zitadel.localhost`, `kafka-ui.localhost`, ...)
- `secretName`: `pyahu-local-tls`
- `caConfigMapName`: `pyahu-local-ca`

Pyahu creates a local development CA under the OS user config directory
(`~/.config/pyahu/certs` on Linux and
`~/Library/Application Support/pyahu/certs` on macOS) and a project wildcard
certificate under `.pyahu/local/certs`. During `pyahu up`, the leaf certificate
and key are stored in the Kubernetes TLS Secret named by `secretName`, and the
CA certificate is published in the ConfigMap named by `caConfigMapName`.

The local CA is not public and is not installed into the host trust store unless
the user explicitly runs `pyahu certs trust`. `pyahu certs status` reports
whether the CA and certificate exist and whether the host currently trusts the
CA. `pyahu certs rotate` regenerates the local CA and wildcard certificate; run
`pyahu certs trust` and `pyahu up` after rotation.

V1 keeps the default k3s Traefik ingress controller. It does not add Envoy,
cert-manager, ACME, or public CA issuance for `.localhost`.

## Services

A service is disabled when its key is absent or when `enabled: false` is set.
When a service key is present, `enabled` defaults to `true`.

### PostgreSQL

```yaml
services:
  postgres:
    enabled: true
    version: "18.4"
    ports:
      primary: 5432
      read: 5433
    auth:
      username: pyahu
      password: pyahu_local
    instances: 1
    readReplicas: 1
    replication:
      username: pyahu_replicator
      password: pyahu_replicator_local
    storage: 2Gi
    databases:
      - name: app
        owner: pyahu
        seed: ./seeds/app.sql
```

Defaults:

- `enabled`: `true` when `services.postgres` is present
- `version`: Pyahu default PostgreSQL stable version
- `ports.primary`: `5432`
- `ports.read`: `5433` when read replicas are enabled
- `auth.username`: `pyahu`
- `auth.password`: `pyahu_local`
- `instances`: `1`; v1 supports a single primary only
- `readReplicas`: `0`
- `replication.username`: `pyahu_replicator`
- `replication.password`: `pyahu_replicator_local`
- `storage`: `2Gi`
- `owner`: `auth.username` unless explicitly set

When `readReplicas` is greater than zero, Pyahu creates a separate
`postgres-read` StatefulSet and Service. The `postgres` Service continues to
point only to the primary. `postgres-read` exposes read-only hot standby
replicas locally through `services.postgres.ports.read`, default `5433`.

`databases[].seed` runs through the official PostgreSQL container init flow and
therefore only executes when the database volume is initialized for the first
time.

Connection output:

- `POSTGRES_HOST`
- `POSTGRES_PORT`
- `POSTGRES_DATABASE`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `POSTGRES_URL`
- `POSTGRES_READ_HOST` when `readReplicas > 0`
- `POSTGRES_READ_PORT` when `readReplicas > 0`
- `POSTGRES_READ_URL` when `readReplicas > 0`

### ZITADEL

```yaml
services:
  zitadel:
    enabled: true
    externalURL: https://zitadel.localhost
    databaseRef: postgres
    masterKey: MasterkeyNeedsToHave32Characters
    admin:
      username: admin@pyahu.local
      password: Password1!
```

Defaults:

- `enabled`: `true` when `services.zitadel` is present
- `databaseRef`: `postgres`
- `externalURL`: `https://zitadel.localhost` when `localTLS.enabled` is true;
  otherwise `http://zitadel.localhost`. ZITADEL is served through Traefik on host
  80/443, so the URL is portless. A custom `externalURL` must use the Traefik
  ports (none/`443` for https, none/`80` for http) to stay reachable.
- `admin.username`: `admin@pyahu.local`
- `admin.password`: `Password1!`
- `masterKey`: local default 32-character key
- `ports.http` / `ports.https`: accepted for backward compatibility but ignored;
  ZITADEL uses the shared Traefik entrypoints.

Admin credentials and master key are stored in Kubernetes Secret and exposed
through `pyahu env`. They can be provided in the stack file or inherited from
global user defaults.

Connection output:

- `ZITADEL_ISSUER`
- `ZITADEL_CONSOLE_URL`
- `ZITADEL_ADMIN_USER`
- `ZITADEL_ADMIN_PASSWORD`

### RabbitMQ

```yaml
services:
  rabbitmq:
    enabled: true
    version: 4.3.2-management-alpine
    ports:
      amqp: 5672
    auth:
      username: pyahu
      password: pyahu_local
    replicas: 1
    storage: 2Gi
    management: true
    vhosts:
      - name: /
    users:
      - name: pyahu
        password: pyahu_local
        tags: administrator
        permissions:
          - vhost: /
            configure: .*
            write: .*
            read: .*
```

Defaults:

- `enabled`: `true` when `services.rabbitmq` is present
- `version`: Pyahu default RabbitMQ management image tag
- `ports.amqp`: `5672`
- `auth.username`: `pyahu`
- `auth.password`: `pyahu_local`
- `replicas`: `1`; this version only supports a single RabbitMQ node because
  clustering is not configured yet
- `storage`: `2Gi`
- `management`: `true` — the UI is served at `https://rabbitmq.localhost` through
  Traefik; `ports.management` is accepted for compatibility but ignored.
- `vhosts`: `/`
- `users`: one administrator user matching `auth.username`

RabbitMQ user passwords are rendered into the generated definitions file as
`password_hash`, not plaintext. If `users` is provided and does not include the
main `auth.username`, Pyahu adds that user automatically so `pyahu env` remains
usable.

Connection output:

- `RABBITMQ_HOST`
- `RABBITMQ_PORT`
- `RABBITMQ_MANAGEMENT_URL`
- `RABBITMQ_USER`
- `RABBITMQ_PASSWORD`
- `RABBITMQ_URL`

### Redis

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
- `image`: `valkey/valkey` (Valkey is the upstream-compatible default; set
  `image: redis` for the Redis image)
- `version`: `8.1-alpine`
- `ports.client`: `6379`
- `auth.password`: empty, which starts the server without `--requirepass`
- `appendOnly`: `true` → container args `--appendonly yes --appendfsync everysec`.
  Clients that use the `WAITAOF` durability barrier fail **every write** when AOF
  is off, so AOF-on is the safe default for a local stack.
- `storage`: `1Gi`

The server runs as a single-replica StatefulSet with the PVC mounted at `/data`
and TCP readiness/liveness probes on 6379 (the probe is image-agnostic: it does
not depend on `valkey-cli` vs `redis-cli`). When a password is set it is read
from the local credentials Secret, not written into the pod spec.

Adding Redis to a cluster that is already running requires `pyahu down` and
`pyahu up`, because the host port mapping is fixed when the cluster is created.

In-cluster DNS for other workloads: `redis.<namespace>.svc.cluster.local:6379`.
That is the address Kafka Connect and other in-cluster clients use; the host port
is only for processes on the developer machine.

Connection output:

- `REDIS_HOST`
- `REDIS_PORT`
- `REDIS_PASSWORD`
- `REDIS_URL` (`redis://[:password@]localhost:<port>`)

### Kafka

```yaml
services:
  kafka:
    enabled: true
    version: "4.3.0"
    ports:
      bootstrap: 9092
    replicas: 1
    storage: 4Gi
    topics:
      - name: app.events
        partitions: 1
        replicas: 1
```

Defaults:

- `enabled`: `true` when `services.kafka` is present
- `ports.bootstrap`: `9092`
- `replicas`: `1`; this version only supports a single KRaft broker
- `storage`: `4Gi`
- topic `partitions`: `1`
- topic `replicas`: `1` and no greater than the number of Kafka brokers

Connection output:

- `KAFKA_BOOTSTRAP_SERVERS`

### Kafka Connect / Debezium

```yaml
services:
  kafkaConnect:
    enabled: true
    image: quay.io/debezium/connect
    version: 3.5.2.Final
    ports:
      rest: 8083
    replicas: 1
    plugins:
      - name: redis-kafka-connect
        url: https://github.com/redis-field-engineering/redis-kafka-connect/releases/download/v1.1.0/redis-redis-kafka-connect-1.1.0.zip
        sha256: 7e4249ca356f702220cf09e9e150c8336e24824d7a72fce05d7999bf2a7e03df
      - name: redis-kafka-connect      # same name = same plugin directory
        file: connect-plugins/my-outbox-router.jar
    connectors:
      - name: app-cdc
        type: source
        kind: debezium.postgres
        database: app
        topicPrefix: app-cdc
        slot: app_cdc_slot
        publication: app_cdc_publication
        snapshotMode: initial
        tables:
          include:
            - public.orders
        config:
          decimal.handling.mode: string
      - name: app-sink
        type: sink
        kind: custom
        config:
          connector.class: io.confluent.connect.jdbc.JdbcSinkConnector
          tasks.max: "1"
          topics: app-cdc.public.orders
```

Defaults:

- `enabled`: `true` when `services.kafkaConnect` is present
- `image`: `quay.io/debezium/connect`
- `version`: Pyahu default Debezium Connect stable version
- `ports.rest`: `8083`
- `replicas`: `1`
- connector `type`: `source`, for every kind; an explicit value must be
  `source` or `sink`
- connector `optional`: `false`
- connector `kind`: `custom` when `config.connector.class` is set, otherwise
  `debezium.postgres`
- connector `database`: first configured PostgreSQL database
- connector `topicPrefix`: connector `name`
- connector `slot`: connector `name` with dashes converted to underscores plus
  `_slot`
- connector `publication`: connector `name` with dashes converted to
  underscores plus `_publication`
- connector `snapshotMode`: `initial`

`plugins[]` installs artifacts into the worker's plugin directories before it
starts, so a connector that does not ship with the image does not require
building one:

- `name` (required): DNS label, and the plugin directory. **Repeating a name is
  allowed and meaningful** — Kafka Connect isolates each plugin directory in its
  own classloader, so a transform that needs a connector's classes has to land in
  the same directory as the connector.
- `url` + `sha256`: download and verify. `sha256` is required with `url`.
- `file`: path relative to the stack file directory, at most 1 MiB, carried in a
  Secret. Meant for a small jar that has no public URL yet.
- Exactly one of `url` or `file`. The artifact must be `.zip`, `.tar.gz`, `.tgz`,
  or `.jar`.

An `install-plugins` initContainer fetches or copies each artifact, verifies the
hash, extracts archives, and flattens every `*.jar` it finds into
`/plugins/<name>/`. The volume is mounted at `/kafka/connect-extra` on the worker,
which runs with `CONNECT_PLUGIN_PATH=/kafka/connect,/kafka/connect-extra`.

Before registering a connector, Pyahu waits for its `connector.class` to appear in
`GET /connector-plugins?connectorsOnly=false`, so a missing plugin is reported as
a missing plugin rather than as an unhealthy connector.

`connectors[].optional: true` marks a connector whose source — a table, a
publication, a stream — only exists after the application has booted once. A
registration that does not become healthy during `pyahu up` is reported as a
warning in the summary instead of failing the command; `pyahu connectors apply`
registers it afterwards. Non-optional connectors keep failing `up`.

`pyahu connectors status` reports the connector state **and each task state**, and
exits non-zero when any task is not `RUNNING` or a connector has no tasks: a
connector stays `RUNNING` while its only task is `FAILED`, and that silent stop is
what connector-level checks miss.

Kafka Connect requires Kafka. Debezium PostgreSQL connectors also require
PostgreSQL. V1 supports a declarative PostgreSQL Debezium source shortcut and
custom Kafka Connect source or sink connectors. Custom connectors use
`kind: custom`, require `connectors[].config.connector.class`, and pass
`connectors[].config` directly to the Kafka Connect REST API. Sink plugins must
already exist in the configured Kafka Connect image.

Generated Debezium PostgreSQL connector configuration uses the local
`services.postgres.auth` user and password. In the official PostgreSQL
container this user is the local superuser, which gives Debezium permission to
create publications, use logical replication slots, and read all configured
tables during local development.

Pyahu configures PostgreSQL with logical replication enabled when PostgreSQL is
enabled:

- `wal_level=logical`
- `max_wal_senders=10`
- `max_replication_slots=10`

Connection output:

- `KAFKA_CONNECT_URL`

### Kafka UI

```yaml
services:
  kafkaUI:
    enabled: true
    image: ghcr.io/kafbat/kafka-ui
    version: v1.5.0
    replicas: 1
```

Defaults:

- `enabled`: `true` when `services.kafkaUI` is present
- `image`: `ghcr.io/kafbat/kafka-ui`
- `version`: Pyahu default Kafka UI stable version
- `replicas`: `1`

Kafka UI is served at `https://kafka-ui.localhost` through Traefik; `ports.http`
is accepted for compatibility but ignored.

Kafka UI requires Kafka. When Kafka Connect is enabled, Pyahu configures the UI
with the internal Kafka Connect REST endpoint.

Connection output:

- `KAFKA_UI_URL`

## ConfigMaps

```yaml
configMaps:
  app:
    data:
      LOG_LEVEL: debug
    files:
      app.conf: ./config/app.conf
```

Rules:

- ConfigMaps are created in the Pyahu stack namespace.
- `data` values are strings.
- `files` keys become ConfigMap keys and values are loaded from local files.

## Secrets

```yaml
secrets:
  app:
    stringData:
      API_TOKEN:
        fromEnv: API_TOKEN
      TLS_KEY:
        fromFile: ./secrets/tls.key
```

Rules:

- Secrets are created in the Pyahu stack namespace.
- `fromEnv` reads from the process environment.
- `fromFile` reads from a local file relative to the stack file.
- `literal` may be added later, but is intentionally excluded from the v1
  starter examples to discourage committing secrets.
- Missing secret inputs are validation errors before provisioning starts.
