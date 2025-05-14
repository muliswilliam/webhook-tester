package metrics

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
