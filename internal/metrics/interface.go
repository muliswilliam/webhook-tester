package metrics

import "time"

type Recorder interface {
	IncWebhooksCreated()
	IncWebhookRequest(webhookID string)
	IncSignUp()
	IncLogin()
	// ObserveDelivery records one delivery's outcome and how long it took.
	ObserveDelivery(outcome DeliveryOutcome, duration time.Duration)
}
