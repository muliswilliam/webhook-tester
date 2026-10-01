package models

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gorm.io/datatypes"
)

// DeliveryTrigger records what started a delivery.
type DeliveryTrigger string

const (
	// DeliveryTriggerAuto is a delivery made automatically when a request
	// was captured.
	DeliveryTriggerAuto DeliveryTrigger = "auto"
	// DeliveryTriggerReplay is a delivery a user started by replaying a
	// captured request to the forward URL.
	DeliveryTriggerReplay DeliveryTrigger = "replay"
)

// ReplayTarget is where a replay re-sends a captured request.
type ReplayTarget string

const (
	// ReplayTargetEndpoint re-sends the request to its Webhook Tester
	// endpoint, capturing a copy. It is the default.
	ReplayTargetEndpoint ReplayTarget = "endpoint"
	// ReplayTargetForward relays the request to its webhook's forward URL,
	// recording a replay delivery on the original request.
	ReplayTargetForward ReplayTarget = "forward"
)

// DeliveryOutcome classifies how a delivery ended. The UI colors deliveries
// by it and the delivery metrics are labelled with it.
type DeliveryOutcome string

const (
	DeliveryOutcome2xx DeliveryOutcome = "2xx"
	DeliveryOutcome3xx DeliveryOutcome = "3xx"
	DeliveryOutcome4xx DeliveryOutcome = "4xx"
	DeliveryOutcome5xx DeliveryOutcome = "5xx"
	// DeliveryOutcomeError is a delivery that failed at the network level
	// (DNS, connection refused, TLS, timeout) or wasn't attempted, e.g.
	// because the forwarding queue was full.
	DeliveryOutcomeError DeliveryOutcome = "error"
	// DeliveryOutcomeBlocked is a delivery refused because its destination
	// resolved to an address forwarding isn't allowed to reach.
	DeliveryOutcomeBlocked DeliveryOutcome = "blocked"
)

// DeliveryOutcomeForStatus classifies a delivery the target answered with
// the HTTP status code. Informational codes, which a client never sees as a
// final answer except for a protocol switch, count as 2xx.
func DeliveryOutcomeForStatus(code int) DeliveryOutcome {
	switch {
	case code < 300:
		return DeliveryOutcome2xx
	case code < 400:
		return DeliveryOutcome3xx
	case code < 500:
		return DeliveryOutcome4xx
	default:
		return DeliveryOutcome5xx
	}
}

// Answered reports whether the target answered, i.e. the delivery has a
// status code.
func (o DeliveryOutcome) Answered() bool {
	switch o {
	case DeliveryOutcome2xx, DeliveryOutcome3xx, DeliveryOutcome4xx, DeliveryOutcome5xx:
		return true
	}
	return false
}

// MaxDeliveryResponseBody caps how many bytes of a forward target's response
// body a delivery stores; ResponseBodyTruncated is set when the body was
// longer.
const MaxDeliveryResponseBody = 64 << 10 // 64 KiB

// MaxDeliveryResponseHeaders caps how many bytes of response headers a
// forward target may send. A longer response is aborted and recorded as an
// error, so a delivery never stores more than this of them.
const MaxDeliveryResponseHeaders = 64 << 10 // 64 KiB

// Delivery is one attempt to relay a captured request to a forward target.
// A delivery that reached the target has a StatusCode; one that failed at
// the network level (DNS, connection refused, TLS, timeout, blocked
// destination) has an Error instead. Outcome tells them apart.
type Delivery struct {
	ID         string          `gorm:"primaryKey" json:"id"`
	RequestID  string          `gorm:"index;not null" json:"request_id"`
	WebhookID  string          `gorm:"index;not null" json:"webhook_id"`
	Trigger    DeliveryTrigger `gorm:"not null" json:"trigger"`
	TargetURL  string          `json:"target_url"`
	Outcome    DeliveryOutcome `gorm:"not null;default:''" json:"outcome"`
	StatusCode *int            `json:"status_code"`
	Error      *string         `json:"error"`
	DurationMs int64           `json:"duration_ms"`
	// ResponseHeaders map each name to its value, or to the list of its
	// values when it was repeated, as WebhookRequest.Headers do.
	ResponseHeaders       datatypes.JSONMap `json:"response_headers"`
	ResponseBody          string            `json:"response_body"`
	ResponseBodyTruncated bool              `json:"response_body_truncated"`
	StartedAt             time.Time         `gorm:"index" json:"started_at"`
}

// StatusLine is the status the target answered with and its reason phrase,
// e.g. "500 Internal Server Error", or just the code for one without a
// standard phrase. It is "" for a delivery that got no answer.
func (d Delivery) StatusLine() string {
	if d.StatusCode == nil {
		return ""
	}
	if text := http.StatusText(*d.StatusCode); text != "" {
		return fmt.Sprintf("%d %s", *d.StatusCode, text)
	}
	return strconv.Itoa(*d.StatusCode)
}

// Label is how the trigger is shown: "Auto" or "Replay".
func (t DeliveryTrigger) Label() string {
	if t == DeliveryTriggerReplay {
		return "Replay"
	}
	return "Auto"
}
