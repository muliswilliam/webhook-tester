package models

import (
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

// MaxDeliveryResponseBody caps how many bytes of a forward target's response
// body a delivery stores; ResponseBodyTruncated is set when the body was
// longer.
const MaxDeliveryResponseBody = 64 << 10 // 64 KiB

// Delivery is one attempt to relay a captured request to a forward target.
// A delivery that reached the target has a StatusCode; one that failed at
// the network level (DNS, connection refused, TLS, timeout, blocked
// destination) has an Error instead.
type Delivery struct {
	ID                    string            `gorm:"primaryKey" json:"id"`
	RequestID             string            `gorm:"index;not null" json:"request_id"`
	WebhookID             string            `gorm:"index;not null" json:"webhook_id"`
	Trigger               DeliveryTrigger   `gorm:"not null" json:"trigger"`
	TargetURL             string            `json:"target_url"`
	StatusCode            *int              `json:"status_code"`
	Error                 *string           `json:"error"`
	DurationMs            int64             `json:"duration_ms"`
	ResponseHeaders       datatypes.JSONMap `json:"response_headers"`
	ResponseBody          string            `json:"response_body"`
	ResponseBodyTruncated bool              `json:"response_body_truncated"`
	StartedAt             time.Time         `gorm:"index" json:"started_at"`
}
