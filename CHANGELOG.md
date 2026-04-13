# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- New metric `canton_bft_quorum_active_members` to track the number of active members in the BFT consensus group.
- Support for configuring alert thresholds via environment variables.

### Changed
- Improved error handling for Canton Admin API connection failures, with exponential backoff on retries.

## [1.0.0] - 2024-05-20

This is the first stable release, recommended for all production environments.

### Added
- **Liveness Rewards Monitoring**: A key new metric, `canton_liveness_rewards_collected_total`, tracks the cumulative liveness rewards collected from the sequencer, providing crucial visibility into operator profitability.
- **Alerting Support**: The agent can now be configured to send alerts via Webhooks (e.g., to Slack, PagerDuty, or OpsGenie) for critical events like validator downtime or sync lag exceeding a configured threshold. See `notifier.go`.
- **Official Docker Image**: Published `ghcr.io/canton-community/node-health-dashboard:1.0.0`.
- Finalized documentation in `docs/OPERATORS_GUIDE.md` and `docs/METRICS_REFERENCE.md`.

### Changed
- **BREAKING**: Renamed metric `canton_sync_delay` to `canton_sync_lag_seconds` for clarity and consistency with Prometheus naming conventions. Grafana dashboards and alerts must be updated.
- Upgraded the bundled Grafana dashboard to Grafana v10.4, using the new time series panels.
- Refined the `docker-compose.yml` file for production readiness, adding container health checks and robust restart policies.

### Fixed
- Resolved an issue where the agent would crash if the Canton node was restarted while the agent was running. The agent now gracefully handles temporary connection loss.

## [0.2.0] - 2024-04-22

### Added
- **Sequencer Health Metrics**: Added new metrics for monitoring sequencer health, including `canton_sequencer_events_processed_total` and `canton_sequencer_head_clean`.
- **Sync Lag Metric**: Introduced the `canton_sync_delay` metric to measure the time difference between the validator's local head state and the sequencer's head. This is a critical indicator of a node falling behind the network.
- **Enhanced Grafana Dashboard**: The dashboard was significantly improved with new panels to visualize sync lag, sequencer activity, and transaction throughput.

### Fixed
- Corrected a race condition in the metrics collector that could cause stale data to be reported on agent startup.

## [0.1.0] - 2024-03-15

### Added
- Initial release of the Canton Node Health Dashboard agent.
- Basic metrics collection via the Canton Admin API: `canton_up` and `canton_participant_active`.
- Prometheus exporter endpoint exposed on port `9091` at the `/metrics` path.
- A simple Grafana dashboard for visualizing basic node uptime and status.
- A `docker-compose.yml` for easy local setup of the complete stack (agent, Prometheus, Grafana).