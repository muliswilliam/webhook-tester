package memory

import (
	"errors"
	"sync"
	"webhook-tester/internal/models"
)

var (
	mutex    sync.Mutex
	webhooks = make(map[string]models.Webhook)
)

func SaveWebhook(webhook models.Webhook) {
	mutex.Lock()
	defer mutex.Unlock()
	webhooks[webhook.ID] = webhook
}

func ListWebhooks() []models.Webhook {
	mutex.Lock()
	defer mutex.Unlock()
	list := make([]models.Webhook, 0, len(webhooks))

	for _, s := range webhooks {
		list = append(list, s)
	}

	return list
}

func GetWebhookByID(sessionId string) (models.Webhook, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	session, ok := webhooks[sessionId]

	return session, ok
}

func UpdateWebhook(patch models.Webhook) (models.Webhook, error) {
	mutex.Lock()
	defer mutex.Unlock()

	webhook, found := webhooks[patch.ID]

	if !found {
		return webhook, errors.New("not found")
	}

	// Apply updates (only if set)
	if patch.Title != "" {
		webhook.Title = patch.Title
	}
	if patch.ResponseCode != 0 {
		webhook.ResponseCode = patch.ResponseCode
	}
	if patch.ContentType != "" {
		webhook.ContentType = patch.ContentType
	}
	if patch.ResponseDelay != 0 {
		webhook.ResponseDelay = patch.ResponseDelay
	}
	if patch.Payload != "" {
		webhook.Payload = patch.Payload
	}
	webhooks[webhook.ID] = webhook

	return webhook, nil
}
