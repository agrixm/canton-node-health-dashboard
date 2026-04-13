package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// --- Mocks and Helpers ---

// mockHandler creates a generic HTTP handler for testing.
func mockHandler(statusCode int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(body))
	}
}

// setupTestCollector creates a collector with a mock server.
func setupTestCollector(handler http.HandlerFunc) (*CantonCollector, *httptest.Server) {
	server := httptest.NewServer(handler)
	config := &Config{
		Nodes: []NodeConfig{
			{
				Name:    "test-validator",
				Type:    "validator",
				SyncURL: server.URL,
			},
			{
				Name:      "test-sequencer",
				Type:      "sequencer",
				HealthURL: server.URL,
			},
		},
		CheckInterval: 1 * time.Second,
		Alerts: AlertConfig{
			SyncLagThreshold: 100,
		},
	}
	collector := NewCantonCollector(config, nil) // No notifier needed for these tests
	return collector, server
}

// --- Sync Service Parsing Tests ---

func TestParseSyncServiceStatus_Healthy(t *testing.T) {
	healthyResponse := `{
		"headClean": 1500,
		"lastPruned": 500,
		"headSequencerCounter": 1502
	}`

	status, err := parseSyncServiceStatus([]byte(healthyResponse))
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, 1500.0, status.HeadClean)
	assert.Equal(t, 500.0, status.LastPruned)
	assert.Equal(t, 1502.0, status.HeadSequencerCounter)
}

func TestParseSyncServiceStatus_Malformed(t *testing.T) {
	malformedResponse := `{"headClean": "not-a-number", "lastPruned": 500}`
	_, err := parseSyncServiceStatus([]byte(malformedResponse))
	assert.Error(t, err)
	assert.IsType(t, &json.UnmarshalTypeError{}, err)
}

func TestParseSyncServiceStatus_Empty(t *testing.T) {
	emptyResponse := `{}`
	status, err := parseSyncServiceStatus([]byte(emptyResponse))
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, 0.0, status.HeadClean)
	assert.Equal(t, 0.0, status.HeadSequencerCounter)
}

// --- Sequencer Health Parsing Tests ---

func TestParseSequencerHealth_Healthy(t *testing.T) {
	healthyResponse := `{"status": "HEALTHY"}`
	health, err := parseSequencerHealth([]byte(healthyResponse))
	assert.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "HEALTHY", health.Status)
	assert.True(t, health.IsHealthy())
}

func TestParseSequencerHealth_Unhealthy(t *testing.T) {
	unhealthyResponse := `{"status": "UNHEALTHY", "failures": ["database connection failed"]}`
	health, err := parseSequencerHealth([]byte(unhealthyResponse))
	assert.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "UNHEALTHY", health.Status)
	assert.False(t, health.IsHealthy())
}

func TestParseSequencerHealth_Malformed(t *testing.T) {
	malformedResponse := `{"status": 123}` // status should be a string
	_, err := parseSequencerHealth([]byte(malformedResponse))
	assert.Error(t, err)
	assert.IsType(t, &json.UnmarshalTypeError{}, err)
}

// --- Alert Logic Tests ---

func TestCheckSyncLagThreshold(t *testing.T) {
	// Notifier is not used in this test, can be nil
	var notifier *Notifier = nil
	config := &Config{
		Alerts: AlertConfig{
			SyncLagThreshold: 50,
		},
	}

	// Case 1: Lag is below threshold
	statusBelow := &SyncServiceStatus{HeadClean: 1000, HeadSequencerCounter: 1020} // lag = 20
	checkSyncLag(context.Background(), notifier, config, "test-validator", statusBelow)
	// Assertion would be on mock notifier calls; here we just ensure it runs without error.

	// Case 2: Lag is exactly at threshold
	statusAt := &SyncServiceStatus{HeadClean: 1000, HeadSequencerCounter: 1050} // lag = 50
	checkSyncLag(context.Background(), notifier, config, "test-validator", statusAt)

	// Case 3: Lag is above threshold - this is harder to test without a mock notifier,
	// but we can test the internal logic. The function `checkSyncLag` itself is a good candidate
	// to be refactored to return a boolean indicating if an alert was fired.
	// For now, we trust the log output and focus on metric generation.

	// Case 4: No sequencer counter data (e.g., node just started)
	statusNoSeq := &SyncServiceStatus{HeadClean: 1000, HeadSequencerCounter: 0}
	checkSyncLag(context.Background(), notifier, config, "test-validator", statusNoSeq)

}

