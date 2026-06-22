---
title: Local certificates
description: How the Pyahu CLI generates and installs local TLS for localhost and *.localhost.
---

Pyahu does not use a public CA for `.localhost`. The CLI generates a local CA, creates a certificate for `localhost` and `*.localhost`, and writes the TLS pair to the Kubernetes Secret `pyahu-local-tls`.

The `*.localhost` wildcard covers all HTTP UIs behind Traefik
(`zitadel.localhost`, `kafka-ui.localhost`, `rabbitmq.localhost`), so trusting the
CA once covers them all. Plain `localhost` is listed separately, because the
wildcard does not match the host without a subdomain.

## Status

```bash
pyahu certs status
```

Expected example after trusting the CA:

```text
local CA:      ~/Library/Application Support/pyahu/certs/ca.crt
CA status:     valid until 2036-06-19
host trust:    trusted
certificate:   .pyahu/local/certs/localhost.crt
cert status:   valid until 2027-07-24
domains:       *.localhost, localhost
```

## Trust the CA on the host

```bash
pyahu certs trust
```

On macOS, the command uses the system trust store and may ask for a password. After that, `curl` and browsers such as Safari/Chrome should accept `https://zitadel.localhost`.

```bash
curl https://zitadel.localhost/debug/healthz
```

Expected response:

```text
ok
```

## Rotate

```bash
pyahu certs rotate
pyahu certs trust
pyahu up
```

After rotation, run `pyahu up` to update the TLS Secret in the cluster.

## Local paths

| Item | Linux | macOS |
| --- | --- | --- |
| Local CA | `~/.config/pyahu/certs` | `~/Library/Application Support/pyahu/certs` |
| Project certificate | `.pyahu/local/certs` | `.pyahu/local/certs` |
