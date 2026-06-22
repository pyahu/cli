---
title: Comandos
description: Referência completa de todos os comandos da Pyahu CLI, com flags e exemplos.
---

Esta é a referência completa da Pyahu CLI. Todos os comandos aceitam as flags globais da
próxima seção. A maioria suporta `--output json` para uso em scripts.

```text
pyahu [comando] [flags]
```

## Flags globais

Disponíveis em qualquer comando:

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `-f, --file` | descoberta automática | Caminho do `pyahu.yaml` do projeto. O config global continua sendo aplicado. |
| `-o, --output` | `human` | Formato de saída: `human` ou `json`. |
| `--no-color` | `false` | Desativa cores na saída. |
| `-q, --quiet` | `false` | Suprime saída não essencial. |
| `-v, --verbose` | `false` | Mostra a saída das ferramentas (k3d etc.). |
| `--no-input` | `false` | Nunca pergunta nada de forma interativa. |
| `--version` | — | Mostra versão, commit e data de build. |
| `-h, --help` | — | Ajuda do comando. |

A CLI procura o arquivo de stack do diretório atual para cima, nesta ordem: `pyahu.yaml`,
`pyahu.yml`, `.pyahu/stack.yaml`, `.pyahu/stack.yml`. Use `--file` só para apontar outro
caminho.

---

## Ciclo de vida

### `pyahu init`

Cria um `pyahu.yaml` a partir de um preset.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--preset` | `minimal` | Preset inicial: `minimal` (só PostgreSQL) ou `platform` (stack completa). |
| `--force` | `false` | Sobrescreve um stack file existente. |

```bash
pyahu init --preset platform
pyahu init --preset minimal --force
pyahu init --preset platform -f infra/pyahu.yaml
```

### `pyahu up`

Cria o cluster k3d quando necessário e reconcilia os recursos Kubernetes. É **idempotente**:
um segundo `pyahu up` converge ou não faz nada.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--skip-wait` | `false` | Aplica os recursos sem esperar pela prontidão dos serviços. |

Fluxo: preflight (doctor) → cria/reutiliza o cluster → espera a API do Kubernetes → aplica
os serviços → espera prontidão → imprime o resumo.

```bash
pyahu up
pyahu up --skip-wait
pyahu up --output json
```

Saída humana ao final:

```text
Pyahu local stack is ready
cluster:    pyahu-local
namespace:  pyahu-local-dev
kubeconfig: /home/voce/.config/k3d/kubeconfig-pyahu-local.yaml

POSTGRES_URL                 postgresql://pyahu:pyahu_local@localhost:5432/app?sslmode=disable
ZITADEL_ISSUER               https://zitadel.localhost
...

next: eval "$(pyahu env)"
```

:::caution
Mudar portas de host depois que o cluster existe exige recriar o cluster. O k3d fixa os
mapeamentos na criação. `pyahu up` detecta mapeamentos faltando e pede `pyahu down` seguido
de `pyahu up`.
:::

### `pyahu down`

Remove os recursos locais da Pyahu.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--keep-cluster` | `false` | Remove só o namespace da stack e mantém o cluster k3d. |

```bash
pyahu down                 # apaga o cluster k3d inteiro
pyahu down --keep-cluster  # mantém o cluster, remove o namespace
```

### `pyahu doctor`

Verifica dependências e portas locais antes de subir a stack. Funciona mesmo sem stack file
(usa defaults para a checagem).

```bash
pyahu doctor
pyahu doctor --output json
```

Ele checa:

- `k3d` instalado no `PATH`
- Docker ou Podman em execução
- Outros clusters locais (k3d/Kind): apenas **aviso**, não falha
- Disponibilidade das portas de host dos serviços habilitados (quando o cluster ainda não existe)

```text
k3d                      ok    k3d is installed
container-runtime        ok    docker is available
local-clusters           ok    no other local Kubernetes clusters detected
port:postgres            ok    127.0.0.1:5432 is available
host                     ok    linux/amd64
```

---

## Inspeção

### `pyahu status`

Mostra o estado do cluster e de cada serviço, incluindo os pods.

```bash
pyahu status
pyahu status --output json
```

```text
cluster: pyahu-local
namespace: pyahu-local-dev
postgres   ready    1/1 ready
zitadel    ready    1/1 ready
kafka      waiting  starting
```

### `pyahu services`

Lista os serviços habilitados, seu estado e endpoints locais. Aliases: `svc`, `ls`.

```bash
pyahu services
pyahu svc
pyahu services --output json
```

```text
cluster:   pyahu-local
namespace: pyahu-local-dev
state:     running

SERVICE        STATUS  VERSION  ENDPOINTS
postgres       ready   18.4     localhost:5432
zitadel        ready   v4.15.2  https://zitadel.localhost
rabbitmq       ready   4.3.2    localhost:5672, https://rabbitmq.localhost
kafka          ready   4.3.0    localhost:9092
kafka-connect  ready   3.5.2    http://localhost:8083
kafka-ui       ready   v1.5.0   https://kafka-ui.localhost
```

### `pyahu describe <service>`

Detalhes de um serviço: status, endpoints (host + interno do cluster), variáveis de ambiente,
detalhes da config e pods.

Serviços válidos: `postgres`, `zitadel`, `rabbitmq`, `kafka`, `kafka-connect`, `kafka-ui`.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--show-secrets` | `false` | Mostra valores secretos na saída humana (por padrão são mascarados). |

