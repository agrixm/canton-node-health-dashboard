# Metrics Reference

## Validator Metrics
| Metric | Type | Description |
|--------|------|-------------|
| `canton_validator_sync_lag_seconds` | Gauge | Seconds behind global sequencer |
| `canton_validator_quorum_health` | Gauge | 1=healthy, 0=quorum loss |
| `canton_validator_liveness_reward_last_epoch` | Gauge | CC earned last epoch |
| `canton_validator_liveness_reward_missed_total` | Counter | Missed reward epochs |

## Sequencer Metrics
| Metric | Type | Description |
|--------|------|-------------|
| `canton_sequencer_tps` | Gauge | Transactions per second |
| `canton_sequencer_latency_p99_ms` | Gauge | p99 latency in ms |
| `canton_sequencer_queue_depth` | Gauge | Pending tx queue depth |

## Alert Thresholds (defaults)
| Alert | Threshold | Severity |
|-------|-----------|----------|
| Sync lag | >60s | Warning |
| Sync lag | >300s | Critical |
| Quorum health | =0 | Critical |
