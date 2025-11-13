# Flotilla

Multi-host Docker orchestration with the ergonomics of Dockge and the reach of Portainer.

[![CI](https://github.com/mikeysoft/flotilla/actions/workflows/ci.yml/badge.svg)](https://github.com/mikeysoft/flotilla/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Docs](https://img.shields.io/badge/docs-flotilla.dev-informational.svg)](docs/README.md)

---

Flotilla centralizes Docker host management behind a secure hub-and-spoke architecture. Agents connect over WebSockets, execute container and stack operations locally, and stream telemetry back to the management server for real-time visibility.

## Highlights

- **Full lifecycle management** for containers and Docker Compose stacks across many hosts.
- **Bidirectional WebSocket control plane** with resilient reconnect and heartbeat handling.
- **Metrics pipeline** covering CPU, memory, network, and disk—stored in InfluxDB for historical analysis.
- **Modern web UI** (React + Tailwind) with live updates, compose editor, and streamlined stack operations.
- **Secure by default** with API key auth, TLS-first design, and planned RBAC/audit trails.

## Architecture at a Glance

```
Management Server (Go, PostgreSQL)
    ├── REST + WebSocket APIs
    ├── InfluxDB metrics writer
    └── React/Tailwind frontend
         ↓ secure WebSocket
    Agents (Go, Docker SDK)
         ↓ Docker Engine
    Containers & Stacks
```

The hub maintains agent sessions, schedules work, and aggregates state. Agents run close to Docker hosts, translating commands into Docker Engine calls and returning responses, events, and metrics.

## Quick Start

1. **Review requirements** in [`docs/setup.md`](docs/setup.md#prerequisites).
2. **Deploy the management server** using the provided Docker Compose recipe.
3. **Provision the first admin user** and API keys.
4. **Enroll agents** on each host (binary, container, or systemd service).
5. **Verify connectivity** via the Flotilla web console and dashboards.

Production-first guidance, hardening tips, and troubleshooting live in [`docs/setup.md`](docs/setup.md).

## Project Layout

| Path | Purpose |
|------|---------|
| `cmd/server`, `cmd/agent` | Entry points for the hub and agent binaries |
| `internal/server`, `internal/agent` | Application logic and integrations |
| `web/` | React + TypeScript frontend |
| `deployments/` | Docker, systemd, and migration assets |
| `docs/` | Operations, release, and architecture documentation |
| `scripts/`, `Makefile` | Developer tooling and automation |

## Documentation

- [`docs/README.md`](docs/README.md) – documentation index
- [`docs/setup.md`](docs/setup.md) – production deployment & first-run guide
- [`docs/versioning.md`](docs/versioning.md) – release, tagging, and manifest strategy
- [`docs/service-registration.md`](docs/service-registration.md) – native service install procedures
- [`docs/development.md`](docs/development.md) – contributor-focused environment setup

Additional design notes and feature deep-dives are under [`features/`](features/).

## Contributing

We love community contributions! Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) for coding standards, workflow expectations, and how to get in touch. Bug reports and feature ideas are welcome via our issue templates.

## Security

Report vulnerabilities privately to `security@mikeysoft.com`. See [`SECURITY.md`](SECURITY.md) for our disclosure timeline and support guarantees.

## License

Copyright © 2025 The Flotilla Authors.

Licensed under the [Apache License, Version 2.0](LICENSE).
