package metrics

// DeliveryOutcome classifies a delivery for the delivery metrics.
type DeliveryOutcome string

const (
	DeliveryOutcome2xx DeliveryOutcome = "2xx"
	DeliveryOutcome3xx DeliveryOutcome = "3xx"
	DeliveryOutcome4xx DeliveryOutcome = "4xx"
	DeliveryOutcome5xx DeliveryOutcome = "5xx"
	// DeliveryOutcomeError is a delivery that failed at the network level
	// (DNS, connection refused, TLS, timeout) or couldn't be attempted,
	// e.g. because the forwarding queue was full.
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