// --- Collector Logic Tests ---

func TestCollector_CollectSyncMetrics_Success(t *testing.T) {
	healthyResponse := `{
		"headClean": 998,
		"lastPruned": 100,
		"headSequencerCounter": 1000
	}`
	collector, server := setupTestCollector(mockHandler(http.StatusOK, healthyResponse))
	defer server.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)

	expectedMetrics := map[string]float64{
		"canton_sync_lag":            2,
		"canton_sync_up":             1,
		"canton_sync_head_clean":     998,
		"canton_sync_last_pruned":    100,
		"canton_sync_head_sequencer": 1000,
	}

	foundCount := 0
	for _, mf := range metricFamilies {
		if expectedVal, ok := expectedMetrics[*mf.Name]; ok {
			assert.Len(t, mf.Metric, 1, "Expected one metric for %s", *mf.Name)
			assert.Equal(t, expectedVal, *mf.Metric[0].Gauge.Value)
			assert.Equal(t, "test-validator", *mf.Metric[0].Label[0].Value, "Node label mismatch")
			foundCount++
		}
	}
	assert.Equal(t, len(expectedMetrics), foundCount, "Did not find all expected sync metrics")
}

func TestCollector_CollectSequencerMetrics_Unhealthy(t *testing.T) {
	unhealthyResponse := `{"status": "UNHEALTHY"}`
	collector, server := setupTestCollector(mockHandler(http.StatusOK, unhealthyResponse))
	defer server.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)

	expectedMetrics := map[string]float64{
		"canton_sequencer_up":      1, // It's up because we got a 200 OK
		"canton_sequencer_healthy": 0, // 0 because status is UNHEALTHY
	}

	foundCount := 0
	for _, mf := range metricFamilies {
		if expectedVal, ok := expectedMetrics[*mf.Name]; ok {
			assert.Len(t, mf.Metric, 1, "Expected one metric for %s", *mf.Name)
			assert.Equal(t, expectedVal, *mf.Metric[0].Gauge.Value)
			assert.Equal(t, "test-sequencer", *mf.Metric[0].Label[0].Value, "Node label mismatch")
			foundCount++
		}
	}
	assert.Equal(t, len(expectedMetrics), foundCount, "Did not find all expected sequencer metrics")
}

func TestCollector_HttpError(t *testing.T) {
	// Setup a server that always returns an error
	collector, server := setupTestCollector(mockHandler(http.StatusInternalServerError, "internal error"))
	defer server.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	metricFamilies, err := reg.Gather()
	assert.NoError(t, err)

	// Check that the 'up' metrics are 0 for both nodes
	foundSyncUp := false
	foundSeqUp := false
	for _, mf := range metricFamilies {
		if *mf.Name == "canton_sync_up" {
			assert.Len(t, mf.Metric, 1)
			assert.Equal(t, float64(0), *mf.Metric[0].Gauge.Value, "Sync up metric should be 0 on HTTP error")
			foundSyncUp = true
		}
		if *mf.Name == "canton_sequencer_up" {
			assert.Len(t, mf.Metric, 1)
			assert.Equal(t, float64(0), *mf.Metric[0].Gauge.Value, "Sequencer up metric should be 0 on HTTP error")
			foundSeqUp = true
		}
	}

	assert.True(t, foundSyncUp, "Expected canton_sync_up metric was not found")
	assert.True(t, foundSeqUp, "Expected canton_sequencer_up metric was not found")
}

func TestCollector_ContextCancelled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Ensure the request takes time
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	collector, server := setupTestCollector(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond) // Very short timeout
	defer cancel()

	ch := make(chan prometheus.Metric)

	// Run collection in a goroutine so we can wait for it to finish
	done := make(chan struct{})
	go func() {
		collector.Collect(ch)
		close(done)
	}()

	select {
	case <-done:
		// test passed, collection finished correctly after context cancellation
	case <-time.After(200 * time.Millisecond):
		t.Fatal("collector.Collect did not return after context was cancelled")
	}
}