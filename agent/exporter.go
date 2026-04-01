package agent

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusExporter holds the Prometheus metrics that will be exposed.
// It provides methods to update these metrics, which are called by the Collector.
type PrometheusExporter struct {
	syncLagSeconds                prometheus.Gauge
	sequencingFailuresTotal       prometheus.Counter
	bftQuorumDropsTotal           prometheus.Counter
	validatorUp                   prometheus.Gauge
	livenessRewardsCollectedTotal prometheus.Counter
	livenessRewardsMissedTotal    prometheus.Counter
	agentInfo                     prometheus.Gauge
}

// NewPrometheusExporter creates and registers the Prometheus metrics with the default registry.
// It takes a namespace to prefix all metric names (e.g., "canton_validator").
func NewPrometheusExporter(namespace, version string) *PrometheusExporter {
	return &PrometheusExporter{
		syncLagSeconds: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "sync_lag_seconds",
			Help:      "The current synchronization lag of the validator node compared to the sequencer, in seconds.",
		}),
		sequencingFailuresTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "sequencing_failures_total",
			Help:      "Total number of sequencing failures detected since the agent started.",
		}),
		bftQuorumDropsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "bft_quorum_drops_total",
			Help:      "Total number of times the BFT quorum has dropped below the required threshold.",
		}),
		validatorUp: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "validator_up",
			Help:      "Indicates if the validator node is considered up (1) or down (0) based on health checks.",
		}),
		livenessRewardsCollectedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "liveness_rewards_collected_total",
			Help:      "Total number of successful liveness reward collections.",
		}),
		livenessRewardsMissedTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "liveness_rewards_missed_total",
			Help:      "Total number of missed liveness reward collections.",
		}),
		agentInfo: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "agent_info",
			Help:      "Information about the Canton monitoring agent.",
			ConstLabels: prometheus.Labels{
				"version": version,
			},
		}),
	}
}

// Start runs the HTTP server to expose the /metrics endpoint.
// This function blocks and should typically be run in a separate goroutine.
func (e *PrometheusExporter) Start(addr string) {
	log.Printf("Prometheus exporter starting on %s/metrics", addr)
	// Set the agent info gauge to 1 to ensure its labels are exposed
	e.agentInfo.Set(1)

	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start Prometheus exporter HTTP server: %v", err)
	}
}

// SetSyncLag updates the sync_lag_seconds metric.
func (e *PrometheusExporter) SetSyncLag(lag float64) {
	e.syncLagSeconds.Set(lag)
}

// IncSequencingFailures increments the sequencing_failures_total counter.
func (e *PrometheusExporter) IncSequencingFailures() {
	e.sequencingFailuresTotal.Inc()
}

// IncBFTQuorumDrops increments the bft_quorum_drops_total counter.
func (e *PrometheusExporter) IncBFTQuorumDrops() {
	e.bftQuorumDropsTotal.Inc()
}

// SetValidatorStatus updates the validator_up gauge.
// 1 for up, 0 for down.
func (e *PrometheusExporter) SetValidatorStatus(isUp bool) {
	if isUp {
		e.validatorUp.Set(1)
	} else {
		e.validatorUp.Set(0)
	}
}

// IncLivenessRewardsCollected increments the liveness_rewards_collected_total counter.
func (e *PrometheusExporter) IncLivenessRewardsCollected() {
	e.livenessRewardsCollectedTotal.Inc()
}

// IncLivenessRewardsMissed increments the liveness_rewards_missed_total counter.
func (e *PrometheusExporter) IncLivenessRewardsMissed() {
	e.livenessRewardsMissedTotal.Inc()
}