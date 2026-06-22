---
title: Backup e restore
description: Como gerar dumps do PostgreSQL local e restaurar arquivos locais ou S3.
---

Backup e restore são comandos, não configuração YAML. O objetivo é ser direto: tirar um dump real do banco no cluster local para o disco do host e restaurar quando necessário.

## Backup local

```bash
pyahu backup postgres app --dir ./backups
```

A CLI executa `pg_dump --format=custom --no-owner --no-acl` no pod primário do PostgreSQL e grava um arquivo `.dump` no diretório informado.

## Restore local

```bash
pyahu restore postgres app --source ./backups/pyahu-local-app-20260622-131500.dump --yes
```

O restore usa `pg_restore`. Por padrão, operações destrutivas pedem confirmação; em scripts, passe `--yes` intencionalmente.

## Restore a partir de S3

```bash
pyahu restore postgres app --source s3://my-bucket/dev/app.dump --yes
```

Para provedores compatíveis com S3, informe o endpoint:

```bash
pyahu restore postgres app \
  --source s3://bucket/app.dump \
  --s3-endpoint-url http://localhost:9000 \
  --yes
```

O download usa `aws s3 cp` no host quando a origem começa com `s3://`.

## Teste rápido

```bash
pyahu backup postgres app --dir ./backups
pyahu restore postgres app --source "$(ls -t ./backups/*.dump | head -1)" --yes
```

Guarde dumps fora de `.pyahu/local`. O diretório `.pyahu/local` é estado gerado e pode ser removido junto com testes locais.
