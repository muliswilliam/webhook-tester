package view

import (
	"fmt"
	"strconv"

	"webhook-tester/internal/models"
)

// Delivery status tones, which pick the badge colors.
const (
	toneSuccess = "success"
	toneNeutral = "neutral"
	toneWarning = "warning"
	toneDanger  = "danger"
)

// DeliveryStatus is how a delivery's outcome is shown.
type DeliveryStatus struct {
	Label string // short, for a badge: "200", "Error", "Blocked" or "Dropped"
	Tone  string // success, neutral, warning or danger
	Title string // longer, for a tooltip: "200 OK", or the error
}

// outcomeTones color each delivery outcome: green for 2xx, neutral for 3xx,
// amber for 4xx, and red for 5xx and deliveries that never got an answer.
var outcomeTones = map[models.DeliveryOutcome]string{
	models.DeliveryOutcome2xx:     toneSuccess,
	models.DeliveryOutcome3xx:     toneNeutral,
	models.DeliveryOutcome4xx:     toneWarning,
	models.DeliveryOutcome5xx:     toneDanger,
	models.DeliveryOutcomeError:   toneDanger,
	models.DeliveryOutcomeBlocked: toneDanger,
	models.DeliveryOutcomeDropped: toneDanger,
}

// deliveryStatus describes d's outcome.
func deliveryStatus(d models.Delivery) DeliveryStatus {
	s := DeliveryStatus{Tone: outcomeTones[d.Outcome]}
	if s.Tone == "" {
		s.Tone = toneDanger
	}
	if d.Outcome.Answered() && d.StatusCode != nil {
		s.Label = strconv.Itoa(*d.StatusCode)
		s.Title = d.StatusLine()
		return s
	}

	switch d.Outcome {
	case models.DeliveryOutcomeBlocked:
		s.Label = "Blocked"
	case models.DeliveryOutcomeDropped:
		s.Label = "Dropped"
	default:
		s.Label = "Error"
	}
	s.Title = "Not delivered"
	if d.Error != nil {
		s.Title += ": " + *d.Error
	}
	return s
}

// DeliveryBadge is the data of the "delivery-badge" template: the compact
// status of a captured request's latest delivery.
type DeliveryBadge struct {
	RequestID string
	Delivery  *models.Delivery // nil while the request has none
}

// NewDeliveryBadge is the badge of wr, whose deliveries are newest first.
func NewDeliveryBadge(wr models.WebhookRequest) DeliveryBadge {
	b := DeliveryBadge{RequestID: wr.ID}
	if len(wr.Deliveries) > 0 {
		b.Delivery = &wr.Deliveries[0]
	}
	return b
}

// formatDuration shows a duration given in milliseconds, switching to
// seconds from one second up: "84 ms", "1.2 s".
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	return fmt.Sprintf("%.1f s", float64(ms)/1000)
}
