---
title: Configuration
description: Basic structure of pyahu.yaml and important v1 rules.
---

The default file is `pyahu.yaml`. The CLI searches for this file from the current directory upward. Use `--file` or `-f` when you want to point to a different path.

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
```

The **HTTP UIs** (ZITADEL, RabbitMQ, Kafka UI) do not use a host port: they go
through Traefik on 80/443 with `*.localhost` hostnames (`https://kafka-ui.localhost`,
`https://rabbitmq.localhost`, `https://zitadel.localhost`). To change the ZITADEL
domain, use `services.zitadel.externalURL`.

Do not use `cluster.ports` in presets or new documentation. The CLI keeps silent compatibility with this legacy format, but it is not the v1 surface.

:::caution[Upgrading from an old `pyahu.yaml`]
`pyahu init` already generates the new format. If your file was created by an
earlier version, it may have `externalURL: https://zitadel.localhost:8443` with a
hardcoded port, and ZITADEL will insist on `:8443`, which is no longer mapped. Replace it
with `externalURL: https://zitadel.localhost` (without the port) and run `pyahu up`. The
`zitadel.ports`, `rabbitmq.ports.management`, and `kafkaUI.ports.http` fields are still
accepted, but they are ignored.
:::

## Global config

You can define global defaults in the system configuration directory:

| System | Typical path |
| --- | --- |
| Linux | `~/.config/pyahu.yaml` |
| macOS | `~/Library/Application Support/pyahu.yaml` |

The global file is loaded first; the project's `pyahu.yaml` overrides the values.

## Local credentials

PostgreSQL, Zitadel, and RabbitMQ credentials can live in the local `pyahu.yaml` or in the global config.

For PostgreSQL, changing the password after the volume already exists updates the Secrets and the CLI output, but the user inside the database may keep the old password. For local rotation, recreate the cluster or alter the role inside PostgreSQL.
