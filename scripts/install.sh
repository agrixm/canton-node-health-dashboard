#!/usr/bin/env bash
set -euo pipefail

REPO="https://github.com/agrixm/canton-node-health-dashboard"
DIR="${INSTALL_DIR:-$HOME/.canton-monitor}"

command -v docker &>/dev/null || { echo "docker required"; exit 1; }

if [[ -d "$DIR" ]]; then
  git -C "$DIR" pull --ff-only
else
  git clone "$REPO" "$DIR"
fi

[[ -f "$DIR/.env" ]] || cp "$DIR/.env.example" "$DIR/.env"

docker compose -f "$DIR/docker/docker-compose.yml" up -d

echo ""
echo "✅ Canton Node Health Dashboard running"
echo "   Grafana   → http://localhost:3000  (admin / canton)"
echo "   Prometheus → http://localhost:9090"
