---
title: Configuração
description: Estrutura básica do pyahu.yaml e regras importantes da v1.
---

O arquivo padrão é `pyahu.yaml`. A CLI procura esse arquivo do diretório atual para cima. Use `--file` ou `-f` quando quiser apontar para outro caminho.

## Exemplo mínimo

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

## Stack completa

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

## Portas

Os serviços **TCP** expõem uma porta de host dentro do serviço dono do endpoint:

```yaml
services:
  postgres:
    ports:
      primary: 15432
  kafka:
    ports:
      bootstrap: 19092
```

As **UIs HTTP** (ZITADEL, RabbitMQ, Kafka UI) não usam porta de host: elas passam
pelo Traefik em 80/443 com hostnames `*.localhost` (`https://kafka-ui.localhost`,
`https://rabbitmq.localhost`, `https://zitadel.localhost`). Para trocar o domínio
do ZITADEL, use `services.zitadel.externalURL`.

Não use `cluster.ports` em presets ou documentação nova. A CLI mantém compatibilidade silenciosa com esse formato antigo, mas ele não é a superfície da v1.

:::caution[Atualizando de um `pyahu.yaml` antigo]
`pyahu init` já gera o formato novo. Se o seu arquivo foi criado por uma versão
anterior, ele pode ter `externalURL: https://zitadel.localhost:8443` com porta
cravada, e o ZITADEL vai insistir em `:8443`, que não é mais mapeada. Troque por
`externalURL: https://zitadel.localhost` (sem porta) e rode `pyahu up`. Os campos
`zitadel.ports`, `rabbitmq.ports.management` e `kafkaUI.ports.http` continuam
sendo aceitos, mas são ignorados.
:::

## Config global

Você pode definir defaults globais no diretório de configuração do sistema:

| Sistema | Caminho típico |
| --- | --- |
| Linux | `~/.config/pyahu.yaml` |
| macOS | `~/Library/Application Support/pyahu.yaml` |

O arquivo global é carregado primeiro; o `pyahu.yaml` do projeto sobrescreve os valores.

## Credenciais locais

Credenciais de PostgreSQL, Zitadel e RabbitMQ podem ficar no `pyahu.yaml` local ou no config global.

No PostgreSQL, mudança de senha depois que o volume já existe atualiza os Secrets e a saída da CLI, mas o usuário dentro do banco pode continuar com a senha antiga. Para rotação local, recrie o cluster ou altere a role dentro do PostgreSQL.
