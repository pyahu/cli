<p align="center">
  <img src="website/public/logo.svg" alt="Pyahu" width="92" height="92" />
</p>

<h1 align="center">Pyahu CLI</h1>

<p align="center">
  Run local application services on k3d from one project file.
</p>

<p align="center">
  <a href="https://github.com/pyahu/cli/actions/workflows/ci.yml"><img src="https://github.com/pyahu/cli/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/pyahu/cli/releases"><img src="https://img.shields.io/github/v/release/pyahu/cli?sort=semver" alt="Release" /></a>
  <a href="https://goreportcard.com/report/github.com/pyahu/cli"><img src="https://goreportcard.com/badge/github.com/pyahu/cli" alt="Go Report Card" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/pyahu/cli" alt="License" /></a>
</p>

<p align="center">
  <a href="https://cli.pyahu.io">Website</a> ·
  <a href="https://cli.pyahu.io/docs">Documentation</a> ·
  <a href="https://github.com/pyahu/cli/releases">Releases</a>
</p>

---

Pyahu creates a local k3d cluster and runs the services your application needs.
The setup lives in `pyahu.yaml`, so a team can keep the same ports, service
versions and defaults next to its code.

The normal workflow uses a few CLI commands. When something needs debugging,
the generated Kubernetes resources remain available through `kubectl`.

## Quick start

You need Docker or Podman running and [k3d](https://k3d.io) 5.x installed.

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh

mkdir my-app && cd my-app
pyahu init       # writes pyahu.yaml with PostgreSQL enabled
pyahu doctor     # checks the container runtime, k3d and local ports
pyahu up
eval "$(pyahu env)"
```

Your application can now use the generated `POSTGRES_URL`. To see the service
state and endpoint:

```console
$ pyahu services
cluster:   pyahu-local
namespace: pyahu-local-dev
state:     running

SERVICE   STATUS  VERSION  ENDPOINTS
postgres  ready   18.4     localhost:5432
```

## Why use it

- **One project file.** Services, ports and local defaults are reviewable in
  `pyahu.yaml`.
- **A repeatable stack.** `pyahu up` creates or reconciles the same resources
  for every developer.
- **Real Kubernetes behavior.** Ingresses, Secrets, ConfigMaps and persistent
  volumes run in k3d and can be inspected when needed.
- **More than a database.** Database, identity, messaging and CDC can run
  together without a separate setup for each service.

If all you need is a disposable database and Kubernetes behavior does not
matter, a single container may be simpler. Pyahu is most useful when a project
has several dependencies or benefits from a local Kubernetes environment.

## Choose a starting point

| Preset | What it starts | Good for |
| --- | --- | --- |
| `minimal` | PostgreSQL | Small projects and constrained machines |
| `platform` | All supported services | Applications that need the full dependency chain |

```bash
pyahu init --preset minimal    # default
pyahu init --preset platform
```

The full preset runs seven services. Allocate at least 4 CPU cores and 8 GiB of
memory to the container runtime, with 15 GiB of free disk. Start with `minimal`
if you are unsure.

## Included services

| Service | Use | Local endpoint |
| --- | --- | --- |
| PostgreSQL | Databases and optional read replicas | `localhost:5432` |
| ZITADEL | Identity and OIDC | `https://zitadel.localhost` |
| RabbitMQ | AMQP and management UI | `localhost:5672` · `https://rabbitmq.localhost` |
| Redis | Valkey-compatible data and streams | `localhost:6379` |
| Kafka | Local event streaming with KRaft | `localhost:9092` |
| Kafka Connect | Connectors, plugins and Debezium CDC | `http://localhost:8083` |
| Kafka UI | Topics, consumers and connectors | `https://kafka-ui.localhost` |

Services are enabled independently. A small stack can stay small:

```yaml
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: local-dev
services:
  postgres:
    enabled: true
    databases:
      - name: app
  redis:
    enabled: true
```

See [Configuration](https://cli.pyahu.io/docs/configuration) for all supported
fields and the changes that require recreating the cluster.

## Everyday commands

```bash
pyahu services                 # services, state and endpoints
pyahu describe postgres        # configuration and pod details, with secrets masked
pyahu logs postgres --follow   # service logs
eval "$(pyahu env)"            # connection variables for the current shell

export KUBECONFIG="$(pyahu kubeconfig)"
kubectl get pods -n pyahu-local-dev

pyahu down                     # remove the cluster and retain local data
pyahu down --purge-data --yes  # permanently remove retained data too
```

Host ports bind to `127.0.0.1`. Command summaries mask passwords and tokens;
`pyahu env` is the explicit way to print real connection values. Local data is
retained under `~/.pyahu/clusters/<cluster>/storage` unless `--purge-data` is
used.

## Kafka Connect and Debezium

Connectors can live in the same project file as the rest of the stack:

```yaml
services:
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-cdc
        kind: debezium.postgres
        database: app
        tables:
          include: [public.orders]
```

Pyahu applies the connector and checks each task, not only the top-level
connector state. It removes registrations that were previously managed by this
stack and are no longer declared, while leaving manually created connectors
alone. Custom plugin downloads require a SHA-256 value.

Read [Kafka Connect and Debezium](https://cli.pyahu.io/docs/kafka-connect-debezium)
for optional connectors, custom plugins and sink configuration.

## Install options

- [mise](https://cli.pyahu.io/docs/installation#mise) to pin the CLI version in a project
- the install script for macOS and Linux
- `go install github.com/pyahu/cli/cmd/pyahu@latest` with Go 1.26+
- archives for macOS, Linux and Windows from [GitHub Releases](https://github.com/pyahu/cli/releases)

The install script and `pyahu upgrade` verify the checksum published with the
release. Full instructions are in the [installation guide](https://cli.pyahu.io/docs/installation).

## Documentation

- [Getting started](https://cli.pyahu.io/docs)
- [Commands](https://cli.pyahu.io/docs/commands)
- [Configuration](https://cli.pyahu.io/docs/configuration)
- [Troubleshooting](https://cli.pyahu.io/docs/troubleshooting)
- [Local certificates](https://cli.pyahu.io/docs/certificates)
- [Backup and restore](https://cli.pyahu.io/docs/backup-restore)

## Scope

Pyahu focuses on local infrastructure. It does not deploy applications, manage
remote clusters or replace production infrastructure tooling.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the local
setup and development commands. Please follow the [Code of Conduct](CODE_OF_CONDUCT.md)
and report vulnerabilities through the [security policy](SECURITY.md).

## License

[MIT](LICENSE) © Pyahu
