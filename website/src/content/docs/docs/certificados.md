---
title: Certificados locais
description: Como a Pyahu CLI gera e instala TLS local para localhost e *.localhost.
---

A Pyahu não usa CA pública para `.localhost`. A CLI gera uma CA local, cria um certificado para `localhost` e `*.localhost`, e grava o par TLS no Kubernetes Secret `pyahu-local-tls`.

O wildcard `*.localhost` cobre todas as UIs HTTP atrás do Traefik
(`zitadel.localhost`, `kafka-ui.localhost`, `rabbitmq.localhost`), então confiar a
CA uma vez cobre todas. O `localhost` puro fica na lista à parte, porque o
wildcard não casa com o host sem subdomínio.

## Status

```bash
pyahu certs status
```

Exemplo esperado depois de confiar a CA:

```text
local CA:      ~/Library/Application Support/pyahu/certs/ca.crt
CA status:     valid until 2036-06-19
host trust:    trusted
certificate:   .pyahu/local/certs/localhost.crt
cert status:   valid until 2027-07-24
domains:       *.localhost, localhost
```

## Confiar a CA no host

```bash
pyahu certs trust
```

No macOS, o comando usa o trust store do sistema e pode pedir senha. Depois disso, `curl` e browsers como Safari/Chrome devem aceitar `https://zitadel.localhost`.

```bash
curl https://zitadel.localhost/debug/healthz
```

Resposta esperada:

```text
ok
```

## Rotacionar

```bash
pyahu certs rotate
pyahu certs trust
pyahu up
```

Depois da rotação, rode `pyahu up` para atualizar o Secret TLS no cluster.

## Caminhos locais

| Item | Linux | macOS |
| --- | --- | --- |
| CA local | `~/.config/pyahu/certs` | `~/Library/Application Support/pyahu/certs` |
| Certificado do projeto | `.pyahu/local/certs` | `.pyahu/local/certs` |
