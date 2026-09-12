---
title: Getting started
description: Create a local k3d stack and connect an application to its services.
---

Pyahu runs local application services in a k3d cluster. You choose the services
in `pyahu.yaml`; the CLI creates the cluster, waits for the workloads and prints
the endpoints your application can use.

Use Pyahu when a project needs several local dependencies or benefits from real
Kubernetes resources. If you only need a disposable database, a single
container may be simpler.

## 1. Choose a preset

| Preset | Services | Recommended for |
| --- | --- | --- |
| `minimal` | PostgreSQL | A first run, small projects and constrained machines |
| `platform` | PostgreSQL, ZITADEL, RabbitMQ, Redis, Kafka, Kafka Connect and Kafka UI | Applications that need the full dependency chain |

The full preset needs more room than the control CLI itself. Allocate at least
4 CPU cores and 8 GiB of memory to the container runtime, with 15 GiB of free
disk.

## 2. Install the CLI

You need Docker or Podman running and k3d 5.x on your `PATH`.

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh
```

For mise, Windows, manual downloads and shell completion, see
[Installation](/docs/installation).

## 3. Start PostgreSQL

Run these commands inside your project:

```bash
pyahu init       # creates pyahu.yaml with the minimal preset
pyahu doctor     # checks Docker/Podman, k3d and local ports
pyahu up
```

The CLI looks for `pyahu.yaml` from the current directory upward. Commands run
from a project subdirectory therefore use the same stack.

Check what is ready:

```console
$ pyahu services
cluster:   pyahu-local
namespace: pyahu-local-dev
state:     running

SERVICE   STATUS  VERSION  ENDPOINTS
postgres  ready   18.4     localhost:5432
```

## 4. Connect the application

Print the generated connection variables or load them into the current shell:

```bash
pyahu env
eval "$(pyahu env)"
```

The minimal preset provides `POSTGRES_URL`, `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_HOST`, `POSTGRES_PORT` and `POSTGRES_DATABASE`.
Use `pyahu env --format dotenv` to write values in dotenv form.

## Add services

Enable only what the application uses:

```yaml
services:
  postgres:
    enabled: true
  redis:
    enabled: true
  rabbitmq:
    enabled: true
```

Then run `pyahu up` again. If the new service needs a host port that was not
mapped when k3d was created, the CLI explains that the cluster must be
recreated:

```bash
pyahu down
pyahu up
```

Plain `pyahu down` keeps the service data on the host, so it is safe for this
recreation flow. See [Configuration](/docs/configuration) for ports,
credentials and complete examples.

## Inspect and troubleshoot

The common workflow does not require `kubectl` or Helm:

```bash
pyahu status
pyahu describe postgres
pyahu logs postgres --follow
```

The cluster remains inspectable when you need Kubernetes details:

```bash
export KUBECONFIG="$(pyahu kubeconfig)"
kubectl get pods,events -n pyahu-local-dev
```

Start with the [troubleshooting guide](/docs/troubleshooting) if a service does
not become ready.

## Local HTTPS

When an enabled service has an HTTP UI, Pyahu creates a local certificate for
`localhost` and `*.localhost`. Trust its CA once on the host:

```bash
pyahu certs trust
```

This covers ZITADEL, RabbitMQ Management and Kafka UI. See
[Local certificates](/docs/certificates) for status and rotation commands.

## Stop or reset the stack

```bash
pyahu down                     # remove the cluster, keep local data
pyahu down --keep-cluster      # remove only the stack namespace
pyahu down --purge-data --yes  # remove the cluster and permanently delete its data
```

Retained data lives under `~/.pyahu/clusters/<cluster>/storage`. Make a
[PostgreSQL backup](/docs/backup-restore) before a reset when the data matters.

## Next steps

- [Configure services and ports](/docs/configuration)
- [Use the command reference](/docs/commands)
- [Set up Kafka Connect and Debezium](/docs/kafka-connect-debezium)
- [Back up and restore PostgreSQL](/docs/backup-restore)
