# Native Service Registration

Flotilla components can run outside of Docker using native systemd units. This guide covers installing, configuring, and managing the management server and agent as Linux services.

---

## Directory Layout

All service manifests are stored in [`deployments/systemd/`](../deployments/systemd/):

- `flotilla-server.service` – Management server unit.
- `flotilla-agent.service` – Host agent unit.

Copy these files into `/etc/systemd/system/` (root permissions required) and adjust environment files referenced within.

---

## Management Server

### 1. Install Binary

```bash
sudo install -m 0755 ./bin/server /usr/local/bin/flotilla-server
```

Or extract the release archive:

```bash
sudo tar -xzf flotilla-server_<version>_linux_amd64.tar.gz -C /usr/local/bin flotilla-server
```

### 2. Environment Configuration

Create `/etc/flotilla/server.env`:

```
MODE=PROD
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
DATABASE_URL=postgres://flotilla:<password>@db.example.com:5432/flotilla?sslmode=require
JWT_SECRET=<strong-random-string>
TLS_ENABLED=true
TLS_CERT_FILE=/etc/flotilla/tls/server.crt
TLS_KEY_FILE=/etc/flotilla/tls/server.key
INFLUXDB_ENABLED=true
INFLUXDB_URL=https://influx.example.com
INFLUXDB_TOKEN=<token>
INFLUXDB_ORG=flotilla
INFLUXDB_BUCKET=metrics
```

Ensure the TLS certificate and key are accessible to the `flotilla` service account (create one if desired).

### 3. Install systemd Unit

```bash
sudo cp deployments/systemd/flotilla-server.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now flotilla-server
```

### 4. Health Checks

```bash
systemctl status flotilla-server
journalctl -u flotilla-server -f
curl -k https://localhost:8080/healthz
```

---

## Agent Service

### 1. Install Binary

```bash
sudo install -m 0755 ./bin/agent /usr/local/bin/flotilla-agent
```

### 2. Configuration

Create `/etc/flotilla/agent.yaml`:

```yaml
server:
  url: "wss://your-domain.example/ws/agent"
  api_key: "replace-with-api-key"
  reconnect_interval: 5s
  heartbeat_interval: 30s

agent:
  id: "host-$(hostname)"
  name: "production-host-01"
  docker_socket: "/var/run/docker.sock"

logging:
  level: "info"
  format: "json"

metrics:
  enabled: true
  collection_interval: "30s"
  collect_host_stats: true
  collect_network: true
```

See [`deployments/agent/env.example`](../deployments/agent/env.example) for additional knobs.

### 3. Install systemd Unit

```bash
sudo cp deployments/systemd/flotilla-agent.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now flotilla-agent
```

### 4. Permissions

- Grant the service account access to `/var/run/docker.sock`. For example, add it to the `docker` group.
- When collecting host metrics, ensure `/proc`, `/sys/fs/cgroup`, or other required paths are accessible (see agent docs).

### 5. Validation

```bash
systemctl status flotilla-agent
journalctl -u flotilla-agent -f
```

On the management UI, confirm the host appears online with heartbeat data.

---

## Upgrades & Maintenance

- Use `systemctl restart <service>` after deploying new binaries.
- Rotate API keys by updating `/etc/flotilla/agent.yaml` and restarting the agent.
- For zero-downtime server upgrades, place Flotilla behind a load balancer and roll nodes sequentially (future HA feature).
- Automate upgrades with configuration management tools (Ansible, Chef, Puppet) or package managers as the project evolves.

---

## Troubleshooting

| Issue | Resolution |
|-------|------------|
| Service fails on boot | Run `journalctl -xeu <service>`, validate environment file paths, confirm binary permissions. |
| Agent reports TLS errors | Confirm the server certificate chain is trusted and `SSL_CERT_FILE` (or system trust) is configured. |
| Docker socket permission denied | Add the service user to the `docker` group or adjust socket ACLs. |
| Frequent reconnects | Verify outbound connectivity to the server, check firewall policies, and ensure API key validity. |

For further assistance, consult [`docs/setup.md`](setup.md) or open a GitHub issue with diagnostic details.

