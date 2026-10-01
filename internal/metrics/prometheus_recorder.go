package metrics

import (
	"time"

	"webhook-tester/internal/models"
)

type PrometheusRecorder struct{}

func (r *PrometheusRecorder) IncWebhooksCreated() {
	WebhooksCreated.Inc()
}

func (r *PrometheusRecorder) IncWebhookRequest(webhookID string) {
	WebhookRequestsReceived.WithLabelValues(webhookID).Inc()
}

func (r *PrometheusRecorder) IncSignUp() {
	SignupsTotal.Inc()
}

func (r *PrometheusRecorder) IncLogin() {
	LoginsTotal.Inc()
}

func (r *PrometheusRecorder) ObserveDelivery(outcome models.DeliveryOutcome, duration time.Duration) {
	DeliveriesTotal.WithLabelValues(string(outcome)).Inc()
	DeliveryDuration.Observe(duration.Seconds())
}
