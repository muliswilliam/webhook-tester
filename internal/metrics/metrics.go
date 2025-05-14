package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	// webhooks
	WebhooksCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "webhooks_created_total",
		Help: "Total number of webhooks created by users or guests.",
	})

	// incoming webhook requests per webhook ID
	WebhookRequestsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_requests_received_total",
		Help: "Total number of webhook requests received per webhook ID.",
	}, []string{"webhook_id"})

	// user signups
	SignupsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_signups_total",
			Help: "Total number of successful user registrations.",
		},
	)

	// user logins
	LoginsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "user_logins_total",
			Help: "Total number of successful user logins.",
		},
	)
)

// Register prometheus metrics
func Register() {
	prometheus.MustRegister(
		WebhooksCreated,
		WebhookRequestsReceived,
		SignupsTotal,
		LoginsTotal,
	)
}
