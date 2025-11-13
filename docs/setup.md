# Production Setup Guide

This guide walks through deploying Flotilla for the first time in a production (or production-like) environment. It covers management server installation, agent enrollment, and post-install validation.

---

## 1. Prerequisites

| Component | Requirement |
|-----------|-------------|
| Operating System | 64-bit Linux (recommended), macOS, or Windows Server for lab environments |
| Runtime | Docker 24+ (for container deployment) or Go 1.21+ for native binaries |
| Database | PostgreSQL 15+ with a dedicated database/user |
| TLS | Valid certificate/chain for the management server (Let's Encrypt or corporate CA) |
| Network | Outbound TCP 443/8080 from agents to server; inbound 443/8080 for UI/API access |

Additional recommendations:

- Enforce firewall rules allowing only trusted hosts to reach the management server.
- Allocate persistent storage for PostgreSQL and server state (if not containerized).
- Configure InfluxDB 2.x if you plan to ingest metrics from day one.

---

## 2. Provision the Management Server

### 2.1 Clone the Repository

```bash
git clone https://github.com/mikeysoft/flotilla.git
cd flotilla
```

### 2.2 Configure Environment

Copy and adjust the sample environment file:

```bash
cp env.example .env
```

Set the following variables:

- `DATABASE_URL` – PostgreSQL connection string with SSL enabled where applicable.
- `JWT_SECRET` – Long random value (32+ bytes). Rotate per environment.
- `TLS_CERT_FILE` / `TLS_KEY_FILE` – Paths to your certificates when running natively.
- `INFLUXDB_*` – Enable if metrics storage is required.

### 2.3 Deploy with Docker Compose (Recommended)

```bash
docker compose -f deployments/docker-compose.yml up -d
```

This launches:

- Flotilla management server (`server` service)
- PostgreSQL (unless using an external DB)
- Optional InfluxDB and supporting services if enabled in the compose file

> **Tip:** Override defaults via `docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.prod.yml up -d` if you maintain an environment-specific overlay.

### 2.4 Native Binary Deployment (Optional)

1. Build or download the `server` binary (see release workflow).
2. Place the binary under `/usr/local/bin/flotilla-server`.
3. Create a service account and configuration file in `/etc/flotilla/server.env`.
4. Install the systemd unit from [`deployments/systemd/flotilla-server.service`](../deployments/systemd/flotilla-server.service).
5. Enable and start the service:

   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now flotilla-server
   ```

---

## 3. Initialize the Platform

1. **Run migrations**: The server auto-migrates on startup; monitor logs for confirmation.
2. **Create the first admin**:

   ```bash
   curl -X POST https://your-domain.example/api/v1/auth/setup \
     -H "Content-Type: application/json" \
     -d '{"username": "admin", "password": "ChangeMeNow!"}'
   ```

3. **Generate an agent API key** via UI or API:

   ```bash
   curl -X POST https://your-domain.example/api/v1/api-keys \
     -H "Authorization: Bearer <admin-access-token>" \
     -d '{"name": "prod-agent", "scopes": ["agents:connect"]}'
   ```

4. **Enable metrics** (optional but recommended) by configuring `INFLUXDB_*` environment variables and restarting the server.

---

## 4. Enroll Agents

Agents communicate outbound over WSS (`wss://server-domain/ws/agent`). Choose a deployment method per host.

### 4.1 Docker Container

```bash
docker run -d \
  --name flotilla-agent \
  --restart unless-stopped \
  -e FLOTILLA_AGENT_ID=$(hostname) \
  -e FLOTILLA_AGENT_NAME=$(hostname) \
  -e FLOTILLA_SERVER_URL=wss://your-domain.example/ws/agent \
  -e FLOTILLA_API_KEY=<api-key> \
  -v /var/run/docker.sock:/var/run/docker.sock \
  ghcr.io/mikeysoft/flotilla-agent:latest
```

Add TLS trust anchors via a bind mount (`/etc/ssl/certs/ca.crt`) when using a private CA.

### 4.2 Native Binary

1. Download the agent binary matching your architecture (`flotilla-agent_<version>_<os>_<arch>.tar.gz`).
2. Place the binary in `/usr/local/bin/flotilla-agent`.
3. Create `/etc/flotilla/agent.yaml` (see [`deployments/agent/env.example`](../deployments/agent/env.example)).
4. Install the systemd unit [`deployments/systemd/flotilla-agent.service`](../deployments/systemd/flotilla-agent.service) and start it.

### 4.3 Configuration Notes

- Set `SERVER_URL` to the publicly reachable WSS endpoint.
- Provide `API_KEY` generated from the server.
- Optional toggles: `METRICS_ENABLED`, `METRICS_COLLECTION_INTERVAL`, `LOG_LEVEL`.
- When running in containers, mount `/var/run/docker.sock` and any required host paths for metrics fallbacks.

---

## 5. Validate the Installation

1. **Web Console** – Browse to `https://your-domain.example` and log in with the admin account.
2. **Agent Visibility** – Confirm the agent appears as “online” under Hosts.
3. **Container Operations** – Trigger a start/stop action on a non-critical container to verify execution.
4. **Metrics** – Ensure charts populate if InfluxDB is configured.
5. **Audit Logs** – Review server logs for warnings or errors after initial enrollment.

---

## 6. Hardening Checklist

- Enforce TLS 1.3 with modern ciphers by placing Flotilla behind a reverse proxy (NGINX, Traefik).
- Rotate API keys regularly and revoke unused keys.
- Configure role-based access control (planned feature) or tightly scope credentials.
- Enable database backups and test restore procedures.
- Integrate with centralized logging and alerting (e.g., Loki, ELK, Prometheus).
- Restrict agent hosts from accepting inbound connections; only outbound WSS is required.

---

## 7. Troubleshooting

| Symptom | Action |
|---------|--------|
| Agent stuck reconnecting | Verify DNS/TLS configuration, ensure firewall allows outbound 443, and confirm API key validity. |
| Server cannot reach PostgreSQL | Re-check `DATABASE_URL`, security groups, and ensure migrations folder is accessible. |
| Metrics missing | Confirm `INFLUXDB_ENABLED=true`, check token/organization, and verify agent metrics flags. |
| UI fails to load assets | Inspect reverse proxy headers for `X-Forwarded-*` values; re-run `npm run build` if frontend bundle is absent. |

Need help? Open a GitHub discussion or use the security reporting channel for sensitive issues.

