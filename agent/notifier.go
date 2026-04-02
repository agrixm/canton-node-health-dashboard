package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Notifier defines the interface for sending alerts to various services.
type Notifier interface {
	Notify(alert Alert) error
}

// --- Slack Notifier ---

// SlackNotifier sends alerts to a Slack channel via an incoming webhook.
type SlackNotifier struct {
	WebhookURL string
	httpClient *http.Client
}

// SlackPayload represents the JSON structure for a Slack webhook message.
type SlackPayload struct {
	Attachments []SlackAttachment `json:"attachments"`
}

// SlackAttachment provides rich formatting for Slack messages.
type SlackAttachment struct {
	Color  string                 `json:"color"`
	Title  string                 `json:"title"`
	Text   string                 `json:"text"`
	Fields []SlackAttachmentField `json:"fields"`
	Ts     int64                  `json:"ts"`
}

// SlackAttachmentField is used for key-value pairs within an attachment.
type SlackAttachmentField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// NewSlackNotifier creates a new SlackNotifier instance.
// It requires the SLACK_WEBHOOK_URL environment variable to be set.
func NewSlackNotifier() (*SlackNotifier, error) {
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL == "" {
		return nil, fmt.Errorf("SLACK_WEBHOOK_URL environment variable not set")
	}

	return &SlackNotifier{
		WebhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Notify sends a formatted alert to the configured Slack webhook.
func (s *SlackNotifier) Notify(alert Alert) error {
	color := "#808080" // Default to grey for info
	switch alert.Severity {
	case SeverityWarning:
		color = "warning" // Slack's yellow
	case SeverityCritical:
		color = "danger" // Slack's red
	}

	var fields []SlackAttachmentField
	for k, v := range alert.Details {
		fields = append(fields, SlackAttachmentField{
			Title: k,
			Value: v,
			Short: true,
		})
	}

	payload := SlackPayload{
		Attachments: []SlackAttachment{
			{
				Color:  color,
				Title:  fmt.Sprintf("Canton Node Alert: %s", alert.Name),
				Text:   alert.Message,
				Fields: fields,
				Ts:     alert.Timestamp.Unix(),
			},
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequest("POST", s.WebhookURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack API returned non-200 status: %d", resp.StatusCode)
	}

	log.Printf("Successfully sent notification to Slack for alert: %s", alert.Name)
	return nil
}

// --- PagerDuty Notifier ---

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutyNotifier sends alerts to PagerDuty.
type PagerDutyNotifier struct {
	RoutingKey string
	httpClient *http.Client
}

// PagerDutyPayload is the structure for the PagerDuty Events API v2.
type PagerDutyPayload struct {
	RoutingKey  string                `json:"routing_key"`
	EventAction string                `json:"event_action"`
	DedupKey    string                `json:"dedup_key"`
	Payload     PagerDutyEventPayload `json:"payload"`
}

// PagerDutyEventPayload contains the details of the PagerDuty event.
type PagerDutyEventPayload struct {
	Summary       string            `json:"summary"`
	Source        string            `json:"source"`
	Severity      string            `json:"severity"`
	Timestamp     string            `json:"timestamp"`
	Component     string            `json:"component"`
	CustomDetails map[string]string `json:"custom_details"`
}

// NewPagerDutyNotifier creates a new PagerDutyNotifier instance.
// It requires the PAGERDUTY_ROUTING_KEY environment variable to be set.
func NewPagerDutyNotifier() (*PagerDutyNotifier, error) {
	routingKey := os.Getenv("PAGERDUTY_ROUTING_KEY")
	if routingKey == "" {
		return nil, fmt.Errorf("PAGERDUTY_ROUTING_KEY environment variable not set")
	}

	return &PagerDutyNotifier{
		RoutingKey: routingKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Notify sends a formatted alert to the PagerDuty Events API.
func (p *PagerDutyNotifier) Notify(alert Alert) error {
	// PagerDuty only accepts certain severity levels. Map ours to theirs.
	pdSeverity := "info"
	switch alert.Severity {
	case SeverityWarning:
		pdSeverity = "warning"
	case SeverityCritical:
		pdSeverity = "critical"
	}

	// Source can be the node's hostname or a configured identifier
	source, err := os.Hostname()
	if err != nil {
		source = "canton-node" // fallback
	}

	// Create a stable deduplication key to group related alerts
	dedupKey := fmt.Sprintf("canton-monitor-%s-%s", source, strings.ReplaceAll(strings.ToLower(alert.Name), " ", "-"))

	payload := PagerDutyPayload{
		RoutingKey:  p.RoutingKey,
		EventAction: "trigger", // or "resolve"
		DedupKey:    dedupKey,
		Payload: PagerDutyEventPayload{
			Summary:       fmt.Sprintf("Canton Node Alert: %s", alert.Message),
			Source:        source,
			Severity:      pdSeverity,
			Timestamp:     alert.Timestamp.Format(time.RFC3339),
			Component:     "canton-validator",
			CustomDetails: alert.Details,
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal pagerduty payload: %w", err)
	}

	req, err := http.NewRequest("POST", pagerDutyEventsURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create pagerduty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pagerduty notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("pagerduty API returned non-202 status: %d", resp.StatusCode)
	}

	log.Printf("Successfully sent notification to PagerDuty for alert: %s", alert.Name)
	return nil
}