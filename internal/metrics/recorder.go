package metrics


type Recorder interface {
	IncWebhooksCreated()
	IncWebhookRequest(webhookID string)
	IncSignUp()
	IncLogin()
}
