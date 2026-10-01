package dtos

import (
	"encoding/json"
	"testing"
	"time"

	"webhook-tester/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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
	assert.Equal(t, w.CreatedAt.UTC(), dto.CreatedAt)
	assert.Equal(t, time.UTC, dto.CreatedAt.Location())
	assert.Equal(t, w.UpdatedAt.UTC(), dto.UpdatedAt)
	assert.Equal(t, []WebhookRequest{{
		ID: "req-1", WebhookID: "wh-1", Method: "POST", Body: "{}",
		Headers: map[string]string{}, Query: map[string]string{},
	}}, dto.Requests)
	assert.Equal(t, map[string]string{}, dto.ResponseHeaders)
}

// Repeated headers and query parameters keep the API's string values,
// joined by commas as before they were stored separately.
func TestNewWebhookRequestDTO_RepeatedValues(t *testing.T) {
	dto := NewWebhookRequestDTO(models.WebhookRequest{
		Headers: datatypes.JSONMap{"X-Multi": []any{"one", "two"}, "X-Single": "only"},
		Query:   datatypes.JSONMap{"a": []any{"1", "2"}, "n": 5.0},
	})
	assert.Equal(t, map[string]string{"X-Multi": "one,two", "X-Single": "only"}, dto.Headers)
	assert.Equal(t, map[string]string{"a": "1,2", "n": "5"}, dto.Query)
}

func TestNewWebhookDTO_NilFields(t *testing.T) {
	dto := NewWebhookDTO(models.Webhook{ID: "wh-1"})

	assert.Equal(t, "", dto.ContentType)
	assert.Equal(t, "", dto.Payload)
	assert.NotNil(t, dto.Requests)
	assert.NotNil(t, dto.ResponseHeaders)
	assert.Nil(t, dto.ForwardURL)
}

func TestNewWebhookDTO_ForwardURL(t *testing.T) {
	forwardURL := "https://api.example.com/hooks"
	dto := NewWebhookDTO(models.Webhook{ID: "wh-1", ForwardURL: &forwardURL})

	require.NotNil(t, dto.ForwardURL)
	assert.Equal(t, forwardURL, *dto.ForwardURL)
}

func TestNullableString_JSON(t *testing.T) {
	decode := func(body string) UpdateWebhookRequest {
		t.Helper()
		var req UpdateWebhookRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		return req
	}

	assert.False(t, decode(`{}`).ForwardURL.Set, "a missing field isn't set")

	null := decode(`{"forward_url":null}`).ForwardURL
	assert.True(t, null.Set, "an explicit null is set")
	assert.Nil(t, null.Value)

	value := decode(`{"forward_url":"https://x.example"}`).ForwardURL
	assert.True(t, value.Set)
	require.NotNil(t, value.Value)
	assert.Equal(t, "https://x.example", *value.Value)

	var req UpdateWebhookRequest
	assert.ErrorContains(t, json.Unmarshal([]byte(`{"forward_url":true}`), &req), "expected a string or null, got a boolean")

	// Encoding round-trips, leaving an unset field out.
	out, err := json.Marshal(UpdateWebhookRequest{})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "forward_url")
	out, err = json.Marshal(UpdateWebhookRequest{ForwardURL: NewNullableString(nil)})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"forward_url":null`)
	out, err = json.Marshal(UpdateWebhookRequest{ForwardURL: NewNullableString(value.Value)})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"forward_url":"https://x.example"`)
}
