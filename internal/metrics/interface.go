package metrics

import (
	"time"

	"webhook-tester/internal/models"
)

type Recorder interface {
	IncWebhooksCreated()
	IncWebhookRequest(webhookID string)
	IncSignUp()
	IncLogin()
	// ObserveDelivery records one forward attempt's outcome and how long it
	// took. Every attempt is observed, including refused ones (queue full,
	// shutting down) and those whose delivery couldn't be stored.
	ObserveDelivery(outcome models.DeliveryOutcome, duration time.Duration)
}
