// agent/collector.go
package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// CantonAdminAPIClient is a placeholder for a real gRPC client that connects to
// the Canton node's admin API. In a production system, this struct would hold
// the gRPC connection and methods would make actual API calls.
type CantonAdminAPIClient struct {
	target string
	// In a real implementation:
	// conn *grpc.ClientConn
	// client pb.AdminServiceClient
}

// CantonMetrics represents the raw data fetched from the Canton node.
type CantonMetrics struct {
	IsUp                   bool
	SyncLag                time.Duration
	NewSequencingFailures  int64
	BFTQuorumActiveMembers int
	BFTQuorumTotalMembers  int
	NewLivenessRewards     int64
}

// FetchMetrics simulates fetching data from the Canton admin API.
// This function would be replaced with actual gRPC calls to endpoints like
// `HealthService.Status`, `SequencerAdminService.Status`, etc.
func (c *CantonAdminAPIClient) FetchMetrics() (*CantonMetrics, error) {
	// Simulate API call latency.
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

	// Simulate a 5% chance of the API being unreachable.
	if rand.Intn(100) < 5 {
		// Report the node as down, other metrics are irrelevant.
		return &CantonMetrics{IsUp: false}, nil
	}

	// Simulate a 4 out of 5 BFT quorum.
	activeMembers := 4
	if rand.Intn(10) < 2 { // 20% chance of a member dropping
		activeMembers = 3
	}

	// Simulate metric values for a healthy node.
	return &CantonMetrics{
		IsUp:                   true,
		SyncLag:                time.Duration(rand.Int63n(1500)) * time.Millisecond,
		NewSequencingFailures:  int64(rand.Intn(3)), // 0, 1, or 2 new failures since last scrape
		BFTQuorumActiveMembers: activeMembers,
		BFTQuorumTotalMembers:  5,
		NewLivenessRewards:     int64(rand.Intn(10)), // 0-9 new rewards collected since last scrape
	}, nil
}

// Collector implements the prometheus.Collector interface and is responsible
// for scraping metrics from a Canton node.
type Collector struct {
	client *CantonAdminAPIClient

	// Prometheus metric descriptions.
	upDesc                 *prometheus.Desc
	syncLagDesc            *prometheus.Desc
	sequencingFailuresDesc *prometheus.Desc
	quorumActiveDesc       *prometheus.Desc
	quorumConfiguredDesc   *prometheus.Desc
	livenessRewardsDesc    *prometheus.Desc

	// Internal state for cumulative counters.
	totalSequencingFailures int64
	totalLivenessRewards    int64
}

// NewCollector creates a new Canton metrics collector.
// It initializes the metric descriptions and the API client.
func NewCollector(cantonAdminAPIAddress string) *Collector {
	const namespace = "canton"
	const subsystem = "validator"

	return &Collector{
		client: &CantonAdminAPIClient{target: cantonAdminAPIAddress},
		// In a real agent, these counters might be loaded from a persisted state
		// on startup to survive restarts. For simplicity, we start at zero.
		totalSequencingFailures: 0,
		totalLivenessRewards:    0,

		upDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Indicates if the Canton validator node is running and reachable by the agent (1 for up, 0 for down).",
			nil, nil,
		),
		syncLagDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "sync_lag_seconds"),
			"Sync lag between the validator's sequencer client and the sequencer head.",
			nil, nil,
		),
		sequencingFailuresDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "sequencing_failures_total"),
			"Total number of sequencing failures encountered by the validator.",
			nil, nil,
		),
		quorumActiveDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bft", "quorum_active_members"),
			"Number of currently active (healthy) members in the BFT consensus quorum.",
			nil, nil,
		),
		quorumConfiguredDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "bft", "quorum_configured_members"),
			"Total number of configured members in the BFT consensus quorum.",
			nil, nil,
		),
		livenessRewardsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "liveness_rewards_collected_total"),
			"Total number of liveness rewards collected by the validator, if applicable.",
			nil, nil,
		),
	}
}

// Describe sends the static descriptions of all metrics collected by this
// Collector to the provided channel. It is part of the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.upDesc
	ch <- c.syncLagDesc
	ch <- c.sequencingFailuresDesc
	ch <- c.quorumActiveDesc
	ch <- c.quorumConfiguredDesc
	ch <- c.livenessRewardsDesc
}

// Collect is called by the Prometheus registry whenever a scrape is performed.
// It fetches the latest data from the Canton node and sends the corresponding
// metrics to the provided channel. It is part of the prometheus.Collector interface.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	metrics, err := c.client.FetchMetrics()
	if err != nil {
		log.Printf("Error fetching metrics from Canton node at %s: %v", c.client.target, err)
		// Report the node as down and return.
		ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, 0)
		return
	}

	var isUp float64 = 0
	if metrics.IsUp {
		isUp = 1
	}

	ch <- prometheus.MustNewConstMetric(c.upDesc, prometheus.GaugeValue, isUp)

	// Only report the other metrics if the node is confirmed to be up.
	if metrics.IsUp {
		// Update cumulative counters with the new values from the scrape.
		c.totalSequencingFailures += metrics.NewSequencingFailures
		c.totalLivenessRewards += metrics.NewLivenessRewards

		// Expose gauge metrics for values that can go up and down.
		ch <- prometheus.MustNewConstMetric(c.syncLagDesc, prometheus.GaugeValue, metrics.SyncLag.Seconds())
		ch <- prometheus.MustNewConstMetric(c.quorumActiveDesc, prometheus.GaugeValue, float64(metrics.BFTQuorumActiveMembers))
		ch <- prometheus.MustNewConstMetric(c.quorumConfiguredDesc, prometheus.GaugeValue, float64(metrics.BFTQuorumTotalMembers))

		// Expose counter metrics for values that only increase.
		ch <- prometheus.MustNewConstMetric(c.sequencingFailuresDesc, prometheus.CounterValue, float64(c.totalSequencingFailures))
		ch <- prometheus.MustNewConstMetric(c.livenessRewardsDesc, prometheus.CounterValue, float64(c.totalLivenessRewards))
	}
}