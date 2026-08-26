package dtos

import (
	"testing"
	"time"

	"webhook-tester/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestNewWebhookDTO(t *testing.T) {
	contentType := "application/json"
	payload := `{"foo":"bar"}`
	createdAt := time.Now().Add(-time.Hour)
	updatedAt := time.Now()

	requests := []models.WebhookRequest{
		{
			ID:        "req-1",
			WebhookID: "wh-1",
			Method:    "POST",
			Body:      "{}",
		},
	}

	w := models.Webhook{
		ID:            "wh-1",
		Title:         "Test Webhook",
		ResponseCode:  201,
		ResponseDelay: 500,
		ContentType:   &contentType,
		Payload:       &payload,
		NotifyOnEvent: true,
		UserID:        42,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		Requests:      requests,
	}

	dto := NewWebhookDTO(w)

	assert.Equal(t, w.ID, dto.ID)
	assert.Equal(t, w.Title, dto.Title)
	assert.Equal(t, w.ResponseCode, dto.ResponseCode)
	assert.Equal(t, w.ResponseDelay, dto.ResponseDelay)
	assert.Equal(t, contentType, dto.ContentType)
	assert.Equal(t, payload, dto.Payload)
	assert.Equal(t, w.NotifyOnEvent, dto.NotifyOnEvent)
	assert.Equal(t, w.UserID, dto.UserID)
	assert.Equal(t, w.CreatedAt, dto.CreatedAt)
	assert.Equal(t, w.UpdatedAt, dto.UpdatedAt)
	assert.Equal(t, requests, dto.Requests)
}
