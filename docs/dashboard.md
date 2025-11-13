# Dashboard Overview

The dashboard provides an at-a-glance view of fleet health as soon as an operator signs in. It combines summary metrics with actionable tasks so issues can be triaged without drilling into individual hosts.

## Summary Metrics

The summary ribbon refreshes automatically every 30 seconds and reflects the latest data gathered by the background scanner.

| Metric | Description | Source |
| ------ | ----------- | ------ |
| Hosts Online / Total | Count of agents currently connected vs. registered hosts | PostgreSQL + WebSocket hub |
| Hosts Offline | Hosts that have not checked in recently (see task thresholds below) | PostgreSQL |
| Containers | Total containers reported across connected agents | Live agent scans |
| Stacks | Total compose stacks reported across connected agents | Live agent scans |

## Task Engine

System-generated tasks capture remediation work discovered by the background scanner. Manual tasks can be added from the UI for ad‑hoc follow up.

### System Tasks

| Task Type | Trigger | Severity Rules | Resolution |
| --------- | ------- | -------------- | ---------- |
| `host_offline` | Agent disconnected or heartbeat stale | Warning after 60 s, Critical after 5 min (`OFFLINE_CRITICAL_AFTER`) | Auto-resolves when the agent reconnects |
| `host_low_disk` | `disk_free / disk_total` below thresholds | Warning < 15 %; Critical < 5 % (`DISK_WARNING_PERCENT`, `DISK_CRITICAL_PERCENT`) | Auto-resolves once free space recovers |
| `host_low_memory` | Latest host metrics show low available memory | Warning < 15 %; Critical < 5 % (`MEMORY_WARNING_PERCENT`, `MEMORY_CRITICAL_PERCENT`) | Auto-resolves when memory headroom increases |
| `stack_unmanaged` | Stack reported without Flotilla management labels | Info | Resolved when stack is imported or removed |
| `stack_unhealthy` | Stack status `partial`, `stopped`, or `error` | Warning for `partial`/`stopped`, Critical for `error` | Auto-resolves when stack returns to `running` or disappears |

Thresholds can be tuned in `internal/server/dashboard/scanner.go`.

### Manual Tasks

Operators can add manual tasks from the dashboard. Each task supports:

- Title & description
- Severity, category, type tags
- Optional host or stack association
- Due and snooze timestamps

Manual tasks default to the `open` status and can be acknowledged, resolved, or dismissed directly from the task table.

## Notification Roadmap

Current release surfaces tasks in-app with toast confirmations for actions. The following enhancements are planned:

1. **Navigation badge** – highlight unseen high-severity tasks on the sidebar.
2. **Daily digest email** – optional summary of unresolved tasks.
3. **Webhooks** – push task lifecycle events to Slack/Ops tooling via signed payloads.
4. **Browser push notifications** – opt-in alerts for critical regressions while the app is open.

See `features/phase3_advanced.md` for the tracking items related to these notification channels.

