package view

import (
	"fmt"
	"net/http"
	"strings"

	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
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
	Label string // short, for a badge: "200", "Error" or "Blocked"
	Tone  string // success, neutral, warning or danger
	Title string // longer, for a tooltip: "200 OK", or the error
}

// deliveryStatus describes d's outcome: green for 2xx, neutral for a 3xx,
// amber for 4xx, and red for 5xx and deliveries that never got an answer.
func deliveryStatus(d models.Delivery) DeliveryStatus {
	if d.StatusCode == nil {
		s := DeliveryStatus{Label: "Error", Tone: toneDanger, Title: "Not delivered"}
		if d.Error != nil {
			s.Title = "Not delivered: " + *d.Error
			if strings.HasPrefix(*d.Error, service.ErrMsgDestinationNotAllowed) {
				s.Label = "Blocked"
			}
		}
		return s
	}

	code := *d.StatusCode
	s := DeliveryStatus{Label: fmt.Sprint(code), Title: fmt.Sprint(code)}
	if text := http.StatusText(code); text != "" {
		s.Title += " " + text
	}
	switch {
	case code < 300:
		s.Tone = toneSuccess
	case code < 400:
		s.Tone = toneNeutral
	case code < 500:
		s.Tone = toneWarning
	default:
		s.Tone = toneDanger
	}
	return s
}

// DeliveryBadge is the data of the "delivery-badge" template: the compact
// status of a captured request's latest delivery.
type DeliveryBadge struct {
	RequestID string
	Delivery  *models.Delivery // nil while the request has none
}

// deliveryBadge is the badge of wr, whose deliveries are newest first.
func deliveryBadge(wr models.WebhookRequest) DeliveryBadge {
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
