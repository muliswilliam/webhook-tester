package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/unrolled/render"
)

var renderer = render.New()

func CreateWebhook(w http.ResponseWriter, r *http.Request) {
	// create webhook struct
	var input struct {
		Title         string `json:"title"`
		ResponseCode  int    `json:"response_code"`
		ResponseDelay uint   `json:"response_delay"` // milliseconds
		ContentType   string `json:"content_type"`
		Payload       string `json:"payload"`
		NotifyOnEvent bool   `json:"notify_on_event"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	code := input.ResponseCode
	if code == 0 {
		code = 200
	}

	webhook := models.Webhook{
		ID:            utils.GenerateID(),
		Title:         input.Title,
		ResponseCode:  code,
		ResponseDelay: input.ResponseDelay,
		ContentType:   &input.ContentType,
		Payload:       &input.Payload,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		NotifyOnEvent: input.NotifyOnEvent,
	}
	// persist struct
	err := sqlstore.InsertWebhook(webhook)

	if err != nil {
		renderer.JSON(w, http.StatusInternalServerError, "error inserting webhook")
	}

	renderer.JSON(w, http.StatusCreated, webhook)
}

func ListWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks, err := sqlstore.GetAllWebhooks()
	if err != nil {
		renderer.JSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err,
		})
		return
	}

	renderer.JSON(w, http.StatusOK, webhooks)
}

func GetWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	webhook, err := sqlstore.GetWebhook(webhookID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			renderer.JSON(w, http.StatusNotFound, "webhook not found")
		}

		log.Printf("error getting webhook: %v", err)

		renderer.JSON(w, http.StatusInternalServerError, "error getting webhook")
		return
	}

	renderer.JSON(w, http.StatusOK, webhook)
}

func UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")

	webhook := models.Webhook{
		ID: webhookID,
	}

	var input struct {
		Title         string `json:"title"`
		ResponseCode  int    `json:"response_code"`
		ResponseDelay uint   `json:"response_delay"` // milliseconds
		ContentType   string `json:"content_type"`
		Payload       string `json:"payload"`
		NotifyOnEvent bool   `json:"notify_on_event"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		renderer.JSON(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if webhook.Title != input.Title && input.Title != "" {
		webhook.Title = input.Title
	}

	if webhook.ResponseCode != input.ResponseCode && input.ResponseCode != 0 {
		webhook.ResponseCode = input.ResponseCode
	}

	if webhook.ResponseDelay != input.ResponseDelay && input.ResponseDelay != 0 {
		webhook.ResponseDelay = input.ResponseDelay
	}

	if webhook.ContentType != &input.ContentType && input.ContentType != "" {
		webhook.ContentType = &input.ContentType
	}

	if webhook.Payload != &input.Payload {
		webhook.Payload = &input.Payload
	}

	if webhook.NotifyOnEvent != input.NotifyOnEvent {
		webhook.NotifyOnEvent = input.NotifyOnEvent
	}

	webhook.UpdatedAt = time.Now().UTC()

	err := sqlstore.UpdateWebhook(webhook)

	if err != nil {
		renderer.JSON(w, http.StatusInternalServerError, err)
		return
	}

	updated, err := sqlstore.GetWebhook(webhookID)
	if err != nil {
		renderer.JSON(w, http.StatusInternalServerError, "error updating webhook")
		return
	}
	log.Printf("%+v\n", webhook)

	renderer.JSON(w, http.StatusOK, updated)
}

func DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sqlstore.DeleteWebhook(id)
	w.WriteHeader(http.StatusNoContent)
}
