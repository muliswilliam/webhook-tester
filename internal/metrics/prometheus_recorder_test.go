package metrics

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func counterValue(t *testing.T, c interface{ Write(*dto.Metric) error }) float64 {
	t.Helper()
	m := &dto.Metric{}
	require.NoError(t, c.Write(m))
	return m.GetCounter().GetValue()
}

func TestPrometheusRecorderIncWebhooksCreated(t *testing.T) {
	before := counterValue(t, WebhooksCreated)

	r := &PrometheusRecorder{}
	r.IncWebhooksCreated()

	after := counterValue(t, WebhooksCreated)
	assert.Equal(t, before+1, after)
}

func TestPrometheusRecorderIncWebhookRequest(t *testing.T) {
	r := &PrometheusRecorder{}
	before := counterValue(t, WebhookRequestsReceived.WithLabelValues("wh-abc"))

	r.IncWebhookRequest("wh-abc")

	after := counterValue(t, WebhookRequestsReceived.WithLabelValues("wh-abc"))
	assert.Equal(t, before+1, after)
}

func TestPrometheusRecorderIncSignUp(t *testing.T) {
	before := counterValue(t, SignupsTotal)

	r := &PrometheusRecorder{}
	r.IncSignUp()

	after := counterValue(t, SignupsTotal)
	assert.Equal(t, before+1, after)
}

func TestPrometheusRecorderIncLogin(t *testing.T) {
	before := counterValue(t, LoginsTotal)

	r := &PrometheusRecorder{}
	r.IncLogin()

	after := counterValue(t, LoginsTotal)
	assert.Equal(t, before+1, after)
}

func histogramSnapshot(t *testing.T, h interface{ Write(*dto.Metric) error }) (count uint64, sum float64) {
	t.Helper()
	m := &dto.Metric{}
	require.NoError(t, h.Write(m))
	return m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
}

func TestPrometheusRecorderObserveDelivery(t *testing.T) {
	r := &PrometheusRecorder{}
	outcomes := []DeliveryOutcome{
		DeliveryOutcome2xx, DeliveryOutcome3xx, DeliveryOutcome4xx,
		DeliveryOutcome5xx, DeliveryOutcomeError, DeliveryOutcomeBlocked,
	}
	for _, outcome := range outcomes {
		t.Run(string(outcome), func(t *testing.T) {
			before := make(map[DeliveryOutcome]float64)
			for _, o := range outcomes {
				before[o] = counterValue(t, DeliveriesTotal.WithLabelValues(string(o)))
			}
			countBefore, sumBefore := histogramSnapshot(t, DeliveryDuration)

			r.ObserveDelivery(outcome, 1500*time.Millisecond)

			for _, o := range outcomes {
				want := before[o]
				if o == outcome {
					want++
				}
				assert.Equal(t, want, counterValue(t, DeliveriesTotal.WithLabelValues(string(o))), "outcome %s", o)
			}
			countAfter, sumAfter := histogramSnapshot(t, DeliveryDuration)
			assert.Equal(t, countBefore+1, countAfter)
			assert.InDelta(t, sumBefore+1.5, sumAfter, 1e-9)
		})
	}
}

func TestPrometheusRecorderImplementsRecorder(t *testing.T) {
	var _ Recorder = &PrometheusRecorder{}
}
