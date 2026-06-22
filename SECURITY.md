# Security Policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Report them privately via GitHub's
[security advisories](https://github.com/pyahu/cli/security/advisories/new) or by
email to **security@pyahu.io**. Include a description, reproduction steps, and
the affected version (`pyahu --version`).

We aim to acknowledge reports within a few business days and will keep you
updated on the fix and disclosure timeline.

## Supported versions

Pyahu CLI is pre-1.0 and under active development. Security fixes target the
latest released version.

## Scope and local defaults

Pyahu CLI provisions a **local** development stack. The default credentials it
ships (for example `pyahu_local` and `Password1!`) and the generated local CA
are meant for local development on a k3d cluster only. They are intentionally
well-known and are **not secrets**.

Do not use Pyahu CLI, its presets, or its default credentials for anything
exposed to a network you do not fully control. The local CA is installed only
into your own host trust store when you run `pyahu certs trust`.
