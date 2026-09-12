---
title: Troubleshooting
description: Practical checks for ports, resources, certificates, services and Kafka Connect.
---

Start with the CLI's own checks. They catch the most common dependency and port
problems without changing the cluster:

```bash
pyahu doctor
pyahu status
pyahu services
```

Use `--verbose` on a failing command when you need the output from k3d and the
container runtime.

## A local port is already in use

`pyahu doctor` names the service and port that conflict. Find the process that
owns it, stop that process or change the service port in `pyahu.yaml`.

```bash
# macOS or Linux
lsof -nP -iTCP:5432 -sTCP:LISTEN
```

If the Pyahu cluster already exists, changing the YAML is not enough because
k3d fixes host port mappings when it creates the cluster. Recreate it:

```bash
pyahu down
pyahu up
```

This keeps the service data on the host. Add `--purge-data` only when you want a
clean, destructive reset.

## The cluster configuration no longer matches

Pyahu reports configuration drift when an existing k3d cluster has different
server or agent counts, k3s image, network, storage mapping or required host
ports. Run:

```bash
pyahu down
pyahu up
```

The first command removes the cluster but retains its bound storage. The second
creates the cluster with the current project configuration.

## A service stays waiting or fails

Get the service view first:

```bash
pyahu describe postgres
pyahu logs postgres --tail 200
```

Replace `postgres` with the affected service. The supported names are shown by
`pyahu services`.

For Kubernetes events and container state:

```bash
export KUBECONFIG="$(pyahu kubeconfig)"
kubectl get pods -n pyahu-local-dev
kubectl get events -n pyahu-local-dev --sort-by=.lastTimestamp
kubectl describe pod -n pyahu-local-dev <pod-name>
```

Common causes are an image pull failure, a port conflict, a readiness timeout or
too little memory assigned to the container runtime.

## Pods are pending or OOM-killed

The `platform` preset runs seven services. Give Docker Desktop, Colima, Podman
or the active runtime at least 4 CPU cores and 8 GiB of memory, with 15 GiB of
free disk. Then restart the runtime and run `pyahu up` again.

For a smaller machine, start with PostgreSQL and enable dependencies as they
become necessary:

```bash
pyahu init --preset minimal --force
pyahu up
```

`--force` overwrites the existing `pyahu.yaml`; review or commit the current
file before using it.

## A browser rejects the local certificate

Check the CA, certificate and host trust:

```bash
pyahu certs status
pyahu certs trust
```

After a certificate rotation, update the Kubernetes Secret too:

```bash
pyahu certs rotate
pyahu certs trust
pyahu up
```

If one browser remains open during the change, restart it. See
[Local certificates](/docs/certificates) for the generated paths and covered
domains.

## A Kafka Connect plugin is missing

Inspect both the declared artifacts and the classes loaded by the worker:

```bash
pyahu describe kafka-connect
curl -s 'http://localhost:8083/connector-plugins?connectorsOnly=false'
```

For plugin installation errors, inspect the init container:

```bash
export KUBECONFIG="$(pyahu kubeconfig)"
kubectl logs -n pyahu-local-dev deployment/kafka-connect -c install-plugins
```

Check that:

- a downloaded artifact has the correct `sha256`;
- a local `file` path is relative to the directory containing `pyahu.yaml`;
- the archive contains the connector or transform class expected by the config;
- artifacts that need the same classloader share the same plugin `name`.

## A connector is running but its task failed

Kafka Connect can report the connector as `RUNNING` while a task is `FAILED`.
Use the Pyahu view, which checks every task:

```bash
pyahu connectors status
pyahu logs kafka-connect --tail 200
```

If the source table or stream is created only after the application starts,
mark the connector as `optional: true`. Boot the application, then apply it:

```bash
pyahu connectors apply --name <connector-name>
pyahu connectors status
```

## A PostgreSQL password changed but login still fails

Changing the password in `pyahu.yaml` updates the Kubernetes Secret and CLI
output. PostgreSQL may still hold the old password inside an existing data
volume.

For a disposable stack, create a backup if needed and reset the stored data:

```bash
pyahu backup postgres app --dir ./backups
pyahu down --purge-data
pyahu up
```

For data you must keep, change the role password inside PostgreSQL instead of
purging the volume.

## Collect useful information for an issue

Include the following command results when opening a bug report:

```bash
pyahu --version
pyahu doctor --output json
pyahu status --output json
pyahu describe <service>
k3d version
```

Add the failing command with `--verbose` and the relevant service logs. Review
logs before sharing them; `describe` masks secrets by default, but application
or service logs may contain sensitive values.
