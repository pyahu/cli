---
title: Kafka Connect e Debezium
description: Como a Pyahu CLI cria conectores source e sink no Kafka Connect local.
---

No Kafka Connect, um conector é criado a partir de um JSON de configuração enviado para a REST API do Connect.

Na Pyahu CLI v1, você não precisa manter esse JSON como arquivo. Você declara o conector no `pyahu.yaml`; a CLI gera ou recebe o JSON, grava em um Kubernetes Secret e cria um Job que aplica a configuração no Kafka Connect.

## O que a v1 suporta

A superfície declarativa da CLI cobre dois casos:

- Source Debezium para PostgreSQL, com defaults gerados pela Pyahu.
- Source ou sink custom, usando `config` livre no formato do Kafka Connect.

## Source Debezium/PostgreSQL

Para CDC de Postgres, use `type: source` e `kind: debezium.postgres`:

```yaml
services:
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-cdc
        type: source
        kind: debezium.postgres
        database: app
        topicPrefix: app-cdc
        snapshotMode: initial
        tables:
          include:
            - public.orders
            - public.customers
        config:
          decimal.handling.mode: string
          tombstones.on.delete: "false"
```

Campos como `slot`, `publication`, `topicPrefix`, `database` e `snapshotMode` têm defaults. Na prática, o exemplo mínimo pode ser:

```yaml
services:
  kafkaConnect:
    enabled: true
    connectors:
      - name: app-cdc
        type: source
        tables:
          include:
            - public.orders
```

Com isso, `pyahu up` cria ou atualiza o conector.

## JSON gerado

Para o conector `app-cdc`, a CLI gera um JSON parecido com este:

```json
{
  "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
  "tasks.max": "1",
  "database.hostname": "postgres.pyahu-local-dev.svc.cluster.local",
  "database.port": "5432",
  "database.user": "pyahu",
  "database.password": "pyahu_local",
  "database.dbname": "app",
  "topic.prefix": "app-cdc",
  "plugin.name": "pgoutput",
  "slot.name": "app_cdc_slot",
  "publication.name": "app_cdc_publication",
  "publication.autocreate.mode": "filtered",
  "snapshot.mode": "initial",
  "table.include.list": "public.orders,public.customers",
  "decimal.handling.mode": "string",
  "tombstones.on.delete": "false"
}
```

Esse é o corpo enviado para:

```text
PUT /connectors/app-cdc/config
```

## Sink custom

Sink connectors também são JSON do Kafka Connect. A diferença é que o plugin do sink precisa existir na imagem usada por `services.kafkaConnect.image`.

Exemplo com um sink JDBC em uma imagem customizada que já contém o plugin:

```yaml
services:
  kafkaConnect:
    enabled: true
    image: ghcr.io/acme/connect-with-jdbc-sink
    version: 1.0.0
    connectors:
      - name: orders-jdbc-sink
        type: sink
        kind: custom
        config:
          connector.class: io.confluent.connect.jdbc.JdbcSinkConnector
          tasks.max: "1"
          topics: app-cdc.public.orders
          connection.url: jdbc:postgresql://warehouse.local:5432/orders
          connection.user: warehouse
          connection.password: warehouse_local
          insert.mode: upsert
          pk.mode: record_key
          auto.create: "true"
```

Para conectores custom, a Pyahu não tenta interpretar o payload. O conteúdo de `config` vira o JSON enviado ao Kafka Connect:

```json
{
  "connector.class": "io.confluent.connect.jdbc.JdbcSinkConnector",
  "tasks.max": "1",
  "topics": "app-cdc.public.orders",
  "connection.url": "jdbc:postgresql://warehouse.local:5432/orders",
  "connection.user": "warehouse",
  "connection.password": "warehouse_local",
  "insert.mode": "upsert",
  "pk.mode": "record_key",
  "auto.create": "true"
}
```

O mesmo modelo serve para source custom:

```yaml
services:
  kafkaConnect:
    enabled: true
    connectors:
      - name: external-source
        type: source
        kind: custom
        config:
          connector.class: com.example.SourceConnector
          tasks.max: "1"
          topic: external.events
```

## REST API equivalente

Se você estivesse criando manualmente, o comando seria:

```bash
curl -X PUT \
  -H 'Content-Type: application/json' \
  --data-binary @connector.json \
  http://localhost:8083/connectors/app-cdc/config
```

A Pyahu faz isso dentro do cluster, usando a URL interna do Connect:

```text
http://kafka-connect.pyahu-local-dev.svc.cluster.local:8083
```

## Como a CLI aplica

Durante `pyahu up`, a CLI:

1. Sobe o Deployment do Kafka Connect usando a imagem `quay.io/debezium/connect`.
2. Cria os tópicos internos compactados de config, offset e status.
3. Renderiza ou lê o JSON do conector a partir de `services.kafkaConnect.connectors`.
4. Grava esse JSON em um Secret como `connector.json`.
5. Cria um Job que espera o Connect responder e executa `PUT /connectors/<name>/config`.
6. Aguarda o status do conector ficar `RUNNING`.

Quando você altera a configuração, o Secret e o Job recebem um nome com hash do payload. Isso faz a aplicação ser idempotente e permite reaplicar mudanças com outro `pyahu up`.

## Conferir o conector

```bash
curl http://localhost:8083/connectors
curl http://localhost:8083/connectors/app-cdc/status
```

Também dá para ver pelo Kafka UI quando ele estiver habilitado:

```text
https://kafka-ui.localhost
```

## Regras da v1

- `kind: debezium.postgres` sempre é `type: source`.
- `kind: custom` pode ser `type: source` ou `type: sink`.
- Conectores custom exigem `config.connector.class`.
- Plugins de sink não são instalados automaticamente. Use uma imagem de Kafka Connect que já contenha o plugin necessário.
