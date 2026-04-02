package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// NOTE: These are illustrative imports based on common Ledger API and Canton Admin API structures.
	// Replace with the actual Go client library paths for your Canton version.
	ledgerservice "github.com/digital-asset/dazl-client/v7/go/api/com/digitalasset/ledger/api/v1"
	transactionpb "github.com/digital-asset/dazl-client/v7/go/api/com/digitalasset/ledger/api/v1"
	cantonsequencer "github.com/digital-asset/canton-_enterprise-open-source-ce997cce/community/protocol/src/main/protobuf/com/digitalasset/canton/protocol/admin/v0"
)

// Metric descriptions for the Canton validator node.
var (
	syncLagDesc = prometheus.NewDesc(
		"canton_validator_sync_lag_seconds",
		"Time difference between the sequencer's latest record time and the validator participant's latest processed record time.",
		[]string{"participant_id", "sequencer_id"}, nil,
	)

	lastScrapeErrorDesc = prometheus.NewDesc(
		"canton_agent_scrape_error",
		"Indicates an error during the last metric scrape (1 for error, 0 for success).",
		[]string{"target_node"}, nil,
	)
)

// Collector implements the prometheus.Collector interface to gather metrics from a Canton node.
type Collector struct {
	ParticipantID   string
	ParticipantAddr string
	SequencerAddr   string

	participantConn *grpc.ClientConn
	sequencerConn   *grpc.ClientConn

	transactionClient ledgerservice.TransactionServiceClient
	sequencerClient   cantonsequencer.SequencerAdministrationServiceClient

	// State updated by background processes
	mu            sync.RWMutex
	maxRecordTime time.Time
	lastError     error
}

// NewCollector creates and initializes a collector for Canton metrics.
// It establishes gRPC connections and starts a background goroutine to monitor the transaction stream.
func NewCollector(participantID, participantAddr, sequencerAddr, ledgerId string) (*Collector, error) {
	pConn, err := grpc.Dial(participantAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	sConn, err := grpc.Dial(sequencerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		pConn.Close()
		return nil, err
	}

	c := &Collector{
		ParticipantID:     participantID,
		ParticipantAddr:   participantAddr,
		SequencerAddr:     sequencerAddr,
		participantConn:   pConn,
		sequencerConn:     sConn,
		transactionClient: ledgerservice.NewTransactionServiceClient(pConn),
		sequencerClient:   cantonsequencer.NewSequencerAdministrationServiceClient(sConn),
		maxRecordTime:     time.Time{},
	}

	// Start background process to keep maxRecordTime up to date.
	go c.streamTransactions(ledgerId)

	return c, nil
}

// streamTransactions continuously listens for new transactions from the participant's Ledger API
// and updates the latest seen record time. This represents the participant's processing watermark.
func (c *Collector) streamTransactions(ledgerId string) {
	// In a production agent, this would have more robust reconnection and backoff logic.
	for {
		log.Println("Connecting to participant transaction stream...")
		stream, err := c.transactionClient.GetTransactions(context.Background(), &ledgerservice.GetTransactionsRequest{
			LedgerId: ledgerId,
			Begin:    &ledgerservice.LedgerOffset{Value: &ledgerservice.LedgerOffset_Boundary{Boundary: ledgerservice.LedgerOffset_LEDGER_END}},
			Filter: &transactionpb.TransactionFilter{
				FiltersByParty: map[string]*transactionpb.Filters{}, // Empty filter to get all transactions for record time
			},
		})
		if err != nil {
			log.Printf("Failed to start transaction stream: %v. Retrying in 10s...", err)
			c.mu.Lock()
			c.lastError = err
			c.mu.Unlock()
			time.Sleep(10 * time.Second)
			continue
		}
		log.Println("Transaction stream started successfully.")

		for {
			resp, err := stream.Recv()
			if err != nil {
				log.Printf("Transaction stream error: %v. Reconnecting...", err)
				c.mu.Lock()
				c.lastError = err
				c.mu.Unlock()
				break // Break inner loop to trigger reconnection
			}

			for _, tx := range resp.GetTransactions() {
				if tx.GetRecordTime() != nil {
					rt := tx.GetRecordTime().AsTime()
					c.mu.Lock()
					if rt.After(c.maxRecordTime) {
						c.maxRecordTime = rt
					}
					c.lastError = nil // Clear error on successful receive
					c.mu.Unlock()
				}
			}
		}
		time.Sleep(5 * time.Second) // Wait before attempting to reconnect
	}
}

// Describe sends the descriptors of all metrics collected to the provided channel.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- syncLagDesc
	ch <- lastScrapeErrorDesc
}

// Collect is called by the Prometheus registry when collecting metrics.
// It fetches the current sequencer time and compares it with the latest processed time.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	// Fetch the authoritative time from the sequencer's admin API.
	// This represents the "head" of the sequenced events.
	status, err := c.sequencerClient.Status(context.Background(), &cantonsequencer.SequencerStatusRequest{})
	if err != nil {
		log.Printf("Failed to get sequencer status: %v", err)
		ch <- prometheus.MustNewConstMetric(lastScrapeErrorDesc, prometheus.GaugeValue, 1, c.SequencerAddr)
		return
	}

	sequencerHeadTime := status.GetStatus().GetLatestRecordTimestamp().AsTime()
	sequencerID := status.GetStatus().GetSequencerId()

	// Read the latest record time processed by the participant.
	// This value is updated by the background goroutine.
	c.mu.RLock()
	participantHeadTime := c.maxRecordTime
	c.mu.RUnlock()

	// If we haven't seen any transactions yet, we can't calculate a meaningful lag.
	if participantHeadTime.IsZero() {
		log.Println("No transactions processed yet by participant, cannot calculate sync lag.")
		ch <- prometheus.MustNewConstMetric(lastScrapeErrorDesc, prometheus.GaugeValue, 0, c.ParticipantAddr)
		// Don't emit a sync lag metric yet.
		return
	}

	// This calculation is immune to clock skew between the agent's machine and the sequencer.
	// It compares two timestamps that originate from the same domain of time: the sequencer's clock.
	syncLag := sequencerHeadTime.Sub(participantHeadTime)

	// A small negative lag can occur due to probe timing. A large negative lag indicates a major issue.
	if syncLag < 0 {
		log.Printf("Warning: Calculated negative sync lag (%v). Sequencer head time: %s, Participant processed time: %s",
			syncLag, sequencerHeadTime.UTC(), participantHeadTime.UTC())
		// Clamp to zero for the metric, as negative lag is not a useful concept for dashboards.
		syncLag = 0
	}

	ch <- prometheus.MustNewConstMetric(
		syncLagDesc,
		prometheus.GaugeValue,
		syncLag.Seconds(),
		c.ParticipantID,
		sequencerID,
	)

	// Report successful scrapes for both nodes
	ch <- prometheus.MustNewConstMetric(lastScrapeErrorDesc, prometheus.GaugeValue, 0, c.ParticipantAddr)
	ch <- prometheus.MustNewConstMetric(lastScrapeErrorDesc, prometheus.GaugeValue, 0, c.SequencerAddr)
}

// Close cleans up the collector's gRPC connections.
func (c *Collector) Close() {
	if c.participantConn != nil {
		c.participantConn.Close()
	}
	if c.sequencerConn != nil {
		c.sequencerConn.Close()
	}
}