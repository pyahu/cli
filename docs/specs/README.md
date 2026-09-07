# Pyahu CLI Specifications

This directory contains the working specifications for the first Pyahu CLI
release.

Start here:

- [V1 local cluster product specification](pyahu-cli-v1.md)
- [V1 stack file schema](stack-file-v1alpha1.md)
- [V1 implementation plan](implementation-plan-v1.md)
- [Redis service, Kafka Connect plugins, and deferred connectors](redis-and-connect-plugins-v1.md) (v0.2.0)

The v1 scope is intentionally narrow: a local k3d cluster that provisions the
base infrastructure services developers need to start building on Pyahu:
PostgreSQL, ZITADEL, RabbitMQ, Kafka, Kafka Connect with Debezium, and Kafka UI.
v0.2.0 adds Redis (Valkey), declarative Kafka Connect plugins, and deferred
connector registration.
