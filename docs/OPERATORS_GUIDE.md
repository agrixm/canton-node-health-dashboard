# Canton Node Health Dashboard — Operators Guide

## Prerequisites
- Docker + Compose v2
- Canton node running with JSON API on port 7575

## Quick Start
```bash
git clone https://github.com/agrixm/canton-node-health-dashboard
cd canton-node-health-dashboard
cp .env.example .env   # edit CANTON_HOST, CANTON_JWT
docker compose up -d
```

Open Grafana at `http://localhost:3000` (admin / canton).

## Dashboards
- **Canton Validator Overview** — sync lag, quorum health, liveness rewards
- **Canton Sequencer Throughput** — TPS, latency, batch sizes

## Alerting
Configure Slack/PagerDuty in Grafana → Alerting → Contact Points.
Alerts fire for: quorum loss, sync lag > 60s, agent down > 2 min.
