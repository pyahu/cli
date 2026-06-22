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
brings up PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect with Debezium, and
Kafka UI, with local TLS and predictable endpoints, and without turning your
setup into a side project.

It is intentionally lightweight: k3d plus generated resources. Normal operation
does not require `kubectl` or `helm`.

```console
$ pyahu init --preset platform
$ pyahu up
✓ Checando dependências locais  (137ms)
✓ Cluster pyahu-local criado  (8.4s)
✓ Aguardando a API do Kubernetes  (3.1s)
✓ Configurando serviços: postgres, zitadel, rabbitmq, kafka, kafka-connect, kafka-ui

✓ Pyahu local stack is ready
```

## What it provisions

| Service | Role | Endpoint |
| --- | --- | --- |
| PostgreSQL | Relational database (optional read replicas) | `localhost:5432` |
| ZITADEL | Identity & OIDC over local HTTPS | `https://zitadel.localhost` |
| RabbitMQ | AMQP messaging + management UI | `localhost:5672` · `https://rabbitmq.localhost` |
| Kafka | Event streaming broker (KRaft) | `localhost:9092` |
| Kafka Connect | Declarative connectors with Debezium CDC | `http://localhost:8083` |
| Kafka UI | Topics, connectors and consumers | `https://kafka-ui.localhost` |

HTTP UIs go through Traefik on host 80/443 with `*.localhost` hostnames and a
shared local TLS certificate. TCP services and the Kafka Connect REST API use
dedicated host ports.

## Install

Released binaries are published for macOS, Linux, and Windows.

```bash
# Install script (macOS and Linux)
curl -fsSL https://cli.pyahu.io/install.sh | sh

# pick an install dir, no sudo
curl -fsSL https://cli.pyahu.io/install.sh | sh -s -- --bin-dir "$HOME/.local/bin"

# Go 1.26+
go install github.com/pyahu/cli/cmd/pyahu@latest
```

Or grab a binary from the [releases page](https://github.com/pyahu/cli/releases).
Full instructions, verification and shell completion: [Installation docs](https://cli.pyahu.io/docs/instalacao).

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
