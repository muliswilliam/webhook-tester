package handlers

import (
	"encoding/json"
	"net/http"
	"time"
	"webhook-tester/internal/models"
	memory "webhook-tester/internal/store"
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
		NofifyOnEvent bool   `json:"notify_on_event"`
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
		ContentType:   input.ContentType,
		Payload:       input.Payload,
		CreatedAt:     time.Now().UTC().String(),
		NofifyOnEvent: input.NofifyOnEvent,
	}
	// persist struct
	memory.SaveWebhook(webhook)
	renderer.JSON(w, http.StatusCreated, webhook)
}

func ListWebhooks(w http.ResponseWriter, r *http.Request) {
	webhooks := memory.ListWebhooks()
	renderer.JSON(w, http.StatusOK, webhooks)
}

func GetWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	webhook, found := memory.GetWebhookByID(webhookID)

	if !found {
		renderer.JSON(w, http.StatusNotFound, "webhook not found")
		return
	}

	renderer.JSON(w, http.StatusOK, webhook)
}

func UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	_, found := memory.GetWebhookByID(webhookID)
	if !found {
		renderer.JSON(w, http.StatusNotFound, "webhook with specifified id not found")
		return
	}

	var input struct {
		Title         string `json:"title"`
		ResponseCode  int    `json:"response_code"`
		ResponseDelay uint   `json:"response_delay"` // milliseconds
		ContentType   string `json:"content_type"`
		Payload       string `json:"payload"`
		NofifyOnEvent bool   `json:"notify_on_event"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		renderer.JSON(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	webhook := models.Webhook{
		ID:            webhookID,
		Title:         input.Title,
		ResponseCode:  input.ResponseCode,
		ResponseDelay: input.ResponseDelay,
		ContentType:   input.ContentType,
		Payload:       input.Payload,
		CreatedAt:     time.Now().UTC().String(),
		NofifyOnEvent: input.NofifyOnEvent,
	}

	updated, err := memory.UpdateWebhook(webhook)

	if err != nil {
		renderer.JSON(w, http.StatusInternalServerError, err)
		return
	}

	renderer.JSON(w, http.StatusOK, updated)
}

func DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Delete webook by ID"))
}
