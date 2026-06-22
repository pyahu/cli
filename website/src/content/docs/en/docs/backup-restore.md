---
title: Backup and restore
description: How to generate dumps of the local PostgreSQL and restore local or S3 files.
---

Backup and restore are commands, not YAML configuration. The goal is to be direct: take a real database dump from the local cluster to the host disk and restore it when needed.

## Local backup

```bash
pyahu backup postgres app --dir ./backups
```

The CLI runs `pg_dump --format=custom --no-owner --no-acl` on the primary PostgreSQL pod and writes a `.dump` file to the provided directory.

## Local restore

```bash
pyahu restore postgres app --source ./backups/pyahu-local-app-20260622-131500.dump --yes
```

The restore uses `pg_restore`. By default, destructive operations ask for confirmation; in scripts, pass `--yes` intentionally.

## Restore from S3

```bash
pyahu restore postgres app --source s3://my-bucket/dev/app.dump --yes
```

For S3-compatible providers, provide the endpoint:

```bash
pyahu restore postgres app \
  --source s3://bucket/app.dump \
  --s3-endpoint-url http://localhost:9000 \
  --yes
```

The download uses `aws s3 cp` on the host when the source starts with `s3://`.

## Quick test

```bash
pyahu backup postgres app --dir ./backups
pyahu restore postgres app --source "$(ls -t ./backups/*.dump | head -1)" --yes
```

Keep dumps outside `.pyahu/local`. The `.pyahu/local` directory is generated state and can be removed together with local tests.
