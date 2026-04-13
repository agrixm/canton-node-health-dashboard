# Canton Node Health Metrics Reference

This document provides a comprehensive reference for all Prometheus metrics exported by the Canton Node Health Agent. These metrics are designed to give operators deep insight into the performance, health, and status of their Canton validator and sequencer nodes.

## Table of Contents

- [Agent Metrics](#agent-metrics)
- [General Node Metrics](#general-node-metrics)
- [Validator / Participant Metrics](#validator--participant-metrics)
- [Sequencer Metrics](#sequencer-metrics)
- [Domain-Specific Metrics](#domain-specific-metrics)

---

## Agent Metrics

Metrics related to the health and operation of the monitoring agent itself.

### `canton_agent_scrapes_total`

-   **Type:** Counter
-   **Description:** The total number of scrapes performed by the agent against the Canton node's gRPC API. This is useful for verifying that the agent is actively collecting data.
-   **Labels:** `job`, `instance`
-   **Usage & Alerting:** A flatlining `rate(canton_agent_scrapes_total[5m])` indicates the agent might be stuck or has crashed.

### `canton_agent_scrape_errors_total`

-   **Type:** Counter
-   **Description:** The total number of errors encountered while scraping the Canton node. This can indicate configuration issues (wrong host/port), network problems, or an unresponsive Canton node.
-   **Labels:** `job`, `instance`, `error_type` (e.g., `connection_failed`, `permission_denied`)
-   **Usage & Alerting:** Alert if `rate(canton_agent_scrape_errors_total[5m]) > 0`. A sustained rate of errors requires immediate investigation.

---

## General Node Metrics

High-level metrics for the entire Canton process.

### `canton_up`

-   **Type:** Gauge
-   **Description:** A simple health check metric. `1` if the Canton node's gRPC health endpoint is reachable and reports a `SERVING` status, `0` otherwise.
-   **Labels:** `job`, `instance`
-   **Usage & Alerting:** This is the most fundamental alert. Alert if `canton_up == 0` for more than 1 minute.

### `canton_process_uptime_seconds`

-   **Type:** Counter
-   **Description:** The uptime of the Canton Java process in seconds, as reported by the node.
-   **Labels:** `job`, `instance`
-   **Usage & Alerting:** Useful for tracking restarts. An alert on `changes(canton_process_uptime_seconds[5m]) < 0` can detect unexpected node restarts.

---

## Validator / Participant Metrics

Metrics specific to the validator's participant node component.

### `canton_validator_sync_lag_seconds`

-   **Type:** Gauge
-   **Description:** The time difference in seconds between the sequencer's latest timestamp and the validator participant's head clean timestamp. This is a critical indicator of how far behind the validator is from the network's state.
-   **Labels:** `domain_id`
-   **Usage & Alerting:** A small, stable lag is normal. A large or continuously increasing lag indicates that the validator is struggling to process events from the sequencer. Alert if `canton_validator_sync_lag_seconds > 300` (5 minutes).

### `canton_validator_active_contracts`

-   **Type:** Gauge
-   **Description:** The total number of active contracts in the participant's Active Contract Set (ACS).
-   **Labels:** `participant_id`
-   **Usage & Alerting:** Provides a view of the participant's state size. Unexpectedly rapid increases or decreases could signal abnormal application activity or pruning issues.

### `canton_validator_in_flight_submissions`

-   **Type:** Gauge
-   **Description:** The number of command submissions that are currently being processed by the participant but have not yet been finalized.
-   **Labels:** `participant_id`
-   **Usage & Alerting:** A persistently high number can indicate a processing bottleneck, either locally or at the sequencer. Alert if `canton_validator_in_flight_submissions > 50` for a sustained period.

### `canton_validator_liveness_rewards_collected_total`

-   **Type:** Counter
-   **Description:** A counter that increments each time the validator successfully collects a liveness reward from the network's Rewards Manager.
-   **Labels:** `participant_id`, `domain_id`
-   **Usage & Alerting:** Use `rate(canton_validator_liveness_rewards_collected_total[1h])` to monitor the reward collection frequency. If this rate drops to zero for a period longer than the expected reward interval, it indicates a problem with the validator's liveness attestation or reward collection process.

### `canton_validator_liveness_last_collection_timestamp_seconds`

-   **Type:** Gauge
-   **Description:** The Unix timestamp of the last successful liveness reward collection.
-   **Labels:** `participant_id`, `domain_id`
-   **Usage & Alerting:** Alert if `time() - canton_validator_liveness_last_collection_timestamp_seconds` is greater than the expected reward interval (e.g., 24 hours). This is a direct way to check if rewards are being missed.

---

## Sequencer Metrics

Metrics for operators running a sequencer node.

### `canton_sequencer_head_timestamp_seconds`

-   **Type:** Gauge
-   **Description:** The Unix timestamp of the most recent event processed by the sequencer. This represents the "head" of the domain's sequenced log.
-   **Labels:** `domain_id`
-   **Usage & Alerting:** This value should constantly increase. A flatlining value indicates a stalled sequencer. Alert if `increase(canton_sequencer_head_timestamp_seconds[5m]) == 0`.

### `canton_sequencer_bft_quorum_active`

-   **Type:** Gauge
-   **Description:** Indicates the health of the BFT consensus quorum. `1` if a quorum of sequencer replicas is active and participating, `0` if the quorum is lost.
-   **Labels:** `domain_id`
-   **Usage & Alerting:** A critical alert. Alert immediately if `canton_sequencer_bft_quorum_active == 0`. A loss of quorum means the sequencer cannot process new transactions for the domain.

### `canton_sequencer_unhealthy_members`

-   **Type:** Gauge
-   **Description:** The number of sequencer members (replicas) that are currently considered unhealthy by the BFT protocol.
-   **Labels:** `domain_id`
-   **Usage & Alerting:** Alert if `canton_sequencer_unhealthy_members > 0`. Even one unhealthy member can put the quorum at risk. If the value exceeds `(N-1)/3` where N is the total number of replicas, the quorum will be lost.

### `canton_sequencer_events_processed_total`

-   **Type:** Counter
-   **Description:** The total number of events (transactions, heartbeats, etc.) sequenced since the process started.
-   **Labels:** `domain_id`
-   **Usage & Alerting:** The rate of this counter (`rate(canton_sequencer_events_processed_total[5m])`) indicates the current throughput of the domain. Sudden drops to zero can indicate a stall.

---

## Domain-Specific Metrics

Metrics related to a specific domain the node is connected to, applicable to both validators and sequencers.

### `canton_domain_connected`

-   **Type:** Gauge
-   **Description:** `1` if the participant is successfully connected to the specified domain's sequencer, `0` otherwise.
-   **Labels:** `participant_id`, `domain_id`
-   **Usage & Alerting:** An essential connectivity check. Alert if `canton_domain_connected == 0` for more than a few minutes, as this indicates a network partition or configuration issue.

### `canton_domain_sequencing_failures_total`

-   **Type:** Counter
-   **Description:** The total number of commands that failed during the sequencing phase for a given domain. This can be due to issues like invalid signatures, insufficient traffic balance, or sequencer-side errors.
-   **Labels:** `participant_id`, `domain_id`, `reason`
-   **Usage & Alerting:** A sudden spike in `rate(canton_domain_sequencing_failures_total[5m])` requires investigation. The `reason` label can help diagnose the root cause (e.g., `InsufficientTraffic`, `AuthenticationFailed`).

### `canton_domain_traffic_balance`

-   **Type:** Gauge
-   **Description:** The current traffic balance for the participant on the specified domain. This balance is consumed to pay for transaction data traffic.
-   **Labels:** `participant_id`, `domain_id`
-   **Usage & Alerting:** This is a critical operational metric. Alert if `canton_domain_traffic_balance` falls below a predefined threshold (e.g., `10.00`) to ensure there is enough time to top it up before it is depleted, which would halt all transaction submissions.