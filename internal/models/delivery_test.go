package models

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

func TestDeliveryOutcome_Answered(t *testing.T) {
	for _, o := range []DeliveryOutcome{DeliveryOutcome2xx, DeliveryOutcome3xx, DeliveryOutcome4xx, DeliveryOutcome5xx} {
		assert.True(t, o.Answered(), o)
	}
	for _, o := range []DeliveryOutcome{DeliveryOutcomeError, DeliveryOutcomeBlocked, ""} {
		assert.False(t, o.Answered(), o)
	}
}

func TestDelivery_StatusLine(t *testing.T) {
	code := func(c int) *int { return &c }
	assert.Equal(t, "500 Internal Server Error", Delivery{StatusCode: code(500)}.StatusLine())
	assert.Equal(t, "599", Delivery{StatusCode: code(599)}.StatusLine(), "no standard reason phrase")
	assert.Equal(t, "", Delivery{}.StatusLine(), "no answer")
}

func TestDeliveryTrigger_Label(t *testing.T) {
	assert.Equal(t, "Auto", DeliveryTriggerAuto.Label())
	assert.Equal(t, "Replay", DeliveryTriggerReplay.Label())
}
