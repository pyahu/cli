# Contributing to Pyahu CLI

Thanks for your interest in improving Pyahu CLI. This guide covers the dev
setup, the project layout, and the conventions we follow.

By participating you agree to our [Code of Conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- [mise](https://mise.jdx.dev) — provides the toolchain (Go, k3d, Node)
- Docker or Podman — required at runtime for k3d smoke tests

```bash
mise trust
mise install
```

## Build, test, lint

```bash
mise run build   # build ./bin/pyahu
mise run test    # go test ./...
mise run lint    # go vet ./...
mise run fmt     # gofmt -s -w
mise run tidy    # go mod tidy
mise run smoke   # offline CLI smoke checks
```

Unit tests must not require Docker, k3d, or Kubernetes — use the existing fakes.
Real k3d smoke tests stay explicit, not part of the default unit run.

## Project layout

```text
cmd/pyahu/                  # binary entry point (the only package that calls os.Exit)
internal/cli/               # Cobra commands, output rendering, styling
internal/catalog/           # service metadata for services/describe
internal/config/            # stack discovery, strict YAML load, presets
internal/doctor/            # dependency and port checks
internal/runtime/k3d/       # k3d config generation and lifecycle
internal/kube/              # client-go reconciliation, one file per service
pkg/schema/                 # public v1alpha1 stack schema
website/                    # marketing page (Astro) and docs (Starlight)
docs/specs/                 # product and implementation specs
```

## Conventions

- **Lean and explicit.** Simple code over patterns. No speculative complexity.
- **Strict config.** `pkg/schema` uses strict YAML decoding; unknown fields are
  errors. Validate at boundaries (user input, external APIs); trust internal code.
- **Context everywhere.** Every long-running operation takes a `context.Context`.
  Ctrl-C cancels and exits with code 130.
- **No kubectl/helm.** Normal operation may shell out to `k3d`, but must not
  require `kubectl` or `helm`. Reconcile Kubernetes resources through client-go.
- **Idempotent.** A second `pyahu up` converges or no-ops.
- **Output.** Keep human output concise; color/spinners only on a TTY. JSON,
  piped, `--no-color`, `NO_COLOR`, and `--quiet` output stay plain. Use
  `--verbose` for tool output.
- **Tests.** Prefer stdlib `testing` with table-driven tests. Test Cobra
  commands with `SetArgs` and stdout/stderr buffers. Test manifest builders as
  plain Go objects.

## Commits and releases

We use [Conventional Commits](https://www.conventionalcommits.org) and
trunk-based development. Pushes to `main` are tested in CI; releases are cut
automatically from the commit types:

- `feat:` → minor, `fix:` → patch, `feat!:` / `BREAKING CHANGE` → major
- `docs:`, `chore:`, `refactor:`, `style:`, `test:`, `ci:` → no release

Keep commit subjects short and imperative; put detail in the body when needed.

## Pull requests

1. Fork and branch from `main`.
2. Keep changes focused; add or update tests.
3. Run `mise run fmt`, `mise run lint`, and `mise run test` before pushing.
4. Open a PR with a clear description; CI must pass.

## Website

The marketing page (custom Astro) and docs (Starlight) live in `website/`.

```bash
mise run site-install
mise run site-dev
mise run site-build
```

## Reporting issues

Use the issue templates for bugs and feature requests. For security
vulnerabilities, follow the [security policy](SECURITY.md) instead of opening a
public issue.
