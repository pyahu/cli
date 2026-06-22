# Pyahu CLI V1 Implementation Plan

Status: draft
Date: 2026-06-21

## Current Repository Gap

The repository started from a legacy Stacks skeleton:

- module and binary names did not match `pyahu`
- cluster runtime was a Kind placeholder
- provider placeholders included services outside v1 scope
- docs/examples described Kind, Infisical, ingress, and cert-manager

V1 needs to pivot this into:

- module and binary aligned with `pyahu`
- k3d local cluster runtime
- PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect, and Kafka UI local
  services
- Kubernetes Secret and ConfigMap support
- no observability, Infisical, Glitch, Vault, ingress provider, or
  cert-manager provider in the v1 surface

## Target Layout

```text
cmd/pyahu/
internal/cli/
internal/config/
internal/events/
internal/runtime/k3d/
internal/kube/
internal/ui/
internal/doctor/
pkg/schema/
examples/
docs/specs/
```

Keep `internal` for implementation. Keep `pkg/schema` only for public stack
types that are safe to import.

## Engineering Standards

- Every command is created by a constructor that accepts dependencies; avoid
  package-level mutable command state.
- `main` is the only place that calls `os.Exit`.
- All long-running work accepts `context.Context`.
- Use `signal.NotifyContext` so Ctrl-C cancels orchestration.
- Wrap errors with enough context for logs, then convert to concise user-facing
  messages at the CLI boundary.
- Use typed domain errors for validation, dependency checks, cluster failures,
  service failures, readiness timeouts, and interruption.
- Use strict YAML decoding.
- Use `slog` for diagnostic logs and keep normal progress separate from logs.
- Do not shell out to `kubectl` or `helm` in normal operation.
- Shelling out to `k3d` is acceptable in v1 because k3d config files are the
  documented interface and the k3d Go API is not the user contract.
- Golden-test human output where stable; test JSON output as structured data.
- Add integration tests that can be skipped unless Docker/k3d are available.

## Orchestration Flow

`pyahu up`:

1. Start event renderer.
2. Load and strictly validate stack file.
3. Run preflight checks:
   - Docker or Podman API available
   - k3d installed
   - default or configured ports free
   - enough CPU, memory, and disk for enabled services
4. Render `.pyahu/local/k3d.yaml`.
5. Create or reuse k3d cluster.
6. Wait for Kubernetes API and node readiness.
7. Create namespace and apply common labels.
8. Apply ConfigMaps and Secrets.
9. Reconcile service resources:
   - PostgreSQL first
   - RabbitMQ and Kafka
   - Kafka Connect and Kafka UI after Kafka
   - ZITADEL after PostgreSQL is ready
10. Wait for readiness checks.
11. Print connection summary.
12. Persist local state under `.pyahu/local/state.json`.

`pyahu down`:

1. Load stack file or local state.
2. If `--keep-cluster`, uninstall stack resources and keep the k3d cluster.
3. Otherwise delete the Pyahu-owned k3d cluster.
4. Remove `.pyahu/local/` state for the deleted cluster.

## Service Implementation

V1 services are generated as Kubernetes resources through client-go. The public
surface is the stack schema, not a provider/plugin API. A provider abstraction
can be introduced when there is a second runtime or an operator-backed mode.

## Milestones

### Milestone 1: CLI Foundation

- Rename binary path to `cmd/pyahu`.
- Decide module rename strategy.
- Implement root command, global flags, version output, and shell completion.
- Add command constructors without package-level mutable state.
- Implement event model and human renderer.
- Implement JSON output contracts.
- Implement signal cancellation and exit-code mapping.
- Add unit tests for command parsing and output modes.

### Milestone 2: Stack File

- Implement v1alpha1 schema.
- Implement strict YAML load and defaulting.
- Implement validation with actionable field paths.
- Implement `pyahu init` and presets.
- Implement Secret and ConfigMap materialization plan.
- Add unit tests and example stack files.

### Milestone 3: k3d Runtime

- Implement `pyahu doctor`.
- Generate k3d config from stack cluster settings.
- Create, detect, reuse, and delete Pyahu-owned clusters.
- Wait for Kubernetes API and node readiness.
- Store local state in `.pyahu/local/state.json`.
- Add integration tests gated behind an environment variable.

### Milestone 4: Kubernetes Layer

- Build Kubernetes clients from the k3d kubeconfig.
- Implement namespace, label, Secret, and ConfigMap reconciliation.
- Generate a local CA and `localhost`/`*.localhost`/`zitadel.localhost`
  certificate, then reconcile the TLS Secret and CA ConfigMap.
- Implement generated Service, StatefulSet, Deployment, Ingress, and Job
  reconciliation.
- Add readiness helpers.

### Milestone 5: Services

- PostgreSQL via local StatefulSet.
- RabbitMQ via local StatefulSet.
- Kafka via local KRaft StatefulSet.
- Kafka Connect via Debezium Connect Deployment.
- Kafka UI via Kafbat Kafka UI Deployment.
- ZITADEL via official container and the v1 PostgreSQL service.
- Implement `status`, `logs`, and `env` for every service.
- Implement command-driven PostgreSQL backup/restore with host dump files and
  optional S3 restore via AWS CLI.

### Milestone 6: Polish and Release Readiness

- Replace old Stacks docs/examples with Pyahu v1 docs/examples.
- Add troubleshooting docs for port conflicts, image pulls, Docker resources,
  Kafka local listener issues, and ZITADEL local URL issues.
- Add release builds for Linux, macOS, and Windows.
- Add smoke tests for `init`, `doctor`, `up`, `status`, `env`, `logs`, and
  `down`.
- Measure cold and warm provisioning times.

## Technical Risks

Kafka local access:

- KRaft external listener configuration on k3d needs real smoke testing.
- Acceptance requires `KAFKA_BOOTSTRAP_SERVERS=localhost:<port>` to work from
  the host.

ZITADEL local URL:

- ZITADEL issuer URLs are strict. The generated config must keep browser,
  service, and CLI views consistent.
- The default local issuer uses `https://zitadel.localhost` through k3s
  Traefik and the generated local TLS Secret. V1 should not add Envoy,
  cert-manager, ACME, or public CA issuance for `.localhost`.

Resource pressure:

- Kafka plus ZITADEL can still be heavy. `doctor` must warn before the cluster
  starts if container resources are too low.

## Open Decisions

- Exact default k3s image version used by generated k3d clusters.
- Whether to add an operator-backed mode after the lightweight local v1 works.
