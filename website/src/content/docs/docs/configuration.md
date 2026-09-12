---
title: Configuration
description: Choose services, ports, resources and local credentials in pyahu.yaml.
---

The default project file is `pyahu.yaml`. The CLI searches for it from the
current directory upward. Use `--file` or `-f` to point to another path.

## Minimal example

```yaml
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: local-dev
services:
  postgres:
    enabled: true
    ports:
      primary: 5432
    databases:
      - name: app
```

## Complete stack

```yaml
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: platform
services:
  postgres:
    enabled: true
  zitadel:
    enabled: true
    externalURL: https://zitadel.localhost
  rabbitmq:
    enabled: true
  redis:
    enabled: true
  kafka:
    enabled: true
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-cdc
        type: source
        kind: debezium.postgres
        tables:
          include:
            - public.orders
  kafkaUI:
    enabled: true
```

## Ports

**TCP** services expose a host port within the service that owns the endpoint:

```yaml
services:
  postgres:
    ports:
      primary: 15432
  kafka:
    ports:
      bootstrap: 19092
  redis:
    ports:
      client: 16379
```

The **HTTP UIs** (ZITADEL, RabbitMQ, Kafka UI) do not use a host port: they go
through Traefik on 80/443 with `*.localhost` hostnames (`https://kafka-ui.localhost`,
`https://rabbitmq.localhost`, `https://zitadel.localhost`). To change the ZITADEL
domain, use `services.zitadel.externalURL`.

All host port mappings bind to the IPv4 loopback address (`127.0.0.1`) so the
local services are not published on LAN or other external interfaces.

## Apply or recreate?

`pyahu up` reconciles the Kubernetes resources generated from the current
file. It updates service configuration, images, Secrets, ConfigMaps, workloads,
topics and connector registrations without requiring a new cluster.

Some settings belong to k3d itself and are fixed at cluster creation:

- cluster name, server and agent counts;
- k3s image and Docker network;
- the persistent storage bind;
- host port mappings.

If one of these settings changes, `pyahu up` reports the exact drift. Recreate
the cluster to apply it:

```bash
pyahu down
pyahu up
```

Plain `pyahu down` retains the bound service data, so this is the normal path
for adding a service that needs a new host port. Removing services does not
silently remove their persistent volume claims.

## Kubernetes version

Pyahu defaults to `rancher/k3s:v1.36.4-k3s1`, matching the Kubernetes 1.36
`client-go` dependency used by the CLI. The real smoke workflow tests that
default and the previous supported minor, currently k3s 1.35.

You can set `cluster.k3sVersion` to another image tag when necessary, but
versions outside that tested window are best effort. The default and the tested
previous minor are updated as Kubernetes support moves forward.

## Resource requirements

The CLI and its generated control layer are lightweight, but the services are
the real databases, brokers, and identity server. Their current configured
resources are:

| Preset | Pod CPU requests | Pod memory requests | Pod memory limits | Persistent storage |
| --- | ---: | ---: | ---: | ---: |
| `minimal` | 50m | 128Mi | 512Mi | 2Gi |
| `platform` | 560m | 1,488Mi | 4,896Mi | 9Gi |

These totals exclude k3s, Traefik, container images, build cache, and transient
Jobs. Actual memory use is normally below the limits, but Docker must have room
for bursts and Kubernetes system components. For the complete `platform`
preset, start with at least 4 CPU cores, 8GiB of memory available to the
container runtime, and 15GiB of free disk. The `minimal` PostgreSQL preset is the
better starting point on constrained machines.

Pyahu does not currently enforce host CPU, memory, or disk minimums in
`doctor`; inspect the resources assigned to Docker Desktop, Colima, or your
other container runtime if pods remain pending or are OOM-killed.

Do not use `cluster.ports` in new files. The CLI accepts the legacy format for
compatibility, but service-owned port fields are the supported surface.

:::caution[Upgrading from an old `pyahu.yaml`]
`pyahu init` already generates the new format. If your file was created by an
earlier version, it may have `externalURL: https://zitadel.localhost:8443` with a
hardcoded port, and ZITADEL will insist on `:8443`, which is no longer mapped. Replace it
with `externalURL: https://zitadel.localhost` (without the port) and run `pyahu up`. The
`zitadel.ports`, `rabbitmq.ports.management`, and `kafkaUI.ports.http` fields are still
accepted, but they are ignored.
:::

## Redis

The `redis` service runs a single Valkey instance (Redis-compatible) with AOF
enabled by default:

```yaml
services:
  redis:
    enabled: true
    image: valkey/valkey     # use `redis` for the official Redis image
    version: 8.1-alpine
    ports:
      client: 6379
    auth:
      password: ""           # empty starts the server without --requirepass
    appendOnly: true
    storage: 1Gi
```

`appendOnly: true` is the default because an application that uses the `WAITAOF`
durability barrier fails **every write** when AOF is off — it does not merely
lose durability.

Inside the cluster, other workloads (Kafka Connect, for example) reach Redis at
`redis.<namespace>.svc.cluster.local:6379`. The host port is only for processes
on your machine.

Adding Redis to a cluster that is already running requires `pyahu down` and
`pyahu up`: the host port mapping is fixed when the cluster is created.

## Global config

You can define global defaults in the system configuration directory:

| System | Typical path |
| --- | --- |
| Linux | `~/.config/pyahu.yaml` |
| macOS | `~/Library/Application Support/pyahu.yaml` |

The global file is loaded first; the project's `pyahu.yaml` overrides the values.

## Local credentials

PostgreSQL, ZITADEL and RabbitMQ credentials can live in the project
`pyahu.yaml` or in the global config.
Credentials embedded in PostgreSQL, RabbitMQ, and Redis connection URLs are
percent-encoded, so passwords containing characters such as `@`, `:`, or `/`
remain valid. The separate password environment variables retain their original
unencoded values.

Command summaries mask credentials. Run `pyahu env` when an application needs
the real values.

For PostgreSQL, changing the password after the volume already exists updates
the Secret and CLI output, but the user inside the database may keep the old
password. Change the role password inside PostgreSQL or reset the stored data.

## Data lifecycle

`pyahu down` removes the k3d cluster and retains local data under
`~/.pyahu/clusters/<cluster>/storage`. `pyahu down --purge-data` permanently
removes that directory after confirmation.

For Kafka Connect, a successful `pyahu up` records the connector names managed
by the stack. A previously managed connector is deleted when its name is
removed from `pyahu.yaml`; connectors registered manually are left alone.
Kafka topics and service volumes are retained unless you explicitly purge the
stack data.
