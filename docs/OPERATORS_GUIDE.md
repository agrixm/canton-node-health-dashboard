# Canton Node Health Dashboard: Operator's Guide

This guide provides step-by-step instructions for deploying and configuring the Canton Node Health Dashboard, an open-source monitoring stack for Canton validator and sequencer operators.

The stack uses Prometheus for metrics collection, Grafana for visualization, and an on-ledger Daml smart contract for end-to-end liveness checks.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Prerequisites](#prerequisites)
- [Deployment Steps](#deployment-steps)
  - [Step 1: Clone the Repository](#step-1-clone-the-repository)
  - [Step 2: Configure Environment Variables](#step-2-configure-environment-variables)
  - [Step 3: Deploy the Daml Health Check Contract](#step-3-deploy-the-daml-health-check-contract)
  - [Step 4: Start the Monitoring Stack](#step-4-start-the-monitoring-stack)
- [Accessing the Dashboards](#accessing-the-dashboards)
- [Understanding Key Metrics](#understanding-key-metrics)
- [Alerting](#alerting)
  - [Configuring Notifications](#configuring-notifications)
  - [Customizing Alert Rules](#customizing-alert-rules)
- [Troubleshooting](#troubleshooting)
- [Security Considerations](#security-considerations)

## Architecture Overview

The monitoring stack consists of several components working together:

1.  **Canton Node:** Your validator or sequencer node, which exposes metrics via its admin API.
2.  **Prometheus:** A time-series database that periodically "scrapes" (pulls) metrics from the Canton node's `/metrics` endpoint.
3.  **Grafana:** A visualization tool that queries Prometheus and displays the metrics in pre-built dashboards.
4.  **Alertmanager:** Handles alerts sent by Prometheus and routes them to configured notification channels like Slack or PagerDuty.
5.  **Daml Health Check Agent:** A small Go application that periodically exercises an on-ledger Daml contract to verify the liveness and responsiveness of the entire Canton stack, from participant node to sequencer.
6.  **Daml Health Check Contract:** A simple Daml contract deployed on your participant node. Its successful execution is a strong indicator of full network health.

All components are containerized and managed via a single `docker-compose.yml` file for easy deployment.

## Prerequisites

Before you begin, ensure you have the following:

-   **Docker and Docker Compose:** Installed on a machine with network access to your Canton node(s).
-   **Canton Node Admin API Access:** The URL and a valid authentication token for your validator or sequencer's admin API.
-   **Canton Participant Node:** A running participant node connected to the network. You will need:
    -   The participant's JSON API URL (e.g., `http://localhost:7575`).
    -   A JWT token for a party on that participant. This party will be the operator of the health check contract.
-   **Git:** To clone the repository.

## Deployment Steps

### Step 1: Clone the Repository

Clone this repository to the machine where you will run the monitoring stack.

```bash
git clone https://github.com/digital-asset/canton-node-health-dashboard.git
cd canton-node-health-dashboard
```

### Step 2: Configure Environment Variables

The entire stack is configured using a `.env` file. A template is provided; copy it to create your configuration file.

```bash
cp .env.template .env
```

Now, edit the `.env` file with your specific details.

```dotenv
# .env

# --- Canton Node Metrics ---
# The public-facing URL of your Canton node's admin API.
# This is the endpoint Prometheus will scrape.
# Example: http://192.168.1.100:5012
CANTON_NODE_METRICS_URL="http://<your-canton-node-ip>:<admin-port>"

# Bearer token for authenticating with the Canton node's admin API.
CANTON_NODE_ADMIN_TOKEN="your-admin-api-token"

# --- Daml On-Ledger Health Check ---
# URL for your Canton participant's JSON API.
CANTON_PARTICIPANT_JSON_API_URL="http://<your-participant-ip>:7575"

# The Party ID of the operator running the health check.
# This party must be hosted on the participant above.
HEALTH_CHECK_OPERATOR_PARTY_ID="OperatorParty::1220....."

# A long-lived JWT for the operator party.
# Generate this from your participant's user management.
HEALTH_CHECK_OPERATOR_JWT="ey..."

# The Party ID representing the validator node being monitored.
# This can be the same as the operator or a different party.
HEALTH_CHECK_VALIDATOR_PARTY_ID="ValidatorNode::1220....."

# --- Alerting (Optional) ---
# Uncomment and configure if you want notifications.
# ALERTMANAGER_SLACK_API_URL="https://hooks.slack.com/services/..."
# ALERTMANAGER_PAGERDUTY_ROUTING_KEY="your-pagerduty-key"
```

**IMPORTANT:** Ensure the machine running Docker can reach the `CANTON_NODE_METRICS_URL` and `CANTON_PARTICIPANT_JSON_API_URL` you provide.

### Step 3: Deploy the Daml Health Check Contract

The monitoring agent requires a `HealthCheckAgreement` contract to be active on the ledger. You only need to do this once.

You can create this contract by sending a `create` command to your participant's JSON API.

**Request:**

`POST /v1/create`
`Authorization: Bearer <your-health-check-operator-jwt>`
`Content-Type: application/json`

**Body:**

Replace `<OPERATOR_PARTY_ID>` and `<VALIDATOR_PARTY_ID>` with the values from your `.env` file.

```json
{
  "templateId": "HealthCheck:HealthCheckAgreement",
  "payload": {
    "operator": "<OPERATOR_PARTY_ID>",
    "validator": "<VALIDATOR_PARTY_ID>",
    "lastCheckIn": "1970-01-01T00:00:00Z"
  }
}
```

A successful response (`200 OK`) indicates the contract was created. The health check agent will automatically find this contract by its template type and stakeholders.

### Step 4: Start the Monitoring Stack

With the configuration in place, you can start all services using Docker Compose.

```bash
docker-compose up -d
```

This command will pull the necessary Docker images and start Prometheus, Grafana, Alertmanager, and the health check agent in the background.

To check the status of the containers:

```bash
docker-compose ps
```

To view logs for a specific service (e.g., the health check agent):

```bash
docker-compose logs -f health-check-agent
```

## Accessing the Dashboards

Once the stack is running, you can access the Grafana UI.

-   **URL:** `http://<your-docker-host-ip>:3000`
-   **Default Username:** `admin`
-   **Default Password:** `admin`

You will be prompted to change the password on your first login.

The Canton Validator and Sequencer dashboards are pre-installed and available in the "Dashboards" section.

## Understanding Key Metrics

The dashboards provide a high-level overview of node health. Key panels include:

-   **Validator Uptime:** Tracks the `up` metric from Prometheus. If this drops to 0, the node is unreachable.
-   **Sequencer Sync Lag:** The time difference between the sequencer's clock and the latest timestamp of a sequenced event. High lag can indicate performance issues or network problems.
-   **BFT Quorum Status:** (For BFT domains) Monitors if the validator is maintaining quorum with its peers.
-   **On-Ledger Liveness:** Visualizes the `lastCheckIn` time from the `HealthCheckAgreement` contract. A stale timestamp indicates a potential failure in the full transaction lifecycle.
-   **Sequencing Failures:** Tracks the rate of failed sequencing requests, which could point to misconfiguration or overload.
-   **gRPC/API Latency:** Monitors the performance of the node's public and admin APIs.

## Alerting

Prometheus is pre-configured with a set of critical alert rules. When an alert fires, it is passed to Alertmanager, which then dispatches notifications.

### Configuring Notifications

To receive alerts, you must configure a notification receiver in `alertmanager/config.yml` and provide the necessary credentials (e.g., a Slack webhook URL) in your `.env` file.

The default `config.yml` is set up to use environment variables for Slack and PagerDuty. Simply uncomment the relevant lines in your `.env` file and restart the stack:

```bash
# In .env
# ALERTMANAGER_SLACK_API_URL="https://hooks.slack.com/services/..."

# Restart alertmanager to apply the change
docker-compose restart alertmanager
```

### Customizing Alert Rules

Alert rules are defined in `prometheus/rules/canton.rules.yml`. You can modify the thresholds (e.g., change the sync lag alert from 5 minutes to 10 minutes) or add new rules based on any metric available in Prometheus.

After modifying the `.yml` file, reload the Prometheus configuration:

```bash
docker-compose kill -s SIGHUP prometheus
```

## Troubleshooting

-   **No data in Grafana dashboards:**
    1.  Check the Prometheus UI at `http://<your-docker-host-ip>:9090`.
    2.  Go to `Status -> Targets`.
    3.  Verify that the `canton-node` target is `UP`. If it's `DOWN`, check for firewall issues, incorrect IP/port in `.env`, or an invalid admin token.
    4.  Check the logs of the `health-check-agent` container (`docker-compose logs health-check-agent`) for any JSON API authentication or connection errors.

-   **Containers fail to start:**
    1.  Run `docker-compose up` (without the `-d`) to view logs in the foreground.
    2.  Look for error messages related to invalid configuration in `.env` or port conflicts on the host machine.

## Security Considerations

-   **Network Exposure:** By default, the Grafana (`3000`) and Prometheus (`9090`) ports are exposed. In a production environment, you should restrict access to these ports using a firewall, allowing access only from trusted IP addresses.
-   **Default Passwords:** Change the default Grafana `admin` password immediately upon first login.
-   **Secrets:** The `.env` file contains sensitive information (API tokens, JWTs). Ensure this file has restrictive file permissions (`chmod 600 .env`) and is not committed to version control. The provided `.gitignore` file already excludes it.