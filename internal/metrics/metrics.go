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

	// deliveries of captured requests to forward targets, by outcome
	DeliveriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webhook_deliveries_total",
		Help: "Total number of deliveries to forward targets, by outcome (2xx, 3xx, 4xx, 5xx, error, blocked).",
	}, []string{"outcome"})

	// how long deliveries to forward targets take
	DeliveryDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "webhook_delivery_duration_seconds",
		Help:    "Duration of deliveries to forward targets, in seconds.",
		Buckets: prometheus.DefBuckets,
	})
)

// Register registers the app's metrics with reg, panicking if any is already
// registered there. Each server registers them with its own registry, so
// several servers (as in tests) can coexist in one process.
func Register(reg prometheus.Registerer) {
	reg.MustRegister(
		WebhooksCreated,
		WebhookRequestsReceived,
		SignupsTotal,
		LoginsTotal,
		DeliveriesTotal,
		DeliveryDuration,
	)
}
