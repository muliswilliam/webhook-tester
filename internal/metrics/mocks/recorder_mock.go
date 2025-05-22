package metricsMocks

import "github.com/stretchr/testify/mock"

type RecorderMock struct {
	mock.Mock
}

func (r *RecorderMock) IncWebhooksCreated() {
	r.Called()
}

func (r *RecorderMock) IncWebhookRequest(webhookID string) {
	r.Called(webhookID)
}

func (r *RecorderMock) IncSignUp() {
	r.Called()
}

func (r *RecorderMock) IncLogin() {
	r.Called()
}
