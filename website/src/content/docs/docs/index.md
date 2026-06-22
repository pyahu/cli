---
title: Visão geral
description: Comece com a Pyahu CLI e suba uma stack local completa em k3d.
---

A Pyahu CLI é a porta de entrada local para a **Plataforma Pyahu**: um subconjunto fiel
da experiência real, rodando na sua máquina. Ela sobe uma stack de desenvolvimento em
k3d/k3s usando recursos Kubernetes gerados pela própria CLI, sem você precisar escrever
manifests, `kubectl` ou `helm`.

É **Kubernetes de verdade**, com Traefik, PersistentVolumes, ConfigMaps e Secrets. A CLI
não esconde o k8s do dev; ela facilita o provisionamento e a operação, e o cluster continua
seu para inspecionar (`pyahu kubeconfig`) quando quiser.

## Instalação rápida

```bash
curl -fsSL https://cli.pyahu.io/install.sh | sh
```

Outros métodos (script, `go install`, download manual) e os pré-requisitos estão em
[Instalação](/docs/instalacao).

## Primeiro cluster

```bash
pyahu init --preset platform   # cria o pyahu.yaml
pyahu doctor                   # valida Docker/Podman, k3d e portas
pyahu up                       # cria o cluster e aplica os serviços
```

Ao terminar, `pyahu up` imprime o cluster, o namespace, o caminho do kubeconfig e as
variáveis de conexão prontas para colar.

:::tip
Rode os comandos no diretório do seu projeto. A CLI procura o `pyahu.yaml` do diretório
atual para cima, então qualquer subpasta do projeto enxerga a mesma stack.
:::

## Serviços do preset `platform`

| Serviço | Endpoint local |
| --- | --- |
| PostgreSQL | `localhost:5432` |
| PostgreSQL (réplicas de leitura) | `localhost:5433` quando `readReplicas > 0` |
| ZITADEL | `https://zitadel.localhost` |
| RabbitMQ | `localhost:5672` |
| RabbitMQ Management | `https://rabbitmq.localhost` |
| Kafka | `localhost:9092` |
| Kafka Connect | `http://localhost:8083` |
| Debezium | configurado via Kafka Connect |
| Kafka UI | `https://kafka-ui.localhost` |

As UIs HTTP (ZITADEL, RabbitMQ, Kafka UI) passam pelo Traefik em 80/443 com
hostnames `*.localhost` e o certificado local. Os serviços TCP (PostgreSQL,
Kafka, RabbitMQ AMQP) e a REST do Kafka Connect mantêm portas dedicadas.

O preset `minimal` sobe apenas o PostgreSQL. Veja [Configuração](/docs/configuracao) para
ajustar serviços, portas e credenciais.

## Conectar seus apps

```bash
# imprime as variáveis de conexão (shell, dotenv ou json)
pyahu env
eval "$(pyahu env)"
```

Para ver tudo que está rodando e seus endpoints:

```bash
pyahu services
pyahu describe postgres
```

A referência completa de cada comando está em [Comandos](/docs/comandos).

## TLS local

Quando o ZITADEL está habilitado, a CLI emite um certificado local para `*.localhost`.
Confie a CA no host uma vez para usar `https://zitadel.localhost` sem avisos:

```bash
pyahu certs trust
```

Detalhes em [Certificados locais](/docs/certificados).

## Limpeza

```bash
pyahu down                 # remove o cluster k3d
pyahu down --keep-cluster  # mantém o cluster, remove só o namespace da stack
```

Se você mudar portas depois que o cluster já existe, recrie-o. O k3d fixa os mapeamentos
de porta na criação:

```bash
pyahu down
pyahu up
```
