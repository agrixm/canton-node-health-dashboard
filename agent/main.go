package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds the application configuration, loaded from environment variables.
type Config struct {
	CantonApiURL       string
	CantonApiToken     string
	OperatorParty      string
	LivenessTemplateID string
	PollInterval       time.Duration
	ListenAddr         string
}

// CantonClient is a simple client for the Canton JSON API.
type CantonClient struct {
	httpClient *http.Client
	config     *Config
}

// QueryResponse represents the structure of a successful response from the /v1/query endpoint.
type QueryResponse struct {
	Result []Contract `json:"result"`
	Status int        `json:"status"`
}

// Contract represents a single active contract in the QueryResponse.
type Contract struct {
	ContractID string    `json:"contractId"`
	TemplateID string    `json:"templateId"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Global Prometheus metrics.
var (
	cantonAPIUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "canton_api_up",
		Help: "Indicates if the Canton JSON API is reachable (1 for up, 0 for down).",
	})
	ledgerTimeLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "canton_ledger_time_lag_seconds",
		Help: "The difference between wall-clock time and the latest observed ledger effective time.",
	})
	unclaimedLivenessRewards = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "canton_liveness_rewards_unclaimed_total",
		Help: "Total number of unclaimed liveness reward contracts for the operator party.",
	})
	pollErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "canton_agent_poll_errors_total",
		Help: "Total number of errors encountered while polling the Canton API.",
	})
)

func main() {
	log.Println("Starting Canton Node Health Dashboard Agent...")

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	client := &CantonClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		config:     config,
	}

	// Start polling in a separate goroutine.
	go pollMetrics(client)

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Metrics server listening on %s", config.ListenAddr)
	if err := http.ListenAndServe(config.ListenAddr, nil); err != nil {
		log.Fatalf("Failed to start metrics server: %v", err)
	}
}

// loadConfig reads environment variables and returns a Config struct.
func loadConfig() (*Config, error) {
	config := &Config{
		CantonApiURL:       getEnv("CANTON_API_URL", "http://localhost:7575"),
		CantonApiToken:     getEnv("CANTON_API_TOKEN", ""),
		OperatorParty:      getEnv("CANTON_OPERATOR_PARTY", ""),
		LivenessTemplateID: getEnv("LIVENESS_TEMPLATE_ID", ""),
		ListenAddr:         getEnv("LISTEN_ADDR", ":9091"),
	}

	if config.CantonApiToken == "" {
		return nil, fmt.Errorf("CANTON_API_TOKEN environment variable must be set")
	}
	if config.OperatorParty == "" {
		return nil, fmt.Errorf("CANTON_OPERATOR_PARTY environment variable must be set")
	}
	if config.LivenessTemplateID == "" {
		log.Println("WARNING: LIVENESS_TEMPLATE_ID not set. Unclaimed rewards and ledger lag metrics will not be collected.")
	}

	pollIntervalStr := getEnv("POLL_INTERVAL", "15s")
	pollInterval, err := time.ParseDuration(pollIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL duration: %w", err)
	}
	config.PollInterval = pollInterval

	return config, nil
}

// getEnv is a helper to read an environment variable or return a default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// pollMetrics starts a loop that periodically fetches metrics from the Canton node.
func pollMetrics(client *CantonClient) {
	ticker := time.NewTicker(client.config.PollInterval)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		log.Println("Polling Canton API for metrics...")

		contracts, err := client.queryContracts(client.config.LivenessTemplateID)
		if err != nil {
			log.Printf("Error polling Canton API: %v", err)
			cantonAPIUp.Set(0)
			pollErrorsTotal.Inc()
			continue
		}

		// If we reached here, the API is up.
		cantonAPIUp.Set(1)

		// Update metrics based on the query result.
		unclaimedLivenessRewards.Set(float64(len(contracts)))

		// Find the latest contract creation time to calculate ledger lag.
		if len(contracts) > 0 {
			// Sort contracts by creation time to find the most recent one.
			sort.Slice(contracts, func(i, j int) bool {
				return contracts[j].CreatedAt.Before(contracts[i].CreatedAt)
			})
			latestLedgerTime := contracts[0].CreatedAt
			lag := time.Since(latestLedgerTime).Seconds()
			ledgerTimeLag.Set(lag)
			log.Printf("Successfully polled. Unclaimed Rewards: %d, Ledger Lag: %.2fs", len(contracts), lag)
		} else {
			// If no contracts are found, we can't determine the lag.
			// We could query another template or reset the metric.
			// For now, we log it. A more robust solution might query a "heartbeat" contract.
			log.Println("No liveness reward contracts found; unable to calculate ledger time lag.")
		}
	}
}

// queryContracts queries the Canton JSON API for active contracts of a given template ID.
func (c *CantonClient) queryContracts(templateID string) ([]Contract, error) {
	if templateID == "" {
		// If no template is configured, we can't query anything to determine ledger health.
		// Return an error to signal that polling failed in a configurable way.
		return nil, fmt.Errorf("LIVENESS_TEMPLATE_ID is not configured")
	}

	url := fmt.Sprintf("%s/v1/query", c.config.CantonApiURL)
	queryBody := map[string][]string{
		"templateIds": {templateID},
	}

	bodyBytes, err := json.Marshal(queryBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query body: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.CantonApiToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var queryResp QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&queryResp); err != nil {
		return nil, fmt.Errorf("failed to decode api response: %w", err)
	}

	return queryResp.Result, nil
}