```bash
pyahu describe postgres
pyahu describe zitadel --show-secrets
pyahu describe kafka-connect --output json
```

:::note
Na saída humana, senhas e tokens aparecem como `<hidden>` e senhas em URLs viram `hidden`.
Use `--show-secrets` ou `pyahu env` quando precisar dos valores reais.
:::

### `pyahu logs <service>`

Transmite os logs de um serviço.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--follow` | `false` | Acompanha os logs em tempo real. |
| `--tail` | `100` | Número de linhas iniciais a mostrar. |

```bash
pyahu logs postgres --tail 50
pyahu logs zitadel --follow
pyahu logs kafka-connect --tail 200
```

---

## Conexão e dados

### `pyahu env`

Imprime as variáveis de conexão para os apps locais.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--format` | `shell` | `shell` (com `export`), `dotenv` ou `json`. |

```bash
pyahu env                 # export VAR='valor'
pyahu env --format dotenv # VAR=valor
pyahu env --format json
eval "$(pyahu env)"       # carrega no shell atual
```

As variáveis cobrem cada serviço habilitado, por exemplo `POSTGRES_URL`,
`RABBITMQ_URL`, `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_CONNECT_URL`, `ZITADEL_ISSUER`.

### `pyahu kubeconfig`

Imprime o caminho do kubeconfig do cluster local.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--raw` | `false` | Escreve o conteúdo do kubeconfig no stdout, em vez do caminho. |

```bash
pyahu kubeconfig
pyahu kubeconfig --raw > kubeconfig.yaml
export KUBECONFIG="$(pyahu kubeconfig)"
```

### `pyahu backup postgres [database]`

Faz um dump real do PostgreSQL no pod primário direto para um arquivo no host
(`pg_dump --format=custom`). Sem `[database]`, usa o primeiro banco configurado.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--dir` | `.pyahu/backups` | Diretório de host para o arquivo `.dump`. |

```bash
pyahu backup postgres app --dir ./backups
pyahu backup postgres            # usa o primeiro database configurado
```

O arquivo recebe o nome `<stack>-<database>-<YYYYMMDD-HHMMSS>.dump` (UTC).

### `pyahu restore postgres [database]`

Restaura um dump custom do PostgreSQL a partir de um arquivo local ou de `s3://`.

| Flag | Padrão | Descrição |
| --- | --- | --- |
| `--source` | — (obrigatório) | Caminho do arquivo `.dump` ou URI `s3://`. |
| `--s3-endpoint-url` | — | Endpoint S3-compatível para origens `s3://`. |
| `--clean` | `true` | Remove objetos correspondentes antes de restaurar. |
| `--yes` | `false` | Confirma o restore destrutivo sem prompt. |

```bash
pyahu restore postgres app --source ./backups/pyahu-local-app-20260622-131500.dump
pyahu restore postgres app --source s3://meu-bucket/dev/app.dump --yes
pyahu restore postgres app \
  --source s3://bucket/app.dump \
  --s3-endpoint-url http://localhost:9000 \
  --yes
```

:::caution
Com `--clean` (padrão), o restore pode dropar objetos existentes. Em execução interativa ele
pede confirmação; em scripts, `--no-input` ou saída não-humana, passe `--yes` de propósito.
Origens `s3://` usam `aws s3 cp` no host.
:::

---

## TLS local

### `pyahu certs status`

Mostra o estado da CA local, do certificado wildcard e do trust no host.

```bash
pyahu certs status
pyahu certs status --output json
```

```text
local CA:      ~/.config/pyahu/certs/ca.crt
CA status:     valid until 2036-06-19
host trust:    trusted
certificate:   .pyahu/local/certs/localhost.crt
cert status:   valid until 2027-07-24
domains:       *.localhost, localhost, zitadel.localhost
```

### `pyahu certs trust`

Instala a CA local da Pyahu no trust store do host. No macOS pode pedir senha. Depois,
`curl` e browsers aceitam `https://zitadel.localhost` (e as demais UIs `*.localhost`) sem avisos.

```bash
pyahu certs trust
```

### `pyahu certs rotate`

Regenera a CA local e o certificado wildcard. Rode `pyahu certs trust` e `pyahu up` depois
para reconfiar a CA e atualizar o Secret TLS no cluster.

```bash
pyahu certs rotate
pyahu certs trust
pyahu up
```

Mais contexto em [Certificados locais](/docs/certificados).

---

## Shell

### `pyahu completion [shell]`

Gera o script de autocomplete. Shells suportados: `bash`, `zsh`, `fish`, `powershell`.

```bash
pyahu completion zsh > "${fpath[1]}/_pyahu"
pyahu completion bash | sudo tee /etc/bash_completion.d/pyahu > /dev/null
pyahu completion fish > ~/.config/fish/completions/pyahu.fish
```

---

## Saída JSON para scripts

Quase todos os comandos de leitura suportam `--output json`:

```bash
pyahu doctor --output json
pyahu status --output json
pyahu services --output json
pyahu describe postgres --output json
pyahu env --format json
pyahu certs status --output json
```

## Fluxo recomendado

```bash
pyahu init --preset platform
pyahu doctor
pyahu up
pyahu certs trust
pyahu services
eval "$(pyahu env)"
```
