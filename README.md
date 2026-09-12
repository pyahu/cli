<p align="center">
  <img src="website/public/logo.svg" alt="Pyahu" width="92" height="92" />
</p>

<h1 align="center">Pyahu CLI</h1>

<p align="center">
  Your local development stack in a single command.
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

Pyahu CLI provisions a local development stack on a [k3d](https://k3d.io) cluster
with lightweight Kubernetes manifests the CLI generates for you. One command
brings up PostgreSQL, ZITADEL, RabbitMQ, Redis, Kafka, Kafka Connect with Debezium,
and Kafka UI, with local TLS and predictable endpoints, and without turning your
setup into a side project.

Its control layer is intentionally lightweight: k3d plus generated resources,
without Helm releases or extra in-cluster operators. The complete platform
preset still runs seven real services and therefore needs meaningful Docker
resources; see the [resource guidance](https://cli.pyahu.io/docs/configuration#resource-requirements).
Normal operation does not require `kubectl` or `helm`.

```console
$ pyahu init --preset platform
$ pyahu up
✓ Checking local dependencies  (137ms)
✓ Cluster pyahu-local created  (8.4s)
✓ Waiting for the Kubernetes API  (3.1s)
✓ Configuring services: postgres, zitadel, rabbitmq, redis, kafka, kafka-connect, kafka-ui

✓ Pyahu local stack is ready
```

## What it provisions

| Service | Role | Endpoint |
| --- | --- | --- |
| PostgreSQL | Relational database (optional read replicas) | `localhost:5432` |
| ZITADEL | Identity & OIDC over local HTTPS | `https://zitadel.localhost` |
| RabbitMQ | AMQP messaging + management UI | `localhost:5672` · `https://rabbitmq.localhost` |
| Redis | Valkey key-value store, AOF on by default | `localhost:6379` |
| Kafka | Event streaming broker (KRaft) | `localhost:9092` |
| Kafka Connect | Declarative connectors and plugins, Debezium CDC | `http://localhost:8083` |
| Kafka UI | Topics, connectors and consumers | `https://kafka-ui.localhost` |

HTTP UIs go through Traefik on host 80/443 with `*.localhost` hostnames and a
shared local TLS certificate. TCP services and the Kafka Connect REST API use
dedicated host ports.

Kafka Connect takes its plugins from the stack file, so a connector that does not ship with the
Debezium image does not mean building one:

```yaml
services:
  kafkaConnect:
    plugins:
      - name: redis-kafka-connect
        url: https://github.com/redis-field-engineering/redis-kafka-connect/releases/download/v1.1.0/redis-redis-kafka-connect-1.1.0.zip
        sha256: 7e4249ca356f702220cf09e9e150c8336e24824d7a72fce05d7999bf2a7e03df
      - name: redis-kafka-connect      # same name = same plugin directory, for an SMT
        file: connect-plugins/my-outbox-router.jar
    connectors:
      - name: orders-outbox
        kind: debezium.postgres
        optional: true                 # its table only exists after the app boots once
        tables: { include: [public.outbox] }
```

An `optional` connector whose source does not exist yet is a warning during `pyahu up`, not a
failure; `pyahu connectors apply` registers it after the first boot, and `pyahu connectors status`
reports **every task**, because a connector stays `RUNNING` while its only task is `FAILED`.

## Install

Every release is published to
[GitHub Releases](https://github.com/pyahu/cli/releases) — binaries for macOS, Linux and Windows
(amd64 and arm64), plus `checksums.txt`. Every install method below pulls from there.

### mise

Pinning the CLI next to the rest of a project's toolchain is the recommended way: everyone on the
team gets the same version, and it is recorded in the repo.

```bash
# in a project, resolves latest and writes its exact version to ./mise.toml
mise use --pin "github:pyahu/cli@latest"

# or for your user, everywhere
mise use -g --pin "github:pyahu/cli@latest"

mise install
```

```toml
# mise.toml
[tools]
"github:pyahu/cli" = "<resolved-version>"
```

### Pyahu toolchain

The [Pyahu toolchain](https://github.com/pyahu/toolchain) already pins the CLI in its `cloud`
profile, along with k3d, kubectl and the rest of the Kubernetes set. If you use it, you have the
CLI:

```bash
export MISE_ENV=cloud    # add to your shell rc
mise install
```

### Install script (macOS and Linux)

Downloads the release for your platform from GitHub Releases:

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh

# pick an install dir, no sudo
curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --bin-dir "$HOME/.local/bin"
```

### go install

```bash
go install github.com/pyahu/cli/cmd/pyahu@latest   # Go 1.26+
```

### Staying up to date

The CLI warns when it is behind, and `pyahu upgrade` replaces a binary it owns after verifying the
release checksum. A binary managed by mise is left to mise, and the command says so.

```bash
pyahu check-update
pyahu upgrade
```

Full instructions, manual download and shell completion:
[Installation docs](https://cli.pyahu.io/docs/installation).

### Requirements

- Docker or Podman, running
- [k3d](https://k3d.io) 5.x

`pyahu doctor` checks these and your local ports before bringing the stack up.

## Quick start

```bash
pyahu init --preset platform   # write pyahu.yaml (or --preset minimal)
pyahu doctor                   # validate Docker/Podman, k3d and ports
pyahu up                       # create the cluster and apply the services
pyahu certs trust              # trust the local CA for https://*.localhost

pyahu services                 # list services and endpoints
pyahu connectors apply         # register connectors whose source needed the app to boot
pyahu connectors status        # connector and task state; non-zero if any task is not RUNNING
eval "$(pyahu env)"            # load connection env vars into your shell
pyahu down                     # tear it all down
```

The default stack file is `pyahu.yaml`, discovered from the current directory
upward. See the [command reference](https://cli.pyahu.io/docs/comandos) for every
command and flag.

## Documentation

- [Overview & getting started](https://cli.pyahu.io/docs)
- [Commands](https://cli.pyahu.io/docs/comandos)
- [Configuration](https://cli.pyahu.io/docs/configuracao)
- [Kafka Connect & Debezium](https://cli.pyahu.io/docs/kafka-connect-debezium)
- [Local certificates](https://cli.pyahu.io/docs/certificados)
- [Backup & restore](https://cli.pyahu.io/docs/backup-restore)

## Scope

The current v1 focus is **local infrastructure only**. Application deployment,
remote clusters, and Telepresence-style workflows are intentionally out of scope.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the dev
setup, build/test commands, and conventions. Please read the
[Code of Conduct](CODE_OF_CONDUCT.md) and report security issues per the
[security policy](SECURITY.md).

## License

[MIT](LICENSE) © Pyahu
