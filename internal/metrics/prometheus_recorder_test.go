package metrics

import (
	"testing"

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

func TestPrometheusRecorderImplementsRecorder(t *testing.T) {
	var _ Recorder = &PrometheusRecorder{}
}
