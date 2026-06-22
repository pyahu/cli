# Pyahu CLI Specifications

This directory contains the working specifications for the first Pyahu CLI
release.

Start here:

- [V1 local cluster product specification](pyahu-cli-v1.md)
- [V1 stack file schema](stack-file-v1alpha1.md)
- [V1 implementation plan](implementation-plan-v1.md)

The v1 scope is intentionally narrow: a local k3d cluster that provisions the
base infrastructure services developers need to start building on Pyahu:
PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect with Debezium, and Kafka UI.
