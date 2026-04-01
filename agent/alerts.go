package agent

// AlertRule represents a single Prometheus alerting rule.
// This structure can be marshaled into the YAML format required by Prometheus.
type AlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// AlertRuleGroup represents a group of alerting rules for Prometheus.
type AlertRuleGroup struct {
	Name  string      `yaml:"name"`
	Rules []AlertRule `yaml:"rules"`
}

// AlertGroups is the top-level structure for a Prometheus rules file.
type AlertGroups struct {
	Groups []AlertRuleGroup `yaml:"groups"`
}

// GetCantonAlertRules returns a predefined set of alerting rules for a Canton validator node.
// These rules can be written to a file and loaded by Prometheus.
func GetCantonAlertRules() AlertGroups {
	return AlertGroups{
		Groups: []AlertRuleGroup{
			{
				Name: "CantonNodeCriticalAlerts",
				Rules: []AlertRule{
					{
						Alert: "CantonQuorumDrop",
						Expr:  `canton_bft_quorum_active_validators / canton_bft_quorum_total_validators < 0.67`,
						For:   "5m",
						Labels: map[string]string{
							"severity": "critical",
						},
						Annotations: map[string]string{
							"summary":     "Canton BFT quorum has dropped below the fault tolerance threshold (instance {{ $labels.instance }})",
							"description": "The proportion of active validators in the BFT quorum has fallen to {{ $value | printf \"%.2f\" }}. This is below the critical 2/3 fault tolerance threshold. The network is at risk of halting or experiencing sequencing failures. Immediate investigation of validator health across the network is required.",
						},
					},
					{
						Alert: "CantonHighSequencingFailureRate",
						Expr:  `rate(canton_sequencing_failures_total[5m]) > 0`,
						For:   "1m",
						Labels: map[string]string{
							"severity": "critical",
						},
						Annotations: map[string]string{
							"summary":     "Canton node is experiencing sequencing failures (instance {{ $labels.instance }})",
							"description": "The validator node at {{ $labels.instance }} is reporting an increasing rate of sequencing failures. This can be caused by network partitions, BFT consensus issues, or misconfigurations. Check node logs for details on the sequencing errors.",
						},
					},
					{
						Alert: "CantonNodeDown",
						Expr:  `up{job="canton-validator"} == 0`,
						For:   "5m",
						Labels: map[string]string{
							"severity": "critical",
						},
						Annotations: map[string]string{
							"summary":     "Canton node is down (instance {{ $labels.instance }})",
							"description": "The Canton node monitoring agent at {{ $labels.instance }} has been unreachable for over 5 minutes. The node may be offline or the agent has crashed.",
						},
					},
				},
			},
			{
				Name: "CantonNodeWarningAlerts",
				Rules: []AlertRule{
					{
						Alert: "CantonLivenessRewardMiss",
						Expr:  `time() - canton_liveness_reward_last_collection_timestamp_seconds > 7200`, // 2 hours
						For:   "15m",
						Labels: map[string]string{
							"severity": "warning",
						},
						Annotations: map[string]string{
							"summary":     "Canton validator missed liveness reward collection (instance {{ $labels.instance }})",
							"description": "The validator at {{ $labels.instance }} has not collected its liveness reward in the last 2 hours. This could indicate a problem with the node's connectivity, configuration, or wallet funds. Please investigate the validator's logs and ensure it is participating correctly in the network.",
						},
					},
					{
						Alert: "CantonHighSyncLag",
						Expr:  `canton_sync_lag_seconds > 300`, // 5 minutes
						For:   "10m",
						Labels: map[string]string{
							"severity": "warning",
						},
						Annotations: map[string]string{
							"summary":     "Canton node has high sync lag (instance {{ $labels.instance }})",
							"description": "The validator node at {{ $labels.instance }} is lagging {{ $value | humanizeDuration }} behind the head of the sequencer chain. This could be due to network latency, high load, or storage performance issues. If the lag continues to grow, the node may fall out of the active set.",
						},
					},
					{
						Alert: "CantonLowLivenessRewardCollectionRate",
						Expr:  `(rate(canton_liveness_reward_collected_total[24h]) * 3600) < 0.9`, // Less than 90% of expected hourly collections over 24h
						For:   "4h",
						Labels: map[string]string{
							"severity": "warning",
						},
						Annotations: map[string]string{
							"summary":     "Canton node has a low liveness reward collection rate (instance {{ $labels.instance }})",
							"description": "The 24-hour average liveness reward collection rate for validator {{ $labels.instance }} is below 90%. This indicates persistent issues with reward collection which could impact operator revenue and signal underlying node health problems.",
						},
					},
				},
			},
		},
	}
}