package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeliveryOutcomeForStatus(t *testing.T) {
	tests := []struct {
		code int
		want DeliveryOutcome
	}{
		{101, DeliveryOutcome2xx},
		{200, DeliveryOutcome2xx},
		{204, DeliveryOutcome2xx},
		{299, DeliveryOutcome2xx},
		{301, DeliveryOutcome3xx},
		{399, DeliveryOutcome3xx},
		{400, DeliveryOutcome4xx},
		{404, DeliveryOutcome4xx},
		{500, DeliveryOutcome5xx},
		{503, DeliveryOutcome5xx},
		{599, DeliveryOutcome5xx},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, DeliveryOutcomeForStatus(tt.code), "code %d", tt.code)
	}
}